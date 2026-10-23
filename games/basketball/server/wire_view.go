package basketball

import (
	"encoding/json"
	"fmt"
	"github.com/k0ngk0ng/wire-board/games/basketball/engine"
	"github.com/k0ngk0ng/wire-board/games/basketball/syncwire"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

type wireInput struct {
	Sequence       int64          `json:"sequence"`
	Buttons        engine.Buttons `json:"buttons"`
	AppliedButtons engine.Buttons `json:"appliedButtons"`
	Automated      bool           `json:"automated"`
}

// Native acceptance can inspect Runtime/Inputs directly; ordinary HTTP and WS
// serialize only public metadata, recent encoded inputs and a consistency hash.
func (v View) MarshalJSON() ([]byte, error) {
	type plain View
	var inputs [4]wireInput
	for i, input := range v.Inputs {
		inputs[i] = wireInput{Sequence: input.Sequence, Buttons: input.Buttons, AppliedButtons: input.AppliedButtons, Automated: input.Automated}
	}
	return json.Marshal(struct {
		*plain
		Inputs [4]wireInput `json:"inputs"`
	}{plain: (*plain)(&v), Inputs: inputs})
}

type SyncSnapshot struct {
	Inputs     [4]InputState       `json:"inputs"`
	View       View                `json:"view"`
	Checkpoint syncwire.Checkpoint `json:"checkpoint"`
}

func syncSnapshot(match gameapi.Match, seat int) (any, error) {
	value, err := project(match, seat)
	if err != nil {
		return nil, err
	}
	v := value.(View)
	if seat < -1 || seat >= len(v.SeatActors) {
		return nil, fmt.Errorf("无效同步席位")
	}
	return SyncSnapshot{Inputs: v.Inputs, View: v, Checkpoint: syncwire.Checkpoint{Version: 1, State: v.Runtime}}, nil
}
