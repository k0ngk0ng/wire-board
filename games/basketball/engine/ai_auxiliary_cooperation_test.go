package engine

import (
	"reflect"
	"testing"
)

type auxiliaryAICase struct {
	Actor                                                                     uint8
	Mode, Plan, Action, Heading, Facing, Destination, Target, Status, Message uint8
	Owners                                                                    [2]uint8
	AuxiliaryActions                                                          [2]uint8
	Heights                                                                   [2]Fixed
	Positions                                                                 [6]Position
	Link                                                                      uint8
	Holder                                                                    int8
	PlayerActions                                                             [4]uint8
	After                                                                     struct {
		Mode, Plan, Action, Heading, Facing, Destination, Target, Status, Message uint8
		Owners                                                                    [2]uint8
	}
}

func TestOriginalAuxiliaryCooperationSource(t *testing.T) {
	var rows []auxiliaryAICase
	readPropAttackFixture(t, "testdata/ai-auxiliary-cooperation.json.gz", &rows)
	if len(rows) == 0 {
		t.Fatal("missing original source cases")
	}
	for i, row := range rows {
		var aux [2]AuxiliaryState
		for j := range aux {
			aux[j].Owner = int8(row.Owners[j])
			aux[j].Action = row.AuxiliaryActions[j] & 127
			aux[j].Body.Position = row.Positions[j+4]
			aux[j].Body.Position.Height = row.Heights[j]
		}
		var players [4]Position
		copy(players[:], row.Positions[:4])
		var actions [4]Action
		for j, v := range row.PlayerActions {
			actions[j] = Action(v & 127)
		}
		view := AIControlView{Actor: row.Actor, Action: Action(row.Action & 127), ActionEntered: row.Action&128 != 0, Position: row.Positions[row.Actor], Heading: auxiliarySourceHeading(row.Heading), FacingLeft: row.Facing&128 != 0, Auxiliary: &aux, Players: players, PlayerActions: actions, TeammateLink: row.Link, BallHolder: row.Holder}
		result := AIControlDecision{Control: AIControlState{Mode: AIMode(row.Mode & 127), Entered: row.Mode&128 != 0, Plan: AIMode(row.Plan), Destination: CourtZone(row.Destination), TrackedObject: int8(row.Target)}, Action: view.Action, ActionEntered: view.ActionEntered, Heading: view.Heading, FacingLeft: view.FacingLeft, BindHoop: -1}
		if e := advanceOriginalAuxiliaryCooperation(&result, view); e != nil {
			t.Fatal(i, e)
		}
		after := row.After
		got := row.After
		got.Mode = uint8(result.Control.Mode)
		if result.Control.Entered {
			got.Mode |= 128
		}
		got.Plan = uint8(result.Control.Plan)
		got.Action = uint8(result.Action)
		if result.ActionEntered {
			got.Action |= 128
		}
		got.Heading = 255
		if result.Heading != HeadingNone {
			got.Heading = uint8(result.Heading) * 32
		}
		got.Facing = 0
		if result.FacingLeft {
			got.Facing = 128
		}
		got.Destination = uint8(result.Control.Destination)
		got.Target = uint8(result.Control.TrackedObject)
		got.Owners = row.Owners
		got.Status = row.Status
		got.Message = row.Message
		if result.ClearCommandStatus {
			got.Status = 0
		}
		if result.AuxiliaryCommand.SetMessage {
			got.Message = result.AuxiliaryCommand.Message
		}
		if result.AuxiliaryCommand.Bind {
			got.Owners[result.AuxiliaryCommand.Object-4] = row.Actor
		}
		if !reflect.DeepEqual(got, after) {
			t.Fatalf("case%d mode%d got%+v want%+v", i, row.Mode, got, after)
		}
	}
}

func auxiliarySourceHeading(v uint8) Heading {
	if v == 255 {
		return HeadingNone
	}
	return Heading(v / 32)
}

func TestOriginalAuxiliaryCooperationRejectsMissingState(t *testing.T) {
	for _, mode := range []AIMode{3, 13, 14} {
		initial := AIControlDecision{Control: AIControlState{Mode: mode}, Action: ActionRun, Heading: HeadingLeft}
		got := initial
		if e := advanceOriginalAuxiliaryCooperation(&got, AIControlView{}); e == nil || got != initial {
			t.Fatal("missing auxiliary state accepted or changed state", mode, e)
		}
	}
}
