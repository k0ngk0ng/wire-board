package engine

import (
	"encoding/json"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/basketball/campaign"
)

func TestNewOriginalCampaignSource(t *testing.T) {
	var f struct {
		Calls []struct {
			Mode                   uint8
			RandomBeforeRewardDraw RandomState
			Expected               CampaignStart
		}
		NaturalBoundary struct {
			OuterBefore, RandomBeforeRewardDraw RandomState
			Expected                            CampaignStart
		}
	}
	readPropAttackFixture(t, "testdata/campaign-new-game-calls.json.gz", &f)
	if len(f.Calls) != 64 {
		t.Fatal("missing original creation calls", len(f.Calls))
	}
	rewards := map[uint8]bool{}
	for i, r := range f.Calls {
		got, e := NewOriginalCampaign(r.Mode, r.RandomBeforeRewardDraw)
		if e != nil || got != r.Expected {
			t.Fatalf("case%d got%+v source%+v err%v", i, got, r.Expected, e)
		}
		if e = got.Progress.Validate(); e != nil {
			t.Fatal(i, e)
		}
		rewards[got.Progress.RewardSet] = true
		data, e := json.Marshal(got)
		if e != nil {
			t.Fatal(e)
		}
		var restored CampaignStart
		if e = json.Unmarshal(data, &restored); e != nil || restored != got {
			t.Fatal("creation save/restore", i, e)
		}
		again, e := NewOriginalCampaign(r.Mode, r.RandomBeforeRewardDraw)
		if e != nil || again != got {
			t.Fatal("same immutable input is nondeterministic", i, e)
		}
	}
	if len(rewards) != 4 {
		t.Fatal("source reward selector coverage", rewards)
	}
	b := f.NaturalBoundary
	if b.OuterBefore != 0x61834046 || b.RandomBeforeRewardDraw != 0xb0c1a023 {
		t.Fatal("wrong retained original boundary", b)
	}
	got, e := NewOriginalCampaign(0, b.RandomBeforeRewardDraw)
	if e != nil || got != b.Expected || got.Random != 0x5860d011 {
		t.Fatal("factory repeated outer draw or missed inner draw", got, e)
	}
}
func TestNewOriginalCampaignRejectsUnsupportedModesWithoutDraw(t *testing.T) {
	for _, mode := range []uint8{2, 3, 127, 128, 129, 255} {
		for _, random := range []RandomState{0, 0x12345678, 0xb0c1a023, 0xffffffff} {
			got, e := NewOriginalCampaign(mode, random)
			if e == nil || got.Random != random || got.Progress != (campaign.Progress{}) {
				t.Fatal("invalid mode consumed or created state", mode, random, got, e)
			}
		}
	}
}
