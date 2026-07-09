package wasm

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

type contextKey string

const permissionsKey contextKey = "permissions"
const notesResolverKey contextKey = "notesResolver"

type HostPermissions struct {
	ReadOtherNotes bool
	AccessEnv      bool
}

type Manager struct {
	runtime wazero.Runtime
	mu      sync.RWMutex
	modules map[string]wazero.CompiledModule
}

// NewManager initializes the wazero runtime and registers the "env" host module
// containing permission-restricted host callbacks.
func NewManager(ctx context.Context) (*Manager, error) {
	r := wazero.NewRuntime(ctx)

	// Instantiate WASI
	wasi_snapshot_preview1.MustInstantiate(ctx, r)

	// Register "env" host module with permission checks
	_, err := r.NewHostModuleBuilder("env").
		NewFunctionBuilder().
		WithFunc(func(ctx context.Context, mod api.Module, pathOffset, pathSize uint32) uint64 {
			// Check permissions
			perms, ok := ctx.Value(permissionsKey).(HostPermissions)
			if !ok || !perms.ReadOtherNotes {
				return 0 // Permission denied or missing
			}

			// Read note path from guest memory
			pathBytes, ok := mod.Memory().Read(pathOffset, pathSize)
			if !ok {
				return 0
			}

			// Resolve note content using resolver callback in context
			resolver, ok := ctx.Value(notesResolverKey).(func(string) (string, error))
			if !ok {
				return 0
			}

			content, err := resolver(string(pathBytes))
			if err != nil {
				return 0
			}

			// Allocate guest memory for content
			malloc := mod.ExportedFunction("malloc")
			if malloc == nil {
				return 0
			}

			contentBytes := []byte(content)
			size := uint64(len(contentBytes))
			results, err := malloc.Call(ctx, size)
			if err != nil || len(results) == 0 {
				return 0
			}
			outPtr := uint32(results[0])

			// Write content to guest memory
			if !mod.Memory().Write(outPtr, contentBytes) {
				return 0
			}

			// Return packed (ptr << 32) | size
			return (uint64(outPtr) << 32) | uint64(len(contentBytes))
		}).
		Export("read_note").
		NewFunctionBuilder().
		WithFunc(func(ctx context.Context, mod api.Module, keyOffset, keySize uint32) uint64 {
			// Check permissions
			perms, ok := ctx.Value(permissionsKey).(HostPermissions)
			if !ok || !perms.AccessEnv {
				return 0 // Permission denied
			}

			// Read env key from guest memory
			keyBytes, ok := mod.Memory().Read(keyOffset, keySize)
			if !ok {
				return 0
			}

			// Get env value from OS
			val := os.Getenv(string(keyBytes))

			// Allocate guest memory for env value
			malloc := mod.ExportedFunction("malloc")
			if malloc == nil {
				return 0
			}

			valBytes := []byte(val)
			size := uint64(len(valBytes))
			results, err := malloc.Call(ctx, size)
			if err != nil || len(results) == 0 {
				return 0
			}
			outPtr := uint32(results[0])

			// Write env value to guest memory
			if !mod.Memory().Write(outPtr, valBytes) {
				return 0
			}

			// Return packed (ptr << 32) | size
			return (uint64(outPtr) << 32) | uint64(len(valBytes))
		}).
		Export("get_env").
		Instantiate(ctx)

	if err != nil {
		r.Close(ctx)
		return nil, fmt.Errorf("failed to register env host functions: %w", err)
	}

	return &Manager{
		runtime: r,
		modules: make(map[string]wazero.CompiledModule),
	}, nil
}

// Close closes the wazero runtime.
func (m *Manager) Close(ctx context.Context) error {
	return m.runtime.Close(ctx)
}

// LoadPlugin compiles the WASM binary and caches it.
func (m *Manager) LoadPlugin(ctx context.Context, name string, wasmBytes []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	compiled, err := m.runtime.CompileModule(ctx, wasmBytes)
	if err != nil {
		return fmt.Errorf("failed to compile module '%s': %w", name, err)
	}

	m.modules[name] = compiled
	return nil
}

// Execute instantiates the compiled WASM plugin, writes inputs, invokes the execute ABI, and reads outputs.
func (m *Manager) Execute(ctx context.Context, pluginName string, functionName string, payload []byte) ([]byte, error) {
	m.mu.RLock()
	compiled, exists := m.modules[pluginName]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("plugin '%s' not loaded", pluginName)
	}

	// Instantiate the module for this execution
	config := wazero.NewModuleConfig().
		WithStdout(nil).
		WithStderr(nil)

	mod, err := m.runtime.InstantiateModule(ctx, compiled, config)
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate module: %w", err)
	}
	defer mod.Close(ctx)

	// Initialize the runtime if _initialize is exported (reactor library buildmode)
	initializer := mod.ExportedFunction("_initialize")
	if initializer != nil {
		_, err = initializer.Call(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize runtime: %w", err)
		}
	}

	// Retrieve exported functions
	malloc := mod.ExportedFunction("malloc")
	free := mod.ExportedFunction("free")
	execute := mod.ExportedFunction("execute")

	if malloc == nil || free == nil || execute == nil {
		return nil, fmt.Errorf("missing standard ABI in plugin: malloc, free, and execute are required")
	}

	// 1. Allocate guest memory for payload
	payloadSize := uint64(len(payload))
	results, err := malloc.Call(ctx, payloadSize)
	if err != nil || len(results) == 0 {
		return nil, fmt.Errorf("malloc failed for payload: %w", err)
	}
	payloadPtr := uint32(results[0])
	defer free.Call(ctx, uint64(payloadPtr))

	// Write payload to guest memory
	if !mod.Memory().Write(payloadPtr, payload) {
		return nil, fmt.Errorf("failed to write payload to guest memory")
	}

	// 2. Allocate guest memory for function name
	funcBytes := []byte(functionName)
	funcSize := uint64(len(funcBytes))
	results, err = malloc.Call(ctx, funcSize)
	if err != nil || len(results) == 0 {
		return nil, fmt.Errorf("malloc failed for function name: %w", err)
	}
	funcPtr := uint32(results[0])
	defer free.Call(ctx, uint64(funcPtr))

	// Write function name to guest memory
	if !mod.Memory().Write(funcPtr, funcBytes) {
		return nil, fmt.Errorf("failed to write function name to guest memory")
	}

	// 3. Call execute(funcPtr, funcSize, payloadPtr, payloadSize)
	execResults, err := execute.Call(ctx, uint64(funcPtr), funcSize, uint64(payloadPtr), payloadSize)
	if err != nil {
		return nil, fmt.Errorf("execute call failed: %w", err)
	}
	if len(execResults) == 0 {
		return nil, fmt.Errorf("execute call returned nothing")
	}

	packed := execResults[0]
	outPtr := uint32(packed >> 32)
	outSize := uint32(packed)

	if outSize == 0 {
		return nil, nil
	}

	// Read result from guest memory
	outBytes, ok := mod.Memory().Read(outPtr, outSize)
	if !ok {
		return nil, fmt.Errorf("failed to read execution result from guest memory")
	}

	// Clone output bytes before closing module
	copied := make([]byte, len(outBytes))
	copy(copied, outBytes)

	// Free returned guest memory
	guestFree := mod.ExportedFunction("guest_free")
	if guestFree != nil {
		_, _ = guestFree.Call(ctx, uint64(outPtr))
	} else {
		_, _ = free.Call(ctx, uint64(outPtr))
	}

	return copied, nil
}

// WithContextPermissions attaches host function permissions to context.
func WithContextPermissions(ctx context.Context, perms HostPermissions) context.Context {
	return context.WithValue(ctx, permissionsKey, perms)
}

// WithContextNotesResolver attaches a callback to resolve other notes by path.
func WithContextNotesResolver(ctx context.Context, resolver func(string) (string, error)) context.Context {
	return context.WithValue(ctx, notesResolverKey, resolver)
}
