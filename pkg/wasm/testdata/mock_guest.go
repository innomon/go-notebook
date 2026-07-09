package main

import (
	"encoding/json"
	"fmt"
	"unsafe"
)

// Import the host functions from "env" module
//go:wasmimport env read_note
func hostReadNote(pathOffset, pathSize uint32) uint64

//go:wasmimport env get_env
func hostGetEnv(keyOffset, keySize uint32) uint64

var allocations = make(map[uintptr][]byte)

// Helper to write string to guest memory and return packed pointer/size.
// We store the slice in allocations map so TinyGo's GC doesn't reclaim it.
func packOutput(data []byte) uint64 {
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

//go:wasmexport execute
func execute(funcNamePtr, funcNameSize, payloadPtr, payloadSize uint32) uint64 {
	funcNameBytes := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(funcNamePtr))), funcNameSize)
	funcName := string(funcNameBytes)

	payloadBytes := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(payloadPtr))), payloadSize)

	switch funcName {
	case "double":
		var input struct {
			Value int `json:"value"`
		}
		if err := json.Unmarshal(payloadBytes, &input); err != nil {
			return packOutput([]byte(fmt.Sprintf(`{"error": "%v"}`, err)))
		}
		result := input.Value * 2
		return packOutput([]byte(fmt.Sprintf(`{"value": %d}`, result)))

	case "read_other":
		var input struct {
			Path string `json:"path"`
		}
		if err := json.Unmarshal(payloadBytes, &input); err != nil {
			return packOutput([]byte(fmt.Sprintf(`{"error": "%v"}`, err)))
		}
		pathBytes := []byte(input.Path)
		packed := hostReadNote(uint32(uintptr(unsafe.Pointer(&pathBytes[0]))), uint32(len(pathBytes)))
		
		outPtr := uint32(packed >> 32)
		outSize := uint32(packed)
		if outSize == 0 {
			return packOutput([]byte(`{"content": null, "error": "denied or error"}`))
		}
		noteBytes := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(outPtr))), outSize)
		return packOutput([]byte(fmt.Sprintf(`{"content": "%s"}`, string(noteBytes))))

	case "get_env_val":
		var input struct {
			Key string `json:"key"`
		}
		if err := json.Unmarshal(payloadBytes, &input); err != nil {
			return packOutput([]byte(fmt.Sprintf(`{"error": "%v"}`, err)))
		}
		keyBytes := []byte(input.Key)
		packed := hostGetEnv(uint32(uintptr(unsafe.Pointer(&keyBytes[0]))), uint32(len(keyBytes)))
		
		outPtr := uint32(packed >> 32)
		outSize := uint32(packed)
		if outSize == 0 {
			return packOutput([]byte(`{"value": null, "error": "denied"}`))
		}
		envBytes := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(outPtr))), outSize)
		return packOutput([]byte(fmt.Sprintf(`{"value": "%s"}`, string(envBytes))))
	}

	return packOutput([]byte(`{"error": "unknown function"}`))
}

func main() {}
