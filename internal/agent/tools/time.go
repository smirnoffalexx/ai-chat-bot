package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// CurrentTime reports the current time, optionally in a given IANA timezone.
type CurrentTime struct{}

func (CurrentTime) Name() string { return "get_current_time" }

func (CurrentTime) Description() string {
	return "Returns the current date and time. Optionally accepts an IANA timezone " +
		"name (e.g. \"Europe/Berlin\"); defaults to UTC."
}

func (CurrentTime) Schema() json.RawMessage {
	return mustJSON(schema{
		"type": "object",
		"properties": schema{
			"timezone": schema{
				"type":        "string",
				"description": "IANA timezone name, e.g. \"Europe/Berlin\". Defaults to UTC.",
			},
		},
	})
}

func (CurrentTime) Execute(_ context.Context, args json.RawMessage) (string, error) {
	var in struct {
		Timezone string `json:"timezone"`
	}
	if len(args) > 0 {
		_ = json.Unmarshal(args, &in)
	}
	loc := time.UTC
	if in.Timezone != "" {
		l, err := time.LoadLocation(in.Timezone)
		if err != nil {
			return "", fmt.Errorf("unknown timezone %q", in.Timezone)
		}
		loc = l
	}
	return time.Now().In(loc).Format("Monday, 2 January 2006, 15:04:05 MST"), nil
}
