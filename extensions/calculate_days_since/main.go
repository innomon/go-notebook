package main

import (
	"encoding/json"
	"fmt"
	"go-notebook/extensions/guest_sdk"
	"time"
	"unsafe"
)

type InputPayload struct {
	Properties map[string]any `json:"properties"`
	Args       []string       `json:"args"` // First arg is the property name, second is reference date (optional)
}

//go:wasmexport execute
func execute(funcNamePtr, funcNameSize, payloadPtr, payloadSize uint32) uint64 {
	funcNameBytes := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(funcNamePtr))), funcNameSize)
	funcName := string(funcNameBytes)

	payloadBytes := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(payloadPtr))), payloadSize)

	if funcName != "calculate_days_since" {
		return guest_sdk.PackOutput([]byte(fmt.Sprintf(`{"error": "unknown function: %s"}`, funcName)))
	}

	var payload InputPayload
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return guest_sdk.PackOutput([]byte(fmt.Sprintf(`{"error": "json parse error: %v"}`, err)))
	}

	propName := "created_at"
	if len(payload.Args) > 0 && payload.Args[0] != "" {
		propName = payload.Args[0]
	}

	dateVal, ok := payload.Properties[propName]
	if !ok {
		return guest_sdk.PackOutput([]byte(fmt.Sprintf(`{"error": "property '%s' not found"}`, propName)))
	}

	dateStr, ok := dateVal.(string)
	if !ok {
		return guest_sdk.PackOutput([]byte(fmt.Sprintf(`{"error": "property '%s' is not a string"}`, propName)))
	}

	// Parse date (supports YYYY-MM-DD and YYYY-MM-DDTHH:MM:SSZ)
	var parsedTime time.Time
	var err error
	formats := []string{"2006-01-02", "2006-01-02T15:04:05Z", time.RFC3339}
	for _, fmtStr := range formats {
		parsedTime, err = time.Parse(fmtStr, dateStr)
		if err == nil {
			break
		}
	}
	if err != nil {
		return guest_sdk.PackOutput([]byte(fmt.Sprintf(`{"error": "parse date '%s': %v"}`, dateStr, err)))
	}

	// Reference date: default to 2026-07-09 (current local time)
	refStr := "2026-07-09"
	if len(payload.Args) > 1 && payload.Args[1] != "" {
		refStr = payload.Args[1]
	}
	refTime, err := time.Parse("2006-01-02", refStr)
	if err != nil {
		refTime = time.Now()
	}

	duration := refTime.Sub(parsedTime)
	days := int(duration.Hours() / 24)

	return guest_sdk.PackOutput([]byte(fmt.Sprintf(`{"days": %d}`, days)))
}

func main() {}
