package engine

import (
	"reflect"
	"testing"
)

func applySetDownCommand(s *WorldActionState, command *AuxiliarySetDownCommand) {
	if command == nil {
		return
	}
	a := &s.Auxiliary[command.Object-4]
	a.Action, a.Entered = command.Action, false
	if command.ClearAttachment {
		a.Attachment = 0
	}
	if command.ReleaseOwner {
		a.Owner = -1
	}
}

func TestOriginalAuxiliarySetDownStagesBeforeMotion(t *testing.T) {
	var rows []struct {
		Actor, Court  uint8
		Before, After WorldActionState
	}
	decodeAuxiliaryWorldFixture(t, "auxiliary-setdown-probes.json.gz", &rows)
	if len(rows) != 192 {
		t.Fatal(len(rows))
	}
	for i, r := range rows {
		ctx := AuxiliaryCarrierContext{Actor: r.Actor, Owners: [2]int8{r.Before.Auxiliary[0].Owner, r.Before.Auxiliary[1].Owner}, Attachment: r.Before.PlayerLinks[r.Actor], HasBall: r.Before.Holder == int8(r.Actor)}
		prepared, e := PrepareAuxiliarySetDown(r.Before.Actors[r.Actor].Player, ctx)
		if e != nil {
			t.Fatal(i, e)
		}
		world := WorldActionStep{State: r.Before}
		applySetDownCommand(&world.State, prepared.Command)
		court, e := originalWorldMotionCourt(&world, WorldActionContext{CourtID: r.Court, Teams: [2]uint8{0, 1}}, r.Actor)
		if e != nil {
			t.Fatal(e)
		}
		got, e := CompleteAuxiliarySetDown(prepared, ctx, court)
		if e != nil {
			t.Fatal(i, e)
		}
		world.State.Actors[r.Actor].Player = got.Player
		world.State.Ball = ApplyBallCommand(world.State.Ball, got.Ball)
		if got.ChangeAttachment {
			world.State.PlayerLinks[r.Actor] = got.Attachment
		}
		if world.State != r.After {
			t.Fatalf("row%d: %s", i, firstWorldLoopDifference(reflect.ValueOf(world.State), reflect.ValueOf(r.After), "world"))
		}
	}
}

func TestOriginalPickupAndNondirectionalBSetDownChain(t *testing.T) {
	var rows []struct {
		Tick, Actor   int
		PickupFlags   uint8
		Before, After WorldActionState
	}
	decodeAuxiliaryWorldFixture(t, "auxiliary-setdown-chain.json.gz", &rows)
	if len(rows) != 34 {
		t.Fatal("missing complete input/action chain", len(rows))
	}
	state := rows[0].Before
	for _, r := range rows {
		if r.Actor == 0 {
			state.Actors[0].Player.Request, state.Actors[0].Player.Heading = r.Before.Actors[0].Player.Request, r.Before.Actors[0].Player.Heading
			state.Auxiliary[0].Heading, state.Auxiliary[1].Heading = r.Before.Auxiliary[0].Heading, r.Before.Auxiliary[1].Heading
			ctx := AuxiliaryCarrierContext{Actor: 0, PickupFlags: r.PickupFlags, PickupFlagsKnown: true, Owners: [2]int8{state.Auxiliary[0].Owner, state.Auxiliary[1].Owner}, Attachment: state.PlayerLinks[0]}
			resolved, e := ResolvePlayerRequest(state.Actors[0].Player)
			if e != nil {
				t.Fatal(e)
			}
			if resolved.Action == 0x4f {
				prepared, e := PrepareAuxiliarySetDown(state.Actors[0].Player, ctx)
				if e != nil {
					t.Fatal(e)
				}
				applySetDownCommand(&state, prepared.Command)
				got, e := CompleteAuxiliarySetDown(prepared, ctx, NewYorkGroundCourt())
				if e != nil {
					t.Fatal(e)
				}
				state.Actors[0].Player = got.Player
			} else if resolved.Action == 0x20 || resolved.Action == 0x4e {
				got, e := StepAuxiliaryCarrier(state.Actors[0].Player, ctx, KunioProfile(), NewYorkGroundCourt())
				if e != nil {
					t.Fatal(e)
				}
				state.Actors[0].Player = got.Player
				if got.ChangeAuxiliary {
					a := &state.Auxiliary[got.Object-4]
					a.Action, a.Entered = got.Action, false
					if got.BindOwner {
						a.Owner, a.Attachment = 0, 128
					}
				}
			} else {
				p, e := StepFreePlayer(state.Actors[0].Player, KunioProfile(), NewYorkGroundCourt())
				if e != nil {
					t.Fatal(e)
				}
				state.Actors[0].Player = p
			}
		} else {
			result := WorldActionStep{State: state}
			e := advanceWorldAuxiliary(&result, 5, WorldActionContext{CourtID: 1, Teams: [2]uint8{0, 1}}, auxiliaryNewYorkCourt())
			if e != nil {
				t.Fatal(e)
			}
			state = result.State
		}
		if state.Actors[0].Player != r.After.Actors[0].Player || state.Auxiliary[1] != r.After.Auxiliary[1] || state.PoseSource != r.After.PoseSource {
			t.Fatalf("tick%d actor%d: player%s aux%s", r.Tick, r.Actor, firstWorldLoopDifference(reflect.ValueOf(state.Actors[0].Player), reflect.ValueOf(r.After.Actors[0].Player), "player"), firstWorldLoopDifference(reflect.ValueOf(state.Auxiliary[1]), reflect.ValueOf(r.After.Auxiliary[1]), "auxiliary"))
		}
	}
	if state.Auxiliary[1].Owner != -1 || state.Auxiliary[1].Airborne || state.Auxiliary[1].Action != 0 {
		t.Fatal("original B set-down did not reach rest")
	}
}
