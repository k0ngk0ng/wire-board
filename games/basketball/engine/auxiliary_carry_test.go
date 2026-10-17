package engine

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestOriginalAuxiliaryPickupAndThrowActionChain(t *testing.T) {
	f, err := os.Open("testdata/auxiliary-carry-chain.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var rows []struct {
		Chain, Tick, Actor int
		PickupFlags        uint8
		Before, After      WorldActionState
	}
	if err = json.NewDecoder(z).Decode(&rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 92 {
		t.Fatal("missing original chain calls", len(rows))
	}
	var player Player
	var auxiliary AuxiliaryState
	source := uint8(0)
	previousChain := -1
	for _, r := range rows {
		if r.Chain != previousChain {
			player = r.Before.Actors[0].Player
			auxiliary = r.Before.Auxiliary[1]
			source = r.Before.PoseSource
			previousChain = r.Chain
		}
		if r.Actor == 0 {
			// Original input is the external phase; retain positions, motion and all
			// action animation state continuously, and accept only its request/heading.
			player.Request, player.Heading = r.Before.Actors[0].Player.Request, r.Before.Actors[0].Player.Heading
			auxiliary.Heading = r.Before.Auxiliary[1].Heading
			p, e := ResolvePlayerRequest(player)
			if e != nil {
				t.Fatal(e)
			}
			if p.Action == 0x4e || p.Action == 0x20 || p.Action == 0x27 {
				got, e := StepAuxiliaryCarrier(player, AuxiliaryCarrierContext{Actor: 0, PickupFlags: r.PickupFlags, PickupFlagsKnown: true, Owners: [2]int8{r.Before.Auxiliary[0].Owner, auxiliary.Owner}}, KunioProfile(), NewYorkGroundCourt())
				if e != nil {
					t.Fatal(r.Tick, e)
				}
				player = got.Player
				if got.ChangeAuxiliary {
					if got.Object != 5 {
						t.Fatal("wrong pickup object", got)
					}
					auxiliary.Action, auxiliary.Entered = got.Action, false
					if got.BindOwner {
						auxiliary.Owner, auxiliary.Attachment = 0, 128
					}
				}
			} else {
				player, e = StepFreePlayer(player, KunioProfile(), NewYorkGroundCourt())
				if e != nil {
					t.Fatal(r.Tick, e)
				}
			}
		} else {
			if auxiliary != r.Before.Auxiliary[1] {
				t.Fatalf("noncontinuous auxiliary input tick%d", r.Tick)
			}
			ctx := AuxiliaryContext{Object: 5, Court: 1, SpecialHolder: r.Before.SpecialHolder, Owner: &player, OwnerKind: 0}
			var got AuxiliaryStep
			if auxiliary.Action == 0 || auxiliary.Action == 12 {
				got, err = StepAuxiliaryRest(auxiliary, ctx)
			} else {
				got, err = StepAuxiliaryCarryThrow(auxiliary, ctx, auxiliaryNewYorkCourt())
			}
			if err != nil {
				t.Fatal(r.Tick, auxiliary.Action, err)
			}
			auxiliary = got.State
			if got.ChangePoseSource {
				source = got.PoseSource
			}
		}
		if player != r.After.Actors[0].Player || auxiliary != r.After.Auxiliary[1] || source != r.After.PoseSource {
			t.Fatalf("chain%d tick%d actor%d player:%s aux:%s source%d/%d", r.Chain, r.Tick, r.Actor, firstWorldLoopDifference(reflect.ValueOf(player), reflect.ValueOf(r.After.Actors[0].Player), "player"), firstWorldLoopDifference(reflect.ValueOf(auxiliary), reflect.ValueOf(r.After.Auxiliary[1]), "auxiliary"), source, r.After.PoseSource)
		}
	}
	t.Log("two 23-update original pickup/A-throw action chains, own motion/animation retained continuously; input phase external")
}

func TestOriginalAuxiliaryCarryHiddenAndThrowInstructions(t *testing.T) {
	f, err := os.Open("testdata/auxiliary-carry-probes.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var rows []struct {
		Action        uint8
		Before, After WorldActionState
	}
	if err = json.NewDecoder(z).Decode(&rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 152 {
		t.Fatal("missing original probes", len(rows))
	}
	for i, r := range rows {
		got, err := StepAuxiliaryCarryThrow(r.Before.Auxiliary[1], AuxiliaryContext{Object: 5, Court: 1, Owner: &r.Before.Actors[0].Player, OwnerKind: 0, SpecialHolder: r.Before.SpecialHolder, SpecialOwner: &r.Before.Actors[1].Player}, auxiliaryNewYorkCourt())
		source := r.Before.PoseSource
		if got.ChangePoseSource {
			source = got.PoseSource
		}
		if err != nil || got.State != r.After.Auxiliary[1] || source != r.After.PoseSource {
			t.Fatalf("row%d action%d: %s source%d/%d err%v", i, r.Action, firstWorldLoopDifference(reflect.ValueOf(got.State), reflect.ValueOf(r.After.Auxiliary[1]), "auxiliary"), source, r.After.PoseSource, err)
		}
	}
}

func TestOriginalAuxiliaryRequestDispatch(t *testing.T) {
	b, err := os.ReadFile("testdata/auxiliary-request-probes.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		Action, Request, NextAction, NextRequest uint8
		Entered                                  bool
	}
	if err = json.Unmarshal(b, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 512 {
		t.Fatal(len(rows))
	}
	for _, r := range rows {
		before := AuxiliaryState{Action: r.Action, Entered: r.Entered, Request: ActionRequest{Action: Action(r.Request), Pending: true}, Body: Body{Position: Position{X: 123}}}
		want := before
		want.Action, want.Entered = r.NextAction&127, r.NextAction&128 != 0
		want.Request = ActionRequest{Action: Action(r.NextRequest & 127), Pending: r.NextRequest&128 == 0}
		got, err := ResolveAuxiliaryRequest(before)
		if err != nil || got != want {
			t.Fatal(r, got, want, err)
		}
	}
}

func TestAuxiliaryCarrierRequiresHumanPickupSource(t *testing.T) {
	p := Player{Action: 0x4e, Heading: HeadingNone, Body: Body{Position: Position{X: 300 * Pixel, Depth: 40 * Pixel}}}
	ctx := AuxiliaryCarrierContext{Actor: 0, Owners: [2]int8{-1, -1}}
	got, err := StepAuxiliaryCarrier(p, ctx, KunioProfile(), NewYorkGroundCourt())
	if err == nil || got.Player != p || got.ChangeAuxiliary {
		t.Fatal("missing original pickup marks silently selected object4", got, err)
	}
	ctx.PickupFlagsKnown = true
	got, err = StepAuxiliaryCarrier(p, ctx, KunioProfile(), NewYorkGroundCourt())
	if err != nil || got.Object != 4 {
		t.Fatal("known zero flags were rejected", got, err)
	}
	ctx.PickupFlags = 32
	got, err = StepAuxiliaryCarrier(p, ctx, KunioProfile(), NewYorkGroundCourt())
	if err != nil || got.Object != 5 {
		t.Fatal(got, err)
	}
}
