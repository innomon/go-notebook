package wasm

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"testing"
)

func TestMain(m *testing.M) {
	// Compile mock_guest.go to WASM using TinyGo
	cmd := exec.Command("tinygo", "build", "-o", "testdata/mock_guest.wasm", "-target=wasi", "-buildmode=c-shared", "testdata/mock_guest.go")
	if err := cmd.Run(); err != nil {
		println("Failed to compile mock_guest.go to WASM: " + err.Error())
	}

	// Compile empty.go to WASM using TinyGo
	cmdEmpty := exec.Command("tinygo", "build", "-o", "testdata/empty.wasm", "-target=wasi", "-buildmode=c-shared", "testdata/empty.go")
	if err := cmdEmpty.Run(); err != nil {
		println("Failed to compile empty.go to WASM: " + err.Error())
	}

	code := m.Run()

	// Cleanup compiled WASM after tests
	_ = os.Remove("testdata/mock_guest.wasm")
	_ = os.Remove("testdata/empty.wasm")
	os.Exit(code)
}

func TestNewManager(t *testing.T) {
	ctx := context.Background()
	m, err := NewManager(ctx)
	if err != nil {
		t.Fatalf("Expected no error creating manager, got %v", err)
	}
	defer m.Close(ctx)

	if m == nil {
		t.Errorf("Expected manager to be non-nil")
	}
}

func TestExecute_WASMPlugin(t *testing.T) {
	ctx := context.Background()
	m, err := NewManager(ctx)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}
	defer m.Close(ctx)

	// Read compiled WASM
	wasmBytes, err := os.ReadFile("testdata/mock_guest.wasm")
	if err != nil {
		t.Skip("mock_guest.wasm not compiled, skipping integration tests")
	}

	err = m.LoadPlugin(ctx, "mock", wasmBytes)
	if err != nil {
		t.Fatalf("Failed to load plugin: %v", err)
	}

	// 1. Test standard execution (double)
	res, err := m.Execute(ctx, "mock", "double", []byte(`{"value": 8}`))
	if err != nil {
		t.Fatalf("Execution failed: %v", err)
	}

	var doubleOut struct {
		Value int `json:"value"`
	}
	if err := json.Unmarshal(res, &doubleOut); err != nil {
		t.Fatalf("Failed to parse result: %v", err)
	}

	if doubleOut.Value != 16 {
		t.Errorf("Expected double value to be 16, got %d", doubleOut.Value)
	}

	// 2. Test read_note host function (with permission)
	notesMap := map[string]string{
		"notes/secret.md": "This is a secret note content!",
	}
	resolver := func(path string) (string, error) {
		if content, ok := notesMap[path]; ok {
			return content, nil
		}
		return "", os.ErrNotExist
	}

	ctxWithPerm := WithContextPermissions(ctx, HostPermissions{ReadOtherNotes: true})
	ctxWithResolver := WithContextNotesResolver(ctxWithPerm, resolver)

	res, err = m.Execute(ctxWithResolver, "mock", "read_other", []byte(`{"path": "notes/secret.md"}`))
	if err != nil {
		t.Fatalf("Execution failed: %v", err)
	}

	var readOut struct {
		Content string `json:"content"`
		Error   string `json:"error"`
	}
	if err := json.Unmarshal(res, &readOut); err != nil {
		t.Fatalf("Failed to parse read result: %v", err)
	}

	if readOut.Content != "This is a secret note content!" {
		t.Errorf("Expected content 'This is a secret note content!', got '%s' (error: %s)", readOut.Content, readOut.Error)
	}

	// 3. Test read_note host function (WITHOUT permission)
	ctxNoPerm := WithContextPermissions(ctx, HostPermissions{ReadOtherNotes: false})
	ctxNoPermResolver := WithContextNotesResolver(ctxNoPerm, resolver)

	res, err = m.Execute(ctxNoPermResolver, "mock", "read_other", []byte(`{"path": "notes/secret.md"}`))
	if err != nil {
		t.Fatalf("Execution failed: %v", err)
	}

	readOut = struct {
		Content string `json:"content"`
		Error   string `json:"error"`
	}{}
	if err := json.Unmarshal(res, &readOut); err != nil {
		t.Fatalf("Failed to parse read result: %v", err)
	}

	if readOut.Content != "" || readOut.Error != "denied or error" {
		t.Errorf("Expected access to be denied, got content '%s', error '%s'", readOut.Content, readOut.Error)
	}

	// 4. Test get_env host function (with permission)
	_ = os.Setenv("TEST_API_KEY", "secret-key-123")
	defer os.Unsetenv("TEST_API_KEY")

	ctxEnvPerm := WithContextPermissions(ctx, HostPermissions{AccessEnv: true})
	res, err = m.Execute(ctxEnvPerm, "mock", "get_env_val", []byte(`{"key": "TEST_API_KEY"}`))
	if err != nil {
		t.Fatalf("Execution failed: %v", err)
	}

	var envOut struct {
		Value string `json:"value"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(res, &envOut); err != nil {
		t.Fatalf("Failed to parse env result: %v", err)
	}

	if envOut.Value != "secret-key-123" {
		t.Errorf("Expected env value 'secret-key-123', got '%s'", envOut.Value)
	}

	// 5. Test get_env host function (WITHOUT permission)
	ctxEnvNoPerm := WithContextPermissions(ctx, HostPermissions{AccessEnv: false})
	res, err = m.Execute(ctxEnvNoPerm, "mock", "get_env_val", []byte(`{"key": "TEST_API_KEY"}`))
	if err != nil {
		t.Fatalf("Execution failed: %v", err)
	}

	envOut = struct {
		Value string `json:"value"`
		Error string `json:"error"`
	}{}
	if err := json.Unmarshal(res, &envOut); err != nil {
		t.Fatalf("Failed to parse env result: %v", err)
	}

	if envOut.Value != "" || envOut.Error != "denied" {
		t.Errorf("Expected env access to be denied, got value '%s', error '%s'", envOut.Value, envOut.Error)
	}
}

func TestWASM_Errors(t *testing.T) {
	ctx := context.Background()
	m, err := NewManager(ctx)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}
	defer m.Close(ctx)

	// 1. Test loading invalid WASM bytes
	err = m.LoadPlugin(ctx, "invalid", []byte("invalid wasm bytes"))
	if err == nil {
		t.Errorf("Expected compile error for invalid WASM bytes")
	}

	// 2. Test executing an unloaded plugin
	_, err = m.Execute(ctx, "unloaded", "double", []byte(`{}`))
	if err == nil {
		t.Errorf("Expected execution error for unloaded plugin")
	}

	// 3. Test loading a plugin without standard ABI exports (empty.wasm)
	emptyBytes, err := os.ReadFile("testdata/empty.wasm")
	if err == nil {
		err = m.LoadPlugin(ctx, "empty", emptyBytes)
		if err != nil {
			t.Fatalf("Failed to load empty plugin: %v", err)
		}

		_, err = m.Execute(ctx, "empty", "double", []byte(`{}`))
		if err == nil {
			t.Errorf("Expected error when executing plugin lacking standard ABI (malloc/free/execute)")
		}
	}
}

