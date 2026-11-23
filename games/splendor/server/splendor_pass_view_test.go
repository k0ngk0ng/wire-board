package splendor

import "testing"

func TestPassVisibilityRequiresNoLegalAction(t *testing.T) {
	s, err := New("splendor", 2)
	if err != nil {
		t.Fatal(err)
	}
	canPass := func(player int) bool {
		g := s.View(player)["splendor"].(map[string]any)
		allowed, _ := g["canPass"].(bool)
		return allowed
	}
	if canPass(s.Turn) {
		t.Fatal("new game has legal actions")
	}
	g := s.Splendor
	g.Bank = make([]int, 6)
	for tier := range g.Decks {
		g.Decks[tier] = nil
		g.Market[tier] = nil
	}
	if !canPass(s.Turn) {
		t.Fatal("no moves must offer pass")
	}
	if canPass(s.Turn ^ 1) {
		t.Fatal("waiting player cannot pass")
	}
	s.Phase = "noble"
	if canPass(s.Turn) {
		t.Fatal("special phase cannot pass")
	}
	s.Phase = "turn"
	s.Finished = true
	if canPass(s.Turn) {
		t.Fatal("finished game cannot pass")
	}
	s.Finished = false
	g.Bank[0] = 1
	if canPass(s.Turn) {
		t.Fatal("available gem is a legal action")
	}
}
