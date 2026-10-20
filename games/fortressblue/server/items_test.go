package fortressblue

import (
	"encoding/json"
	"testing"
	"time"

	fbdata "github.com/k0ngk0ng/wire-board/games/fortressblue/data"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func playingMatch(t *testing.T, players int) (*State, gameapi.Match) {
	t.Helper()
	s, m := startAt(t, players, "stage-4")
	catalog := fbdata.IDs()
	ids := make([]int, players)
	for i := range ids {
		ids[i] = catalog[i%len(catalog)]
	}
	m, s = selectAll(t, s, m, ids)
	if s.Phase != PhasePlaying {
		t.Fatalf("phase = %s, want playing", s.Phase)
	}
	return s, m
}

func itemAction(seat, turn, slot int, epoch string) json.RawMessage {
	raw, _ := json.Marshal(Action{Type: "item", Epoch: epoch, Turn: turn, Slot: slot})
	return raw
}

func TestStartingItemsAreDistinctAndOriginal(t *testing.T) {
	s, _ := playingMatch(t, 8)
	for seat := range s.Tanks {
		items := s.Tanks[seat].Items
		if len(items) != StartingItems {
			t.Fatalf("seat %d items = %v", seat, items)
		}
		seen := map[int]bool{}
		for _, id := range items {
			if seen[id] {
				t.Fatalf("seat %d duplicate item %d", seat, id)
			}
			seen[id] = true
			if _, ok := fbdata.ItemByID(id); !ok {
				t.Fatalf("seat %d unknown item %d", seat, id)
			}
		}
	}
}

func TestUseHealFuelAndTeamHeal(t *testing.T) {
	s, m := playingMatch(t, 4)
	now := time.Unix(1700000002, 0)
	s.Tanks[0].Items = []int{0, 2, 3, 4}
	s.Tanks[0].HP = 100
	s.Tanks[0].Fuel = 3
	s.Tanks[1].HP = 200
	m, _ = s.envelope()

	m, err := apply(ctx(4, 0, now), m, itemAction(0, s.Turn, 0, s.Epoch))
	if err != nil {
		t.Fatal(err)
	}
	s, _ = decode(m)
	if s.Tanks[0].HP != 280 {
		t.Fatalf("heal hp = %d", s.Tanks[0].HP)
	}
	if s.Turn != 1 || s.Current != 0 {
		t.Fatalf("item ended turn: turn %d current %d", s.Turn, s.Current)
	}

	m, err = apply(ctx(4, 0, now), m, itemAction(0, s.Turn, 0, s.Epoch))
	if err != nil {
		t.Fatal(err)
	}
	s, _ = decode(m)
	if s.Tanks[0].Fuel != 73 {
		t.Fatalf("fuel = %v", s.Tanks[0].Fuel)
	}

	m, err = apply(ctx(4, 0, now), m, itemAction(0, s.Turn, 0, s.Epoch))
	if err != nil {
		t.Fatal(err)
	}
	s, _ = decode(m)
	if s.Tanks[0].HP != 400 || s.Tanks[1].HP != 320 {
		t.Fatalf("team heal hp = %d/%d", s.Tanks[0].HP, s.Tanks[1].HP)
	}
}

func TestShieldHalvesBlastDamage(t *testing.T) {
	s, _ := playingMatch(t, 2)
	weapon := Weapon{Name: "主炮", Damage: 100, Blast: 14}
	target := &s.Tanks[1]
	center := tankCenter(*target)
	plain := applyBlast(s, 0, weapon, center, "", 1, 1)
	s.Tanks[1].HP = s.Tanks[1].MaxHP
	addStatus(target, Status{Kind: "shield", Turns: 2, Source: 1})
	shielded := applyBlast(s, 0, weapon, center, "", 2, 1)
	if shielded.Damage >= plain.Damage || shielded.Damage > plain.Damage/2+2 {
		t.Fatalf("shield damage = %d, plain = %d", shielded.Damage, plain.Damage)
	}
}

func TestPowerNoWindAndWindReverse(t *testing.T) {
	s, m := playingMatch(t, 2)
	now := time.Unix(1700000003, 0)
	s.Tanks[0].Items = []int{25, 35, 26}
	s.Wind = 8
	m, _ = s.envelope()
	for slot := 0; slot < 3; slot++ {
		var err error
		m, err = apply(ctx(2, 0, now), m, itemAction(0, s.Turn, 0, s.Epoch))
		if err != nil {
			t.Fatal(err)
		}
		s, _ = decode(m)
	}
	if !hasStatus(&s.Tanks[0], "power") || !hasStatus(&s.Tanks[0], "nowind") {
		t.Fatalf("statuses = %+v", s.Tanks[0].Statuses)
	}
	if s.Wind != -8 {
		t.Fatalf("wind = %d, want -8", s.Wind)
	}
	raw, _ := json.Marshal(Action{Type: "fire", Epoch: s.Epoch, Turn: s.Turn, Angle: 45, Power: 60, Facing: 1, Weapon: "primary"})
	m, err := apply(ctx(2, 0, now), m, raw)
	if err != nil {
		t.Fatal(err)
	}
	s, _ = decode(m)
	if hasStatus(&s.Tanks[0], "power") || hasStatus(&s.Tanks[0], "nowind") {
		t.Fatalf("shot did not consume statuses: %+v", s.Tanks[0].Statuses)
	}
	if s.LastShot == nil {
		t.Fatal("missing shot")
	}
}

func TestItemValidation(t *testing.T) {
	s, m := playingMatch(t, 2)
	now := time.Unix(1700000004, 0)
	cases := []json.RawMessage{
		itemAction(0, s.Turn+1, 0, s.Epoch),
		itemAction(0, s.Turn, 99, s.Epoch),
	}
	for i, raw := range cases {
		if _, err := apply(ctx(2, 0, now), m, raw); err == nil {
			t.Fatalf("case %d accepted", i)
		}
	}
	bad, _ := json.Marshal(Action{Type: "item", Epoch: s.Epoch, Turn: s.Turn, Slot: 0, Angle: 20})
	if _, err := apply(ctx(2, 0, now), m, bad); err == nil {
		t.Fatal("item with extra fields accepted")
	}
}

func TestBotUsesItemThenFires(t *testing.T) {
	s, m := playingMatch(t, 2)
	s.Tanks[0].Items = []int{0}
	s.Tanks[0].HP = 40
	m, _ = s.envelope()
	raw, err := botAction(ctx(2, 0, time.Unix(1700000005, 0)), m)
	if err != nil {
		t.Fatal(err)
	}
	var first Action
	if err = json.Unmarshal(raw, &first); err != nil {
		t.Fatal(err)
	}
	if first.Type != "item" {
		t.Fatalf("bot first action = %+v", first)
	}
	m, err = apply(ctx(2, 0, time.Unix(1700000005, 0)), m, raw)
	if err != nil {
		t.Fatal(err)
	}
	s, _ = decode(m)
	if s.Tanks[0].HP <= 40 || s.Turn != 1 {
		t.Fatalf("bot item did not heal in place: hp=%d turn=%d", s.Tanks[0].HP, s.Turn)
	}
	raw, err = botAction(ctx(2, 0, time.Unix(1700000006, 0)), m)
	if err != nil {
		t.Fatal(err)
	}
	var second Action
	_ = json.Unmarshal(raw, &second)
	if second.Type != "fire" {
		t.Fatalf("bot second action = %+v", second)
	}
}

func TestItemsSurviveRestore(t *testing.T) {
	s, m := playingMatch(t, 2)
	s.Tanks[0].Items = []int{1, 26}
	m, _ = s.envelope()
	s2, err := decode(m)
	if err != nil {
		t.Fatal(err)
	}
	if len(s2.Tanks[0].Items) != 2 || s2.Tanks[0].Items[0] != 1 || s2.Tanks[0].Items[1] != 26 {
		t.Fatalf("items after restore = %v", s2.Tanks[0].Items)
	}
}
