// Package tools contains example agent tools and a small helper for building
// JSON-Schema parameter definitions. Add your own task-solving tools here — any
// type implementing port.Tool is auto-registered when passed to usecase.NewAgent.
package tools

import (
	"encoding/json"
)

// schema is a convenience for writing a JSON-Schema object literal.
type schema map[string]any

// mustJSON marshals v or panics; intended for static schemas built at startup.
func mustJSON(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}
