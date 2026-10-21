package tests

// Native/portal evidence supplied before implementation adds these failures:
// artificial two-shot quota; a minimum 20% charge; healing item interpreted as
// fuel; fixed healing amounts; healing landing before flight, hurting opponents,
// carving terrain, leaking future HP, or losing pending effect on restoration.
import (
	"encoding/json"
	fb "github.com/k0ngk0ng/wire-board/games/fortressblue/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"reflect"
	"testing"
	"time"
)

func TestNativeChargeAndUnlimitedSpecial(t *testing.T) {
	mod, m, now := liveFixture(t, 2, false)
	s, _ := timeline(t, m)
	for _, power := range []float64{0, 19, 100} {
		raw, _ := json.Marshal(fb.Action{Type: "fire", Epoch: s.Epoch, Turn: s.Turn, Angle: 90, Power: power, Facing: 1, Weapon: "primary"})
		if _, e := mod.Apply(gameapi.Context{Players: 2, Actor: 0, Now: now}, m, raw); e != nil {
			t.Fatalf("native normalized charge %v rejected: %v", power, e)
		}
	}
	for shot := 0; shot < 3; shot++ {
		m = completeSupplyTransport(t, mod, m)
		s, _ = timeline(t, m)
		for s.Current != 0 {
			m = timerAt(t, mod, m, s.Deadline)
			m = completeSupplyTransport(t, mod, m)
			s, _ = timeline(t, m)
		}
		now = time.UnixMilli(s.Deadline - 20000 + 1)
		raw, _ := json.Marshal(fb.Action{Type: "fire", Epoch: s.Epoch, Turn: s.Turn, Angle: 90, Power: 100, Facing: 1, Weapon: "special"})
		n, e := mod.Apply(gameapi.Context{Players: 2, Actor: 0, Now: now}, m, raw)
		if e != nil {
			t.Fatalf("special shot %d rejected: %v", shot+1, e)
		}
		for i := 0; i < 20; i++ {
			_, w := timeline(t, n)
			if w.Resolution == nil {
				break
			}
			n = timerAt(t, mod, n, n.SystemDeadline)
		}
		m = n
		s, _ = timeline(t, m)
		if s.Tanks[0].Special != -1 {
			t.Fatalf("new match special quota should be unlimited, got %d", s.Tanks[0].Special)
		}
		now = time.UnixMilli(s.Deadline - 10000)
	}
}
func TestNativePercentageHealingItems(t *testing.T) {
	for _, tc := range []struct{ id, pct int }{{0, 20}, {1, 40}, {3, 20}} {
		mod, m, now := liveFixture(t, 4, false)
		s, _ := timeline(t, m)
		s.Tanks[0].HP = 100
		s.Tanks[1].HP = 100
		s.Tanks[2].HP = 100
		s.Tanks[0].Items = []int{tc.id}
		m.Data, _ = json.Marshal(s)
		raw, _ := json.Marshal(fb.Action{Type: "item", Epoch: s.Epoch, Turn: s.Turn, Slot: 0})
		n, e := mod.Apply(gameapi.Context{Players: 4, Actor: 0, Now: now}, m, raw)
		if e != nil {
			t.Fatal(e)
		}
		got, _ := timeline(t, n)
		if got.Tanks[0].HP != 100+s.Tanks[0].MaxHP*tc.pct/100 {
			t.Fatalf("item %d uses fixed HP instead of percentage", tc.id)
		}
		if tc.id == 3 && got.Tanks[1].HP != 100+s.Tanks[1].MaxHP/5 {
			t.Fatal("team heal missed ally percentage")
		}
		if got.Tanks[2].HP != 100 {
			t.Fatal("team heal healed enemy")
		}
	}
}
func TestHealingShellUsesSharedImpactTimelineAndHealsBothTeams(t *testing.T) {
	mod, m, now := liveFixture(t, 4, false)
	s, _ := timeline(t, m)
	s.Tanks[0].Items = []int{2}
	s.Tanks[0].HP = 400
	s.Tanks[1].X, s.Tanks[1].Y, s.Tanks[1].HP = 440, 816, 100
	s.Tanks[2].HP = 100
	s.Tanks[3].HP = s.Tanks[3].MaxHP - 10
	before := s
	m.Data, _ = json.Marshal(s)
	raw, _ := json.Marshal(fb.Action{Type: "item", Epoch: s.Epoch, Turn: s.Turn, Slot: 0})
	n, e := mod.Apply(gameapi.Context{Players: 4, Actor: 0, Now: now}, m, raw)
	if e != nil {
		t.Fatal(e)
	}
	armed, _ := timeline(t, n)
	if armed.Tanks[0].Fuel != before.Tanks[0].Fuel || armed.Tanks[0].HP != 400 {
		t.Fatal("energy shell changed fuel/HP before firing")
	}
	var public struct {
		Tanks []struct{ PendingShotEffect string }
	}
	view, e := mod.View(n, 0)
	if e != nil {
		t.Fatal(e)
	}
	bytes, _ := json.Marshal(view)
	_ = json.Unmarshal(bytes, &public)
	if public.Tanks[0].PendingShotEffect != "heal" {
		t.Fatal("armed healing effect not visible")
	}
	restored, e := mod.Restore(gameapi.Context{Players: 4, Actor: 0, Now: now}, n)
	if e != nil || !reflect.DeepEqual(n.Data, restored.Data) {
		t.Fatal("restore lost pending healing shell")
	}
	n = fireFixture(t, mod, n, now)
	flying, w := timeline(t, n)
	if flying.LastShot.Weapon != "healing" {
		t.Fatal("shot is not marked healing")
	}
	for i := range flying.Tanks {
		if flying.Tanks[i].HP != before.Tanks[i].HP {
			t.Fatal("healing before impact")
		}
	}
	n = timerAt(t, mod, n, w.Resolution.ImpactAt)
	impact, _ := timeline(t, n)
	for _, i := range []int{1, 2, 3} {
		want := min(before.Tanks[i].MaxHP, before.Tanks[i].HP+before.Tanks[i].MaxHP*30/100)
		if impact.Tanks[i].HP != want {
			t.Fatalf("healing target %d: got %d want %d", i, impact.Tanks[i].HP, want)
		}
	}
	if len(impact.Craters) != len(before.Craters) || impact.Tanks[0].Damage != before.Tanks[0].Damage {
		t.Fatal("healing shell damaged terrain or scored damage")
	}
	for _, ev := range impact.Events {
		if ev.At == w.Resolution.ImpactAt && (ev.Type == "hit" || ev.Type == "death") {
			t.Fatal("healing shell generated damage/death")
		}
	}
}

func TestPiercingShellPreservesOrdinaryWind(t *testing.T) {
	mod, m, now := liveFixture(t, 2, false)
	s, _ := timeline(t, m)
	s.Wind = 8
	s.Tanks[0].Items = []int{35}
	m.Data, _ = json.Marshal(s)
	baseline := fireFixture(t, mod, m, now)
	base, _ := timeline(t, baseline)
	raw, _ := json.Marshal(fb.Action{Type: "item", Epoch: s.Epoch, Turn: s.Turn, Slot: 0})
	armed, e := mod.Apply(gameapi.Context{Players: 2, Actor: 0, Now: now}, m, raw)
	if e != nil {
		t.Fatal(e)
	}
	shot := fireFixture(t, mod, armed, now)
	got, _ := timeline(t, shot)
	if !reflect.DeepEqual(base.LastShot.Arcs, got.LastShot.Arcs) {
		t.Fatal("tornado-piercing item removed ordinary wind")
	}
}

func TestLegacyTankThirteenStatsRestoreWithoutReset(t *testing.T) {
	mod, m, now := liveFixture(t, 2, false)
	s, _ := timeline(t, m)
	s.Tanks[0].TankID = 13
	s.Tanks[0].MaxHP = 990
	s.Tanks[0].HP = 613
	s.Tanks[0].Special = 2
	s.Tanks[0].SpecialUnlimited = false
	m.Data, _ = json.Marshal(s)
	restored, e := mod.Restore(gameapi.Context{Players: 2, Actor: 0, Now: now}, m)
	if e != nil {
		t.Fatalf("previously published tank-13 save rejected after source-stat correction: %v", e)
	}
	got, _ := timeline(t, restored)
	if got.Tanks[0].MaxHP != 990 || got.Tanks[0].HP != 613 || got.Deadline != s.Deadline || got.Turn != s.Turn {
		t.Fatal("legacy migration reset health or action progress")
	}
}
