package engine

import (
	"encoding/json"
	"reflect"
	"slices"
	"testing"
)

type campaignShotEquipmentRow struct {
	worldActionFixture
	Actor    uint8
	Label    string
	Sounds   []uint8
	RNGCalls int
}

func TestCampaignShotEquipmentOriginalPlayerCalls(t *testing.T) {
	var rows []campaignShotEquipmentRow
	readPropAttackFixture(t, "testdata/campaign-shot-player.json.gz", &rows)
	if len(rows) != 3184 {
		t.Fatal("missing original release calls", len(rows))
	}
	item1Ground, item1Air, randomPositive := 0, 0, 0
	for i, r := range rows {
		result := WorldActionStep{State: r.Before}
		ctx := r.context(t)
		if e := advanceWorldPlayer(&result, r.Actor, ctx, NewYorkGroundCourt()); e != nil {
			t.Fatal(i, r.Label, e)
		}
		if result.State != r.After {
			t.Fatalf("call%d %s: %s", i, r.Label, firstWorldLoopDifference(reflect.ValueOf(result.State), reflect.ValueOf(r.After), "world"))
		}
		if !slices.Equal(result.SoundIDs, r.Sounds) {
			t.Fatal("sound mismatch", i, r.Label, result.SoundIDs, r.Sounds)
		}
		before := r.Before.Actors[r.Actor].Player
		if !before.Entered && ctx.ShotEquipment[r.Actor] == 1 && ctx.Teams[r.Actor/2] == 0 && ctx.EnhancementEnabled {
			if r.RNGCalls != 0 || r.After.Random != r.Before.Random {
				t.Fatal("shirt1 consumed RNG", r.Label)
			}
			if before.Action == ActionShoot && r.After.Ball.Action == 13 {
				item1Ground++
			}
			if before.Action == ActionAirShoot && r.After.Ball.Action == originalCharacterSpecialBalls[0][ctx.Characters[r.Actor]] {
				item1Air++
			}
		}
		if r.RNGCalls > 0 {
			randomPositive++
		}
	}
	if item1Ground == 0 || item1Air == 0 || randomPositive == 0 {
		t.Fatal("source paths absent", item1Ground, item1Air, randomPositive)
	}
	t.Logf("original calls=%d shirt1-ground13=%d air-character-special=%d RNG-positive-controls=%d", len(rows), item1Ground, item1Air, randomPositive)
}
func TestCampaignShotEquipmentOriginalWholeWorld(t *testing.T) {
	var rows []campaignShotEquipmentRow
	readPropAttackFixture(t, "testdata/campaign-shot-world.json.gz", &rows)
	if len(rows) != 96 {
		t.Fatal("missing whole-world source rows", len(rows))
	}
	for i, r := range rows {
		got, e := StepWorldActions(r.Before, r.context(t))
		if e != nil {
			t.Fatal(i, r.Label, e)
		}
		if got.State != r.After {
			t.Fatalf("world%d %s: %s", i, r.Label, firstWorldLoopDifference(reflect.ValueOf(got.State), reflect.ValueOf(r.After), "world"))
		}
		if !slices.Equal(got.Calls, r.Calls) || !slices.Equal(got.SoundIDs, r.Sounds) {
			t.Fatal("original visit/sound order", i, r.Label, got.Calls, r.Calls, got.SoundIDs, r.Sounds)
		}
		data, e := json.Marshal(r.Before)
		if e != nil {
			t.Fatal(e)
		}
		var restored WorldActionState
		if e = json.Unmarshal(data, &restored); e != nil {
			t.Fatal(e)
		}
		replay, e := StepWorldActions(restored, r.context(t))
		if e != nil || !reflect.DeepEqual(got, replay) {
			t.Fatal("restored whole action phase", i, e)
		}
	}
}
