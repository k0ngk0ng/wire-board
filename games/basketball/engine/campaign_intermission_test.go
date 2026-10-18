package engine

// Failure inventory before implementation: miss the second roster draw, draw
// twice on held Start or restore, overwrite score/period/wins/current equipment,
// clear persistent carried gameplay memory, retain old roster/grades/HUD aliases,
// mutate the input snapshot on error, or apply campaign selection to versus.
// Source selection/cleanup and later commit are separate menu observations;
// their intervening presentation RNG ticks are not a combined timing oracle.
import (
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/basketball/campaign"
)

func TestCampaignConfirmationOriginalOperations(t *testing.T) {
	var rows []struct {
		Run                             string
		SelectionBefore, SelectionAfter struct {
			Mode        uint8
			Pending     [4]uint8
			ObjectKind5 uint8
			MenuScratch [9]uint8
			Random      RandomState
		}
		Commit struct {
			Characters                     [4]uint8
			Grades                         [4]CharacterGrades
			Equipment                      [4][3]campaign.Item
			Mode, Court, Period, RewardSet uint8
			Teams                          [2]uint8
			WinBits                        [2]uint8
			Inventory                      [16]campaign.Item
		}
	}
	raw, err := os.ReadFile("testdata/campaign-confirmation.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 4 {
		t.Fatal("missing original menu confirmation and commit boundaries")
	}
	for _, row := range rows {
		t.Run(row.Run, func(t *testing.T) {
			p := campaign.Progress{Mode: row.Commit.Mode, Period: row.Commit.Period &^ 1, WinBits: row.Commit.WinBits, RewardSet: row.Commit.RewardSet, Equipment: row.Commit.Equipment, Inventory: row.Commit.Inventory}
			choice := CampaignMatchSelection{Opponent: row.Commit.Teams[1], Characters: row.SelectionBefore.Pending, HumanTactics: [2]uint8{4, 4}, Seed: row.SelectionBefore.Random}
			s, e := NewCampaignRuntime(p, choice)
			if e != nil {
				t.Fatal(e)
			}
			o := *s.Opening
			o.Transition.Scene = 2
			o.Transition.Match.Phase = 4
			o.Transition.Match.Period = row.Commit.Period
			o.Transition.Kinds[5] = row.SelectionBefore.ObjectKind5
			o.Transition.Passages = row.SelectionBefore.MenuScratch
			o.Transition.Scores = [2]uint16{17, 23}
			o.Transition.Objects[0].Body.Position.X = 777
			beforeConfig, beforeScene := s.Config, o
			c, got, e := confirmCampaignIntermission(s.Config, o)
			if e != nil {
				t.Fatal(e)
			}
			tr := got.Transition
			if c.Match.Characters != row.SelectionAfter.Pending || tr.Characters != row.Commit.Characters || tr.Random != row.SelectionAfter.Random || tr.Kinds[5] != row.SelectionAfter.ObjectKind5 || tr.Passages != row.SelectionAfter.MenuScratch || got.Grades != row.Commit.Grades {
				t.Fatal("source confirmation/commit differs", c.Match.Characters, tr.Random, got.Grades)
			}
			if tr.Match.Clock.ScoreLowCache != ([2]uint8{tr.Characters[0], tr.Characters[1]}) || tr.Match.Clock.DisplayedSecond != tr.Characters[2] || tr.Match.Clock.HUDTurn != tr.Characters[3] {
				t.Fatal("03CB..CE aliases did not follow committed roster")
			}
			if tr.Scene != 1 || tr.Match.Phase != 0 || tr.Match.Entered || tr.Scores != beforeScene.Transition.Scores || tr.Match.Period != beforeScene.Transition.Match.Period || tr.TeamWinBits != beforeScene.Transition.TeamWinBits || got.Equipment != beforeScene.Equipment || *c.Campaign != *s.Config.Campaign {
				t.Fatal("confirmation rewrote retained match state")
			}
			for i := 6; i < 13; i++ {
				if tr.Objects[i] != beforeScene.Transition.Objects[i] {
					t.Fatal("confirmation rewrote non-menu actor", i)
				}
			}
			if !reflect.DeepEqual(beforeConfig, s.Config) || beforeScene != o {
				t.Fatal("confirmation mutated inputs")
			}
			for _, kind := range []string{"no-campaign", "playing", "duplicate-teammate"} {
				badConfig, badScene := beforeConfig, beforeScene
				switch kind {
				case "no-campaign":
					badConfig.Campaign = nil
				case "playing":
					badScene.Transition.Scene = 1
				case "duplicate-teammate":
					badConfig.Match.Characters[1] = badConfig.Match.Characters[0]
				}
				x, y, e := confirmCampaignIntermission(badConfig, badScene)
				if e == nil || !reflect.DeepEqual(x, badConfig) || y != badScene {
					t.Fatal("invalid confirmation was not atomic", kind, e)
				}
			}
		})
	}
}

func TestOriginalCampaignRosterCommitAndTail(t *testing.T) {
	var rows []struct {
		Run                                                 string
		CommitRawBefore, CommitRawAfter, CommitTailRawAfter []byte
	}
	raw, e := os.ReadFile("testdata/campaign-confirmation.json")
	if e != nil {
		t.Fatal(e)
	}
	if e = json.Unmarshal(raw, &rows); e != nil {
		t.Fatal(e)
	}
	if len(rows) != 4 {
		t.Fatal("missing source commit rows")
	}
	for _, row := range rows {
		t.Run(row.Run, func(t *testing.T) {
			if len(row.CommitRawBefore) != 2048 || len(row.CommitRawAfter) != 2048 || len(row.CommitTailRawAfter) != 2048 {
				t.Fatal("missing full original commit/tail RAM")
			}
			before, want := openingRAM(row.CommitRawBefore), openingRAM(row.CommitRawAfter)
			got, e := commitOriginalCampaignRoster(before)
			if e != nil || got != want {
				t.Fatal("B2EF..B32D", e, firstWorldLoopDifference(reflect.ValueOf(got), reflect.ValueOf(want), "opening"))
			}
			for i := uint8(0); i < 4; i++ {
				sounds, e := stepOriginalResultActor(&got, i)
				if e != nil || len(sounds) != 0 {
					t.Fatal("original actor tail", i, sounds, e)
				}
			}
			want = openingRAM(row.CommitTailRawAfter)
			if got != want {
				t.Fatal("B32D..B337", firstWorldLoopDifference(reflect.ValueOf(got), reflect.ValueOf(want), "opening"))
			}
		})
	}
}

func TestCampaignIntermissionStartCommitsOnce(t *testing.T) {
	p, choice, _ := campaignHomeFixture(t)
	s, err := NewCampaignRuntime(p, choice)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 24000 && !(s.AwaitingNextPeriod && s.WaitVideoFrames == 0); i++ {
		r, e := StepVideo(s, [4]Buttons{})
		if e != nil {
			t.Fatal(e)
		}
		s = r.State
	}
	if !s.AwaitingNextPeriod || s.WaitVideoFrames != 0 {
		t.Fatal("normal first half did not reach preparation")
	}
	scores := s.Transition.Scene.Transition.Scores
	period := s.Transition.Scene.Transition.Match.Period
	if period != 3 || s.VideoFrames < 10800 {
		t.Fatal("natural half-period progression missing")
	}
	oldChars := s.Config.Match.Characters
	for i := 0; i < 6; i++ {
		r, e := StepVideo(s, [4]Buttons{})
		if e != nil {
			t.Fatal(e)
		}
		s = r.State
	}
	if !s.AwaitingNextPeriod || s.Config.Match.Characters != oldChars {
		t.Fatal("waiting automatically confirmed roster")
	}
	bad := cloneRuntime(s)
	bad.Carry = nil
	if ValidateRuntimeState(bad) == nil {
		t.Fatal("campaign halftime without carried world accepted")
	}
	bad = cloneRuntime(s)
	bad.Transition.Scene.Transition.Match.Period = p.Period
	if ValidateRuntimeState(bad) == nil {
		t.Fatal("even half-period accepted as halftime")
	}
	var committed CampaignRosterSelection
	var carry *WorldLoopState
	confirmed := false
	for i := 0; i < 4 && !confirmed; i++ {
		before := s
		r, e := StepVideo(s, [4]Buttons{ButtonStart})
		if e != nil {
			t.Fatal(e)
		}
		if r.LogicAdvanced {
			outer, _ := NextRandom(before.Transition.Scene.Transition.Random)
			committed, e = SelectOriginalCampaignRoster(p.Mode, [2]uint8{oldChars[0], oldChars[1]}, outer)
			if e != nil {
				t.Fatal(e)
			}
			if r.State.Opening == nil || r.State.Transition != nil || r.State.AwaitingNextPeriod || r.State.Config.Match.Characters != committed.Characters || r.State.Opening.Transition.Random != committed.Random {
				t.Fatal("Start missed original roster selection", r.State.Config.Match.Characters, committed)
			}
			carry = before.Carry
			if !reflect.DeepEqual(carry, r.State.Carry) || r.State.Opening.Transition.Scores != scores || r.State.Opening.Transition.Match.Period != period || *r.State.Config.Campaign != p {
				t.Fatal("Start reset carried gameplay/progress")
			}
			confirmed = true
		}
		s = r.State
	}
	if !confirmed {
		t.Fatal("buffered Start did not confirm")
	}
	raw, _ := json.Marshal(s)
	var restored RuntimeState
	if err = json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	if err = ValidateRuntimeState(restored); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 500 && s.Playing == nil; i++ {
		// Held Start has no new edge and must not redraw an enemy roster.
		a, e := StepVideo(s, [4]Buttons{ButtonStart})
		b, e2 := StepVideo(restored, [4]Buttons{ButtonStart})
		if e != nil || e2 != nil || !reflect.DeepEqual(a, b) {
			t.Fatal("restored confirmation changed the next video", e, e2)
		}
		s, restored = a.State, b.State
		if s.Config.Match.Characters != committed.Characters {
			t.Fatal("roster selected again during opening")
		}
	}
	if s.Playing == nil || s.Playing.Context.Actions.Characters != committed.Characters || s.Playing.Match.Period != period || s.Playing.Context.Difficulty != period || s.Playing.World.Preparation.Scores != scores {
		t.Fatal("confirmed second half did not enter correctly")
	}
	if err = ValidateRuntimeState(s); err != nil {
		t.Fatal(err)
	}
	want, err := NewOriginalDisplay(s.Config.Match)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if s.Display.HUD.Panels[i].HeaderGlyphs != want.HUD.Panels[i].HeaderGlyphs {
			t.Fatal("HUD retained old character names", i)
		}
	}
	t.Logf("natural intermission period=%d scores=%v old_roster=%v new_roster=%v committed_rng=%08x second_playing_video=%d", period, scores, oldChars, committed.Characters, uint32(committed.Random), s.VideoFrames)
}
