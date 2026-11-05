package yuyuhakusho

import "encoding/json"

func numericBytes(values []uint8) []int {
	numbers := make([]int, len(values))
	for i, value := range values {
		numbers[i] = int(value)
	}
	return numbers
}

// MarshalJSON preserves numeric character IDs on both the platform wire and
// saved options. A []uint8 field otherwise receives encoding/json's []byte
// base64 representation, which is not the browser's number[] contract.
func (o Options) MarshalJSON() ([]byte, error) {
	type fields Options
	return json.Marshal(struct {
		fields
		Characters []int `json:"characters"`
	}{fields: fields(o), Characters: numericBytes(o.Characters)})
}

// MarshalJSON keeps the in-memory original slots typed as bytes while exposing
// a JSON numeric array to the browser. Checkpoint remains an opaque JSON string.
func (v View) MarshalJSON() ([]byte, error) {
	type fields View
	return json.Marshal(struct {
		fields
		SeatSlots []int `json:"seatSlots"`
	}{fields: fields(v), SeatSlots: numericBytes(v.SeatSlots)})
}
