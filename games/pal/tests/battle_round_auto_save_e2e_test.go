package pal_test

// Verify the actual original Classic command boundary: saving after Force
// accepted the first role must retain its native continuation, pending auto
// selection for later roles and first accepted command. Clearing stale input
// must not clear this accepted rule state or consume mana/rewards early.
import (
	"bytes"
	"encoding/json"
	"os"
	"testing"
	"time"

	pal "github.com/k0ngk0ng/wire-board/games/pal/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func TestOriginalAutoRoundSavedContinuation(t *testing.T) {
	b, err := os.ReadFile(campaignEvidencePath("ambush-battle-state.json", 1))
	if err != nil {
		t.Fatal(err)
	}
	m, runtime := nativeModule(t)
	g := gameapi.Match{Schema: 1, Data: b}
	forceActive := func() bool {
		t.Helper()
		var data struct {
			Native []byte `json:"native"`
		}
		if err := json.Unmarshal(g.Data, &data); err != nil {
			t.Fatal(err)
		}
		snapshot, err := runtime.Decode(data.Native)
		if err != nil {
			t.Fatal(err)
		}
		return snapshot.View.Battle.Force
	}
	v := view(t, m, g, 0)
	c := gameapi.Context{Players: 1, Now: time.UnixMilli(v.ControlAt), Automated: []bool{false}}
	g, err = m.ResumeSaved(c, g)
	if err != nil {
		t.Fatal(err)
	}
	tick := func() {
		t.Helper()
		c.Now = time.UnixMilli(g.SystemDeadline)
		g, err = m.OnTimer(c, g)
		if err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; ; i++ {
		v = view(t, m, g, 0)
		if v.Battle != nil && v.Battle.Phase == 0 && v.Battle.State == 1 && v.Battle.Menu == 0 && v.Battle.Current == 0 {
			break
		}
		if i > 1024 {
			t.Fatal("original first-role selector did not arrive", v.Phase, v.Battle, v.Dialogue)
		}
		if v.Dialogue != nil {
			g = apply(t, m, c, g, pal.Action{Type: "read", Epoch: v.Epoch, Prompt: v.Dialogue.ID})
		}
		tick()
	}
	g = apply(t, m, c, g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 1, Direction: 4, Keys: 32768})
	for i := 0; ; i++ {
		v = view(t, m, g, 0)
		if v.Battle != nil && forceActive() && v.Battle.Phase == 0 && v.Battle.Players[0].State == 2 && v.Battle.Players[1].State != 2 {
			break
		}
		if i > 64 || v.Battle == nil || v.Battle.Phase != 0 {
			t.Fatal("original accepted auto boundary was skipped", v.Battle)
		}
		tick()
	}
	before := v
	first := before.Battle.Players[0]
	g, err = m.SaveControl(c, g, true)
	if err != nil {
		t.Fatal(err)
	}
	c.Now = c.Now.Add(24 * time.Hour)
	m, runtime = nativeModule(t)
	g, err = m.ResumeSaved(c, g)
	if err != nil {
		t.Fatal(err)
	}
	after := view(t, m, g, 0)
	normalize := func(v pal.View) pal.View {
		v.Epoch, v.Seq, v.AppliedSeq, v.DiscardedSeq, v.ControlAt = 0, 0, 0, 0, 0
		return v
	}
	if !bytes.Equal(raw(normalize(before)), raw(normalize(after))) || !forceActive() {
		t.Fatal("save erased accepted original auto selection")
	}
	if _, err := m.Apply(c, g, raw(pal.Action{Type: "input", Epoch: before.Epoch, Seq: before.Seq + 1, ControlAt: before.ControlAt, Direction: 4, Keys: 32768})); err == nil {
		t.Fatal("saved auto round accepted old controls")
	}
	for i := 0; ; i++ {
		v = view(t, m, g, 0)
		if v.Battle == nil || v.Battle.Players[0].Action != first.Action || v.Battle.Players[0].Object != first.Object || v.Battle.Players[0].Target != first.Target || v.Cash != before.Cash {
			t.Fatal("resumed auto changed its first accepted choice/reward")
		}
		if v.Battle.Phase != 0 {
			if forceActive() || v.Battle.Players[1].State != 2 {
				t.Fatal("original auto failed to commit the remaining role", v.Battle)
			}
			break
		}
		if i > 128 {
			t.Fatal("saved auto round did not continue without another key")
		}
		tick()
	}
	report, _ := json.MarshalIndent(map[string]any{"passed": true, "scope": "actual original inn ambush, accepted Classic Force between original role selectors, personal save and full fresh-runtime continuation without another key", "firstAcceptedCommandPreserved": true, "acceptedAutoFlagSaved": true, "remainingRoleCommitted": true, "oldInputRejected": true, "cashUnchangedBeforeExecution": true}, "", "  ")
	if err := os.WriteFile(campaignEvidencePath("battle-auto-round-save.json", 1), report, 0600); err != nil {
		t.Fatal(err)
	}
}
