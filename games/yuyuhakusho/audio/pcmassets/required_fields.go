package pcmassets

import (
	"bytes"
	"encoding/json"
	"errors"
)

// Required zero-valued fields must remain distinguishable from absent/null.
// Decode into detached values so an error never partially replaces a descriptor.
func (d *Descriptor) UnmarshalJSON(raw []byte) error {
	type fields Descriptor
	var next fields
	if err := requiredObject(raw, &next, "id", "sample_id", "native_rate", "flags", "protocol", "routing_or_pan"); err != nil {
		return err
	}
	*d = Descriptor(next)
	return nil
}
func (d *musicDescriptor) UnmarshalJSON(raw []byte) error {
	type fields musicDescriptor
	var next fields
	if err := requiredObject(raw, &next, "id", "sample_id", "native_rate", "flags", "routing"); err != nil {
		return err
	}
	*d = musicDescriptor(next)
	return nil
}
func requiredObject(raw []byte, dst any, keys ...string) error {
	if err := checkKeys(raw); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return err
	}
	if len(fields) != len(keys) {
		return errors.New("missing or unknown descriptor field")
	}
	for _, key := range keys {
		value, ok := fields[key]
		if !ok || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return errors.New("missing or null descriptor field")
		}
	}
	return json.Unmarshal(raw, dst)
}
