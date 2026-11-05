package yuyuhakusho

import (
	"encoding/json"
	"errors"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/renderstate"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/syncwire"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func project(m gameapi.Match, seat int) (any, error) {
	s, err := decode(m)
	if err != nil {
		return nil, err
	}
	if seat < -1 || seat >= len(s.Inputs) {
		return nil, errors.New("无效观战或玩家席位")
	}
	rules, err := renderstate.RulesDigest(s.World)
	if err != nil {
		return nil, err
	}
	visual, err := renderstate.Digest(s.World, s.Presentation)
	if err != nil {
		return nil, err
	}
	replay, err := syncwire.Encode(s.History)
	if err != nil {
		return nil, err
	}
	view := View{Protocol: syncwire.Version, Epoch: s.Epoch, Frame: s.Frame, Phase: s.phase(), Options: s.Options, SeatSlots: seatSlots(len(s.Inputs)), LocalSeat: seat, Loading: s.Loading, Digest: rules, PresentationDigest: visual, Replay: replay, ServerAt: s.ServerAt, ResourceDescriptorSHA256: s.ResourceDescriptorSHA256, Players: []PlayerView{}, Inputs: []WireInput{}}
	for platformSeat, slot := range view.SeatSlots {
		object := s.World.Objects[0x8400+uint16(slot)*0x400]
		active := object.Header.Behavior == 0x12668
		computer := s.World.Match.Controllers[slot] == 0
		character := s.Options.Characters[platformSeat]
		if active {
			character = object.Tag
			computer = object.InputChannel == 255
		}
		view.Players = append(view.Players, PlayerView{Seat: platformSeat, Slot: slot, CharacterID: character, Computer: computer, Active: active, Health: object.Body.Health, Energy: int16(object.Energy >> 16), X: int16(object.Body.X >> 16), Y: int16(object.Body.Y >> 16)})
		input := s.Inputs[platformSeat]
		view.Inputs = append(view.Inputs, WireInput{Sequence: input.Sequence, AppliedSequence: input.AppliedSequence, Buttons: input.Buttons, AppliedButtons: s.World.Controllers[platformSeat].Current, Automated: input.Automated})
	}
	return view, nil
}
func syncSnapshot(m gameapi.Match, seat int) (any, error) {
	value, err := project(m, seat)
	if err != nil {
		return nil, err
	}
	view := value.(View)
	s, err := decode(m)
	if err != nil {
		return nil, err
	}
	options, err := s.liveOptions()
	if err != nil {
		return nil, err
	}
	checkpoint := Checkpoint{Version: 1, Epoch: s.Epoch, Frame: s.Frame, Options: options, World: s.World, Presentation: s.Presentation, Identity: resources.graphics.Identity(), ResourceDescriptorSHA256: s.ResourceDescriptorSHA256}
	raw, err := json.Marshal(checkpoint)
	if err != nil {
		return nil, err
	}
	return SyncSnapshot{View: view, Checkpoint: string(raw)}, nil
}
