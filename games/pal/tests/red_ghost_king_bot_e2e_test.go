package pal_test

// Before any bot change: replay the genuine new-core red-ghost-king battle,
// including its publicly visible reinforcements, with only production bot
// inputs. Preserve selected visible spells/targets and HP for diagnosis; no
// enemy HP, scripts, test route data or private native state reaches the bot.
import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	pal "github.com/k0ngk0ng/wire-board/games/pal/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func TestOriginalRedGhostKingBotFromSavedBattle(t *testing.T) {
	if os.Getenv("PAL_RED_KING_BOT") != "1" {
		t.Skip("explicit real-campaign battle regression")
	}
	sourcePath := campaignEvidencePath("blood-pool-king-battle-state.json", 1)
	prefix := "red-king-bot"
	if mode := os.Getenv("PAL_RED_KING_SOURCE"); mode == "archived" {
		sourcePath = filepath.Join("..", "..", "..", ".local/pal/verification/scene-wave-stage/campaign/blood-pool-king-battle-state-1.json")
		prefix += "-archived"
	} else if mode != "" && mode != "current" {
		t.Fatal("unsupported red king input", mode)
	}
	source, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	sourceHash := sha256.Sum256(source)
	m, _ := nativeModule(t)
	g := gameapi.Match{Schema: 1, Data: source}
	v := view(t, m, g, 0)
	c := gameapi.Context{Players: 1, Now: time.UnixMilli(v.ControlAt), Automated: []bool{false}}
	g, err = m.ResumeSaved(c, g)
	if err != nil {
		t.Fatal(err)
	}
	r := &riverMedicineDriver{t: t, m: m, c: c, g: g}
	initial := r.visible()
	if initial.Scene != 60 || initial.Battle == nil {
		t.Fatal("red king source is not an original campaign battle")
	}
	trace := []any{}
	defer func() {
		for name, data := range map[string][]byte{prefix + "-trace.json": raw(trace), prefix + "-final-state.json": r.g.Data, prefix + "-final-view.json": raw(r.visible())} {
			if err := os.WriteFile(campaignEvidencePath(name, 1), data, 0600); err != nil {
				t.Log(err)
			}
		}
	}()
	commands, reinforcements, saved := 0, false, false
	for step := 0; step < 6000; step++ {
		v := r.visible()
		if v.Dialogue != nil && strings.Contains(v.Dialogue.Text, "土灵珠在这妖怪身上") {
			if !reinforcements || !saved || v.Cash <= initial.Cash || len(v.Party) != 3 {
				t.Fatal("red king victory omitted original battle/reward", reinforcements, v.Cash, v.Party)
			}
			report, _ := json.MarshalIndent(map[string]any{"passed": true, "scope": "production public-only bot from unedited actual red-ghost-king campaign battle through reinforcement and earth-orb dialogue; selected offensive target preserved in fresh-runtime save, stale input rejected; no scene/stats/script edits", "candidate": campaignVariant(), "developmentProfile": developmentHealthProfile(), "sourceCheckpointSHA256": hex.EncodeToString(sourceHash[:]), "selectedOffenseSaved": saved, "commands": commands, "reinforcementsObserved": reinforcements, "cashBefore": initial.Cash, "cashAfter": v.Cash}, "", "  ")
			if err := os.WriteFile(campaignEvidencePath(prefix+".json", 1), report, 0600); err != nil {
				t.Fatal(err)
			}
			return
		}
		if b := v.Battle; b != nil {
			alive := false
			for _, p := range v.Party {
				alive = alive || p.HP > 0
			}
			if !alive {
				t.Fatal("red king production bot defeated", step, v.Party)
			}
			for _, e := range b.Enemies {
				reinforcements = reinforcements || e.Object != 0 && e.Object != 473
			}
			if !saved && b.Current == 1 && b.Action == 3 && b.State == 2 {
				before := v
				r.g, err = r.m.SaveControl(r.c, r.g, true)
				if err != nil {
					t.Fatal(err)
				}
				r.c.Now = r.c.Now.Add(7 * 24 * time.Hour)
				r.m, _ = nativeModule(t)
				r.g, err = r.m.ResumeSaved(r.c, r.g)
				if err != nil {
					t.Fatal(err)
				}
				normalize := func(v pal.View) pal.View {
					v.Epoch, v.Seq, v.AppliedSeq, v.DiscardedSeq, v.ControlAt = 0, 0, 0, 0, 0
					return v
				}
				if !bytes.Equal(raw(normalize(before)), raw(normalize(r.visible()))) {
					t.Fatal("red king saved offensive target changed")
				}
				if _, err := r.m.Apply(r.c, r.g, raw(pal.Action{Type: "input", Epoch: before.Epoch, Seq: before.Seq + 1, ControlAt: before.ControlAt, Direction: 4, Keys: 2})); err == nil {
					t.Fatal("red king saved target accepted an old command")
				}
				saved = true
				v = r.visible()
			}
		}
		a, err := r.m.BotAction(r.c, r.g)
		if err != nil {
			t.Fatal(err)
		}
		if len(a) > 0 {
			var action pal.Action
			if err = json.Unmarshal(a, &action); err != nil {
				t.Fatal(err)
			}
			if v.Battle != nil && (action.Keys != 0 || step%20 == 0) {
				trace = append(trace, map[string]any{"step": step, "keys": action.Keys, "battle": v.Battle, "party": v.Party, "selected": originalVisibleText(v, true)})
			}
			r.g, err = r.m.Apply(campaignBotContext(r.c), r.g, a)
			if err != nil {
				t.Fatal(err)
			}
			commands++
		}
		r.c.Now = time.UnixMilli(r.g.SystemDeadline + 200)
		r.g, err = r.m.OnTimer(r.c, r.g)
		if err != nil {
			t.Fatal(err)
		}
	}
	t.Fatal("red king bot did not complete its original battle", commands)
}
