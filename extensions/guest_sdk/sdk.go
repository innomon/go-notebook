package guest_sdk

import (
	"errors"
	"unsafe"
)

// Import the host functions from "env" module
//go:wasmimport env read_note
func hostReadNote(pathOffset, pathSize uint32) uint64

//go:wasmimport env get_env
func hostGetEnv(keyOffset, keySize uint32) uint64

var allocations = make(map[uintptr][]byte)

// PackOutput writes the response bytes to guest memory, registers them in a global map
// to prevent TinyGo's GC from reclaiming them, and returns a packed pointer/size uint64.
func PackOutput(data []byte) uint64 {
	size := uint32(len(data))
	if size == 0 {
		return 0
	}
	buf := make([]byte, size)
	copy(buf, data)
	ptr := uintptr(unsafe.Pointer(&buf[0]))
	allocations[ptr] = buf
	return (uint64(ptr) << 32) | uint64(size)
}

//go:wasmexport guest_free
func guestFree(ptr uint32) {
	delete(allocations, uintptr(ptr))
}

// ReadNote calls the host callback to read another note from the workspace.
func ReadNote(path string) (string, error) {
	if len(path) == 0 {
		return "", errors.New("empty path")
	}
	pathBytes := []byte(path)
	packed := hostReadNote(uint32(uintptr(unsafe.Pointer(&pathBytes[0]))), uint32(len(pathBytes)))
	
	ptr := uint32(packed >> 32)
	size := uint32(packed)
	if size == 0 {
		return "", errors.New("read_note denied or failed")
	}
	
	contentBytes := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(ptr))), size)
	return string(contentBytes), nil
}

// GetEnv calls the host callback to read an environment variable from the host system.
func GetEnv(key string) (string, error) {
	if len(key) == 0 {
		return "", errors.New("empty key")
	}
	keyBytes := []byte(key)
	packed := hostGetEnv(uint32(uintptr(unsafe.Pointer(&keyBytes[0]))), uint32(len(keyBytes)))
	
	ptr := uint32(packed >> 32)
	size := uint32(packed)
	if size == 0 {
		return "", errors.New("get_env denied or failed")
	}
	
	valBytes := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(ptr))), size)
	return string(valBytes), nil
}
