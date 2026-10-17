package audiostate

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

//go:embed source-data.json
var sourceJSON []byte

type sourceInstruction struct {
	Opcode  byte   `json:"opcode"`
	Args    []byte `json:"args"`
	Size    byte   `json:"size"`
	Address uint16 `json:"source_address"`
}
type sourceHeader struct {
	Priority byte  `json:"priority"`
	Repeat   *byte `json:"repeat"`
	Channels []struct {
		Channel byte   `json:"channel"`
		Stream  uint16 `json:"stream"`
	} `json:"channels"`
}
type sourceInstrument struct {
	Envelope uint16 `json:"envelope"`
	Control  byte   `json:"control"`
	Sweep    byte   `json:"sweep"`
}
type sourceEnvelope struct {
	Initial byte `json:"initial"`
	Gate    byte `json:"gate"`
	Release byte `json:"release"`
	Steps   []struct {
		Delta byte `json:"delta"`
		Wait  byte `json:"wait"`
	} `json:"steps"`
}
type sourceData struct {
	Sounds       map[byte]uint16              `json:"sounds"`
	Headers      map[uint16]sourceHeader      `json:"headers"`
	Instructions map[uint32]sourceInstruction `json:"instructions"`
	Instruments  map[uint16]sourceInstrument  `json:"instruments"`
	Envelopes    map[uint16]sourceEnvelope    `json:"envelopes"`
	Percussion   map[uint16]uint16            `json:"percussion"`
	PitchLow     []byte                       `json:"pitch_low"`
	PitchHigh    []byte                       `json:"pitch_high"`
	HandlerHigh  []byte                       `json:"handler_high"`
}

var sources, sourceError = loadSources()

func loadSources() (sourceData, error) {
	var d sourceData
	if err := json.Unmarshal(sourceJSON, &d); err != nil {
		return d, fmt.Errorf("audio source data: %w", err)
	}
	if len(d.Sounds) != 76 || len(d.PitchLow) != 65 || len(d.PitchHigh) != 64 || len(d.HandlerHigh) != 32 {
		return d, fmt.Errorf("incomplete audio source tables")
	}
	return d, nil
}

func (e sourceEnvelope) value(index byte) (byte, error) {
	switch index {
	case 0:
		return e.Initial, nil
	case 1:
		return e.Gate, nil
	case 2:
		return e.Release, nil
	}
	step := int(index-3) / 2
	if step >= len(e.Steps) {
		return 0, fmt.Errorf("envelope index %d outside source", index)
	}
	if (index-3)%2 == 0 {
		return e.Steps[step].Delta, nil
	}
	return e.Steps[step].Wait, nil
}
