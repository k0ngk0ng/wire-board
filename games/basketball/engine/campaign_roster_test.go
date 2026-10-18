package engine

import "testing"

type campaignRosterBoundary struct {
	Mode       uint8
	Period     uint8
	Teams      [2]uint8
	Pending    [4]uint8
	Characters [4]uint8
	Random     RandomState
}
type campaignRosterOracle struct {
	Before, After campaignRosterBoundary
	RandomCalls   int
}

func campaignRosterCases(t *testing.T) ([]campaignRosterOracle, []campaignRosterOracle, []campaignRosterOracle) {
	t.Helper()
	var f struct {
		Calls, Boundaries []campaignRosterOracle
		Natural           []struct{ Oracle campaignRosterOracle }
	}
	readPropAttackFixture(t, "testdata/campaign-roster-calls.json.gz", &f)
	if len(f.Calls) != 392 || len(f.Boundaries) != 64 || len(f.Natural) != 4 {
		t.Fatal("incomplete original roster oracle", len(f.Calls), len(f.Boundaries), len(f.Natural))
	}
	natural := make([]campaignRosterOracle, len(f.Natural))
	for i, r := range f.Natural {
		natural[i] = r.Oracle
	}
	return f.Calls, f.Boundaries, natural
}
func TestOriginalCampaignRosterSelection(t *testing.T) {
	calls, _, natural := campaignRosterCases(t)
	for _, rows := range [][]campaignRosterOracle{calls, natural} {
		for i, r := range rows {
			teammates := [2]uint8{r.Before.Pending[0], r.Before.Pending[1]}
			got, e := SelectOriginalCampaignRoster(r.Before.Mode, teammates, r.Before.Random)
			if e != nil || got.Characters != r.After.Pending || got.Random != r.After.Random || r.RandomCalls != 1 {
				t.Fatal(i, r.Before.Mode, r.Before.Period, r.Before.Teams, got, r.After, e)
			}
		}
	}
}
func TestOriginalCampaignRosterRejectsNonCampaignBoundary(t *testing.T) {
	_, rows, _ := campaignRosterCases(t)
	for i, r := range rows {
		got, e := SelectOriginalCampaignRoster(r.Before.Mode, [2]uint8{0, 1}, r.Before.Random)
		if e == nil || got.Random != r.Before.Random || got.Characters != ([4]uint8{}) || r.RandomCalls != 0 {
			t.Fatal(i, "noncampaign boundary", got, e)
		}
	}
}
func TestOriginalCampaignRosterInvalidInputsDoNotConsumeRandom(t *testing.T) {
	for _, mode := range []uint8{0, 1, 2, 127, 128, 129, 255} {
		for _, team := range [][2]uint8{{0, 0}, {2, 2}, {4, 1}, {0, 4}, {255, 3}, {0, 1}, {2, 3}} {
			invalid := mode > 1 || team[0] >= 4 || team[1] >= 4 || team[0] == team[1]
			if !invalid {
				continue
			}
			got, e := SelectOriginalCampaignRoster(mode, team, 0x12345678)
			if e == nil || got.Random != 0x12345678 || got.Characters != ([4]uint8{}) {
				t.Fatal(mode, team, got, e)
			}
		}
	}
}
func TestOriginalCampaignRosterAllValidTeammatesKeepSelection(t *testing.T) {
	calls, _, _ := campaignRosterCases(t)
	for _, r := range calls[:4] {
		for mode := uint8(0); mode < 2; mode++ {
			for a := uint8(0); a < 4; a++ {
				for b := uint8(0); b < 4; b++ {
					if a == b {
						continue
					}
					got, e := SelectOriginalCampaignRoster(mode, [2]uint8{a, b}, r.Before.Random)
					want := [4]uint8{a, b, r.After.Pending[2], r.After.Pending[3]}
					if e != nil || got.Characters != want || got.Random != r.After.Random {
						t.Fatal(mode, a, b, got, want, e)
					}
				}
			}
		}
	}
}
