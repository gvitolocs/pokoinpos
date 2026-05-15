package main

import (
	"encoding/json"
	"fmt"
	"time"
)

func logEvent(event string, fields map[string]any) {
	payload := map[string]any{
		"ts":    time.Now().UTC().Format(time.RFC3339),
		"event": event,
	}
	for key, value := range fields {
		payload[key] = value
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		fmt.Printf("{\"event\":\"%s\",\"error\":\"log_marshal_failed\"}\n", event)
		return
	}
	fmt.Println(string(raw))
}
