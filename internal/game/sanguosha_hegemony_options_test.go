package game

import "testing"

func TestSanguoshaHegemonyOptionsStayIndependent(t *testing.T) {
	for _, bad := range []SGOptions{
		{Mode: "hegemony", Deck: "standard"}, {Mode: "hegemony", Deck: "military"},
		{Mode: "hegemony", StandardVersion: "breakthrough"}, {Mode: "hegemony", StandardVersion: "classic"},
		{Mode: "hegemony", Packs: []string{"hegemony", "god"}}, {Mode: "identity", Deck: "hegemony"},
		{Mode: "identity", Packs: []string{"hegemony"}},
	} {
		if _, err := NormalizeSGOptions(bad); err == nil {
			t.Fatal("mixed rules accepted", bad)
		}
	}
	o, err := NormalizeSGOptions(SGOptions{Mode: "hegemony"})
	if err != nil || o.Deck != "hegemony" || len(o.Packs) != 1 || o.Packs[0] != "hegemony" || o.StandardVersion != "" {
		t.Fatal(o, err)
	}
	if _, err := NormalizeSGOptions(o); err != nil {
		t.Fatal("canonical configuration does not round-trip", err)
	}
	for _, n := range []int{0, 3, 9} {
		if _, err := NewSanguosha(n, o); err == nil {
			t.Fatal("invalid seat count", n)
		}
	}
	s, err := NewSanguosha(4, SGOptions{})
	if err != nil || s.Sanguosha.Hegemony != nil || len(s.Sanguosha.Deck) != 108 {
		t.Fatal("classic default changed", err)
	}
}
