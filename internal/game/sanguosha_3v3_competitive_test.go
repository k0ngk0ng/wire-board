package game

import (
	"encoding/json"
	"slices"
	"strconv"
	"testing"
)

// Failure scenarios, specified before the official 3v3 configuration:
//   - official rooms do not restrict the general pool to the standard 25 plus
//     the seven wind generals (Yuji and the god pack excluded);
//   - the official deck keeps Lightning or mixes the military cards in;
//   - the pile stage accepts the same pile twice or an out-of-turn pick;
//   - the face-down pile contents leak to the table before both leaders chose;
//   - the laid-out pool is not the union of the two chosen piles;
//   - a save taken during the pile stage restores a different pile or turn;
//   - the site extension or the identity mode inherits the official limits.

func sgThreePilesState(t *testing.T) *State {
	t.Helper()
	s, err := NewSanguosha(6, SGOptions{Mode: "3v3"})
	if err != nil {
		t.Fatal(err)
	}
	if s.Sanguosha.ThreeV3.Stage != "piles" {
		t.Fatalf("official 3v3 must open on the pile stage, got %s", s.Sanguosha.ThreeV3.Stage)
	}
	return s
}

// sgThreePilePicks drives one pile choice through the public prompt.
func sgThreePilePicks(t *testing.T, s *State, camp int, choice string) {
	t.Helper()
	turn := s.Sanguosha.ThreeV3.PileTurn
	if turn != camp {
		t.Fatalf("pile turn is camp %d, wanted camp %d", turn, camp)
	}
	leader := s.Sanguosha.ThreeV3.Leaders[camp]
	q := s.Sanguosha.Pending
	if q == nil || q.Kind != "sg_3v3_pile" || q.Player != leader {
		t.Fatalf("no pile prompt for camp %d: %+v", camp, q)
	}
	sgDo(t, s, leader, Action{Type: "sg_respond", Choice: choice})
}

func sgThreeView(t *testing.T, s *State, viewer int) map[string]any {
	t.Helper()
	return s.View(viewer)["sanguosha"].(map[string]any)
}

func TestSanguoshaThreeV3OfficialConfiguration(t *testing.T) {
	t.Run("pool is the printed 32 and the deck drops Lightning", func(t *testing.T) {
		s := sgThreePilesState(t)
		g := s.Sanguosha
		three := g.ThreeV3
		if three.Variant != SGThreeOfficial {
			t.Fatal("default variant must be the official one", three.Variant)
		}
		if g.Options.StandardVersion != "classic" || g.Options.Deck != "standard" {
			t.Fatal("official draft must fix the classic standard deck", g.Options)
		}
		if len(g.Deck) != 106 {
			t.Fatalf("official deck must hold 106 cards, got %d", len(g.Deck))
		}
		for _, id := range g.Deck {
			if sgCard(id).Kind == "lightning" {
				t.Fatal("official deck must not contain Lightning", id)
			}
		}
		if len(three.Piles) != 4 {
			t.Fatalf("official 3v3 lays out four piles, got %d", len(three.Piles))
		}
		seen := map[string]bool{}
		for i, pile := range three.Piles {
			if len(pile) != 8 {
				t.Fatalf("pile %d holds %d generals, want 8", i, len(pile))
			}
			for _, id := range pile {
				if seen[id] {
					t.Fatal("general appears in two piles", id)
				}
				seen[id] = true
				if sgGeneral(id).ID == "" {
					t.Fatal("unknown general in a pile", id)
				}
			}
		}
		if len(seen) != 32 {
			t.Fatalf("official pool holds %d generals, want 32", len(seen))
		}
		for _, id := range []string{"yuji", "shenguanyu"} {
			if seen[id] {
				t.Fatal("official pool must exclude", id)
			}
		}
		if !slices.Equal(three.PilePicks, []int{-1, -1}) || three.PileTurn != sgThreeWarm {
			t.Fatal("opening pile bookkeeping", three.PilePicks, three.PileTurn)
		}
	})

	t.Run("face-down piles stay hidden until both are chosen", func(t *testing.T) {
		s := sgThreePilesState(t)
		check := func(who int) {
			t.Helper()
			draft := sgThreeView(t, s, who)["threeV3"].(map[string]any)
			if _, ok := draft["piles"]; ok {
				t.Fatalf("viewer %d saw the face-down piles: %v", who, draft["piles"])
			}
			sizes, ok := draft["pileSizes"].([]int)
			if !ok || !slices.Equal(sizes, []int{8, 8, 8, 8}) {
				t.Fatalf("viewer %d pile sizes %v", who, draft["pileSizes"])
			}
		}
		check(-1)
		coldLeader := s.Sanguosha.ThreeV3.Leaders[sgThreeCold]
		check(coldLeader)
		sgThreePilePicks(t, s, sgThreeWarm, "1")
		check(coldLeader)
		if _, ok := sgThreeView(t, s, s.Sanguosha.ThreeV3.Leaders[sgThreeWarm])["threeV3"].(map[string]any)["piles"]; ok {
			t.Fatal("the first pick must not reveal the piles")
		}
	})

	t.Run("illegal pile choices are rejected without moving the stage", func(t *testing.T) {
		s := sgThreePilesState(t)
		coldLeader := s.Sanguosha.ThreeV3.Leaders[sgThreeCold]
		q := s.Sanguosha.Pending
		if err := s.Apply(coldLeader, Action{Type: "sg_respond", Prompt: q.ID, Choice: "2"}); err == nil {
			t.Fatal("the camp without the turn picked a pile")
		}
		for _, bad := range []string{"", "0", "5", "x"} {
			if err := s.Apply(s.Sanguosha.ThreeV3.Leaders[sgThreeWarm], Action{Type: "sg_respond", Prompt: q.ID, Choice: bad}); err == nil {
				t.Fatalf("pile choice %q was accepted", bad)
			}
		}
		if stage := s.Sanguosha.ThreeV3.Stage; stage != "piles" {
			t.Fatal("illegal picks must not advance the stage", stage)
		}
		sgThreePilePicks(t, s, sgThreeWarm, "2")
		if got := s.Sanguosha.ThreeV3.PilePicks[sgThreeWarm]; got != 1 {
			t.Fatal("warm pile recorded wrong", got)
		}
		q = s.Sanguosha.Pending
		if err := s.Apply(coldLeader, Action{Type: "sg_respond", Prompt: q.ID, Choice: "2"}); err == nil {
			t.Fatal("a chosen pile was taken twice")
		}
		if stage := s.Sanguosha.ThreeV3.Stage; stage != "piles" {
			t.Fatal("a repeated pick must not advance the stage", stage)
		}
	})

	t.Run("the laid-out pool is the union of the two chosen piles", func(t *testing.T) {
		s := sgThreePilesState(t)
		warmPile := slices.Clone(s.Sanguosha.ThreeV3.Piles[2])
		coldPile := slices.Clone(s.Sanguosha.ThreeV3.Piles[3])
		sgThreePilePicks(t, s, sgThreeWarm, "3")
		sgThreePilePicks(t, s, sgThreeCold, "4")
		three := s.Sanguosha.ThreeV3
		if three.Stage != "pickFirst" {
			t.Fatal("both picks must move on to the first-pick choice", three.Stage)
		}
		if want := append(clone(warmPile), coldPile...); !slices.Equal(three.Pool, want) {
			t.Fatalf("laid-out pool %v, want %v", three.Pool, want)
		}
		if len(three.Pool) != 16 {
			t.Fatal("the laid-out pool must hold 16 generals", len(three.Pool))
		}
		draft := sgThreeView(t, s, -1)["threeV3"].(map[string]any)
		piles, ok := draft["piles"].([][]string)
		if !ok || len(piles) != 4 {
			t.Fatalf("chosen piles must be public afterwards: %v", draft["piles"])
		}
		if !slices.Equal(draft["pilePicks"].([]int), []int{3, 2}) {
			t.Fatal("public pile picks", draft["pilePicks"])
		}
	})

	t.Run("a save on the pile stage keeps pile and turn", func(t *testing.T) {
		s := sgThreePilesState(t)
		sgThreePilePicks(t, s, sgThreeWarm, "4")
		raw, err := json.Marshal(s)
		if err != nil {
			t.Fatal(err)
		}
		restored := &State{}
		if err = json.Unmarshal(raw, restored); err != nil {
			t.Fatal(err)
		}
		three := restored.Sanguosha.ThreeV3
		if three.Stage != "piles" || three.PileTurn != sgThreeCold || !slices.Equal(three.PilePicks, []int{-1, 3}) {
			t.Fatalf("restored pile stage %s turn %d picks %v", three.Stage, three.PileTurn, three.PilePicks)
		}
		if got := restored.Sanguosha.Pending; got == nil || got.Kind != "sg_3v3_pile" || got.Player != three.Leaders[sgThreeCold] {
			t.Fatalf("restored pile prompt %+v", got)
		}
		sgThreePilePicks(t, restored, sgThreeCold, "1")
		if restored.Sanguosha.ThreeV3.Stage != "pickFirst" {
			t.Fatal("restored table could not finish the pile stage")
		}
	})
}

func TestSanguoshaThreeV3ExtensionAndIdentityStayUnrestricted(t *testing.T) {
	t.Run("the site extension keeps the chosen packs and deck", func(t *testing.T) {
		s, err := NewSanguosha(6, SGOptions{Mode: "3v3", ThreeVariant: SGThreeExtended, Deck: "military", Packs: []string{"standard", "fire"}})
		if err != nil {
			t.Fatal(err)
		}
		three := s.Sanguosha.ThreeV3
		if three.Variant != SGThreeExtended || three.Stage != "pickFirst" {
			t.Fatal("extended opening", three.Variant, three.Stage)
		}
		if len(three.Piles) != 0 || len(three.Pool) != 16 {
			t.Fatal("extended lays out sixteen generals directly", len(three.Piles), len(three.Pool))
		}
		if len(s.Sanguosha.Deck) != len(sgCards)+len(sgMilitaryCards) {
			t.Fatalf("extended deck must keep the military cards: %d", len(s.Sanguosha.Deck))
		}
		lightning := false
		for _, id := range s.Sanguosha.Deck {
			lightning = lightning || sgCard(id).Kind == "lightning"
		}
		if !lightning {
			t.Fatal("extended deck lost Lightning")
		}
		catalog := map[string]bool{}
		for _, general := range s.Sanguosha.generalCatalog() {
			catalog[general.ID] = true
		}
		if !catalog["huangyueying"] {
			t.Fatal("catalog fixture changed")
		}
		for _, id := range three.Pool {
			if !catalog[id] {
				t.Fatal("extension pool left the selected catalog", id)
			}
		}
		fire := sgFireGenerals[0].ID
		if !catalog[fire] {
			t.Fatal("the fire pack did not reach the catalog", fire)
		}
	})

	t.Run("identity keeps the printed 108 deck", func(t *testing.T) {
		s, err := NewSanguosha(6, SGOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if len(s.Sanguosha.Deck) != 108 || s.Sanguosha.ThreeV3 != nil {
			t.Fatal("identity mode inherited the official 3v3 limits", len(s.Sanguosha.Deck))
		}
	})

	if _, err := NormalizeSGOptions(SGOptions{Mode: "3v3", ThreeVariant: "house"}); err == nil {
		t.Error("an unknown 3v3 variant was accepted")
	}
	if _, err := NormalizeSGOptions(SGOptions{Mode: "identity", ThreeVariant: SGThreeExtended}); err == nil {
		t.Error("the 3v3 variant was accepted outside 3v3")
	}
}

func TestSanguoshaThreeV3PilePromptAndBots(t *testing.T) {
	s := sgThreePilesState(t)
	warmLeader := s.Sanguosha.ThreeV3.Leaders[sgThreeWarm]
	prompt, ok := sgThreeView(t, s, warmLeader)["pending"].(map[string]any)
	if !ok || prompt["kind"] != "sg_3v3_pile" {
		t.Fatalf("warm leader prompt %v", sgThreeView(t, s, warmLeader)["pending"])
	}
	if got := prompt["choices"].([]string); !slices.Equal(got, []string{"1", "2", "3", "4"}) {
		t.Fatal("warm pile choices", got)
	}
	spectator, ok := sgThreeView(t, s, -1)["pending"].(map[string]any)
	if !ok || spectator["canRespond"] != false {
		t.Fatalf("spectator pile prompt %v", sgThreeView(t, s, -1)["pending"])
	}
	if _, ok := spectator["choices"]; ok {
		t.Fatal("a spectator must not receive the pile choices")
	}
	first, ok := s.sgThreeBotAction(warmLeader)
	if !ok || !slices.Contains([]string{"1", "2", "3", "4"}, first.Choice) {
		t.Fatalf("warm bot pile action %+v %v", first, ok)
	}
	sgThreePilePicks(t, s, sgThreeWarm, "1")
	coldLeader := s.Sanguosha.ThreeV3.Leaders[sgThreeCold]
	prompt, ok = sgThreeView(t, s, coldLeader)["pending"].(map[string]any)
	if !ok {
		t.Fatal("cold leader received no pile prompt")
	}
	if got := prompt["choices"].([]string); !slices.Equal(got, []string{"2", "3", "4"}) {
		t.Fatal("cold pile choices must drop the taken pile", got)
	}
	second, ok := s.sgThreeBotAction(coldLeader)
	if !ok || second.Choice == "1" {
		t.Fatalf("cold bot pile action %+v %v", second, ok)
	}
	sgThreePilePicks(t, s, sgThreeCold, "3")
	if stage := s.Sanguosha.ThreeV3.Stage; stage != "pickFirst" {
		t.Fatal("bots could not finish the pile stage", stage)
	}
	askPile := strconv.Itoa(len(s.Sanguosha.ThreeV3.Piles))
	if askPile != "4" {
		t.Fatal("pile count changed", askPile)
	}
}
