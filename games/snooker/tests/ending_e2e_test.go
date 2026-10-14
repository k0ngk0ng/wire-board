package snooker_test

// Before explicit ending explanations: a black stays up after a final foul,
// but the result says only "finished"; a pot, declaration, concession and
// forfeit are indistinguishable; old finished snapshots lack a reason card.
import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func assertEndingExplanation(t *testing.T, mod gameapi.Module, m gameapi.Match, terms ...string) {
	t.Helper()
	view, e := mod.View(m, 0)
	if e != nil {
		t.Fatal(e)
	}
	text, _ := view.(map[string]any)["endExplanation"].(string)
	for _, term := range terms {
		if !strings.Contains(text, term) {
			t.Fatal("missing explicit ending explanation", term, text)
		}
	}
	logs := strings.Join(state(t, m).Logs, "\n")
	if !strings.Contains(logs, text) {
		t.Fatal("ending explanation absent from match log", text)
	}
	trace(t, map[string]any{"explanation": text, "state": state(t, m)})
}

func TestSnookerBlackFoulEndingExplainsRemainingBlack(t *testing.T) {
	mod, ctx, m := begin(t, 2)
	m = onlyBlack(t, m, [2]int{127, 1})
	s := state(t, m)
	s.Turn = 1
	s.TurnIndex = 1
	m.Data, _ = json.Marshal(s)
	ctx.Actor = 1
	m = action(t, mod, ctx, m, map[string]any{"type": "shoot", "angle": 0, "power": .06})
	m = finishStroke(t, mod, ctx, m)
	s = state(t, m)
	if m.Result == nil || s.Scores != [2]int{134, 1} || s.Balls[21].Down {
		t.Fatal("fixture did not reproduce the black remaining after a foul")
	}
	assertEndingExplanation(t, mod, m, "黑球", "未先击中目标球", "7 分", "首次得分或犯规")
	// A v1.12.1 finished snapshot has no rule explanation in its historical log;
	// projecting it must still explain the outcome without rewriting its history.
	s.Logs = []string{"玩家 2 犯规，A 队加 7 分：未先击中目标球", "第 1 局结束，A 队获胜（134 : 1）"}
	m.Data, _ = json.Marshal(s)
	before := string(m.Data)
	v, e := mod.View(m, 0)
	if e != nil || !strings.Contains(v.(map[string]any)["endExplanation"].(string), "未先击中目标球") || string(m.Data) != before {
		t.Fatal("legacy finished match explanation failed", e)
	}
}

func TestSnookerOtherEndingReasonsAreExplicit(t *testing.T) {
	for _, kind := range []string{"pot", "claim", "concede", "resign"} {
		t.Run(kind, func(t *testing.T) {
			mod, ctx, m := begin(t, 2)
			switch kind {
			case "pot":
				m = onlyBlack(t, m, [2]int{})
				m = action(t, mod, ctx, m, map[string]any{"type": "shoot", "angle": -1.5707963267948966, "power": .25})
				m = finishStroke(t, mod, ctx, m)
				assertEndingExplanation(t, mod, m, "黑球", "进球")
			case "claim":
				m = onlyBlack(t, m, [2]int{8, 0})
				m = action(t, mod, ctx, m, map[string]any{"type": "claim_frame"})
				assertEndingExplanation(t, mod, m, "领先超过 7 分", "宣告")
			case "concede":
				s := state(t, m)
				s.Scores = [2]int{200, 0}
				m.Data, _ = json.Marshal(s)
				ctx.Actor = 1
				m = action(t, mod, ctx, m, map[string]any{"type": "concede_frame"})
				ctx.Actor = 0
				m = action(t, mod, ctx, m, map[string]any{"type": "concede_reply", "choice": "accept"})
				assertEndingExplanation(t, mod, m, "认输本局", "接受")
			case "resign":
				ctx.Actor = 1
				m = action(t, mod, ctx, m, map[string]any{"type": "resign"})
				assertEndingExplanation(t, mod, m, "弃赛", "整场")
			}
		})
	}
}
