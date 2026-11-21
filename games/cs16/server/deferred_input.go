package cs16

import "encoding/json"

func deferredInput(raw json.RawMessage) bool {
	var action struct {
		Type   string `json:"type"`
		Cancel bool   `json:"cancel"`
	}
	return json.Unmarshal(raw, &action) == nil && action.Type == "input" && !action.Cancel
}
