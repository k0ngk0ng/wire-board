package game

import (
	"encoding/json"
	"errors"
	"fmt"
)

const (
	catanEventNormalCards = 36
	catanEventNewYear     = catanEventNormalCards
	catanEventBottom      = 5
)

// catanEventDeck implements the 2025 T&B deck lifecycle only. Normal card IDs
// are opaque slots, NOT production numbers or a verified card catalogue.
// catanEventSession binds slots to a legacy reference or explicitly labelled
// site catalogue. Neither claims to verify the physical 2025 card faces.
// DrawPile is stored bottom first; New Year always sits above five hidden cards.
type catanEventDeck struct {
	DrawPile []int  `json:"drawPile"`
	Discard  []int  `json:"discard"` // Chronological, most recently revealed last.
	Cycle    uint64 `json:"cycle"`
}

type catanEventDraw struct {
	Card    int
	NewYear bool // A new deck was created before drawing this normal card.
}

// catanEventDeckView contains only information revealed through normal play.
// Never serialize catanEventDeck itself into a player's or spectator's view.
type catanEventDeckView struct {
	Cycle        uint64 `json:"cycle"`
	Revealed     []int  `json:"revealed"`
	Current      *int   `json:"current,omitempty"`
	Remaining    int    `json:"remaining"`    // Normal cards, including the hidden five.
	UntilNewYear int    `json:"untilNewYear"` // Normal draws before the next reshuffle.
}

func newCatanEventDeck() catanEventDeck {
	d := catanEventDeck{Cycle: 1}
	d.shuffle()
	return d
}

func (d *catanEventDeck) shuffle() {
	normal := make([]int, catanEventNormalCards)
	for i := range normal {
		normal[i] = i
	}
	shuffle(normal)
	d.DrawPile = make([]int, 0, catanEventNormalCards+1)
	d.DrawPile = append(d.DrawPile, normal[:catanEventBottom]...)
	d.DrawPile = append(d.DrawPile, catanEventNewYear)
	d.DrawPile = append(d.DrawPile, normal[catanEventBottom:]...)
	d.Discard = []int{}
}

func (d catanEventDeck) validate() error {
	if d.Cycle == 0 || len(d.DrawPile) < catanEventBottom+1 || len(d.DrawPile) > catanEventNormalCards+1 ||
		len(d.DrawPile)+len(d.Discard) != catanEventNormalCards+1 {
		return errors.New("invalid CATAN event deck size or cycle")
	}
	if d.DrawPile[catanEventBottom] != catanEventNewYear {
		return errors.New("invalid CATAN New Year position")
	}
	var seen [catanEventNormalCards + 1]bool
	for _, pile := range [][]int{d.DrawPile, d.Discard} {
		for _, id := range pile {
			if id < 0 || id > catanEventNewYear || seen[id] {
				return fmt.Errorf("invalid or repeated CATAN event card: %d", id)
			}
			seen[id] = true
		}
	}
	return nil
}

// UnmarshalJSON rejects corrupt saves without partially overwriting live state.
func (d *catanEventDeck) UnmarshalJSON(data []byte) error {
	type savedDeck catanEventDeck
	var saved savedDeck
	if err := json.Unmarshal(data, &saved); err != nil {
		return err
	}
	next := catanEventDeck(saved)
	if err := next.validate(); err != nil {
		return err
	}
	*d = next
	return nil
}

func (d *catanEventDeck) next() (catanEventDraw, error) {
	if err := d.validate(); err != nil {
		return catanEventDraw{}, err
	}
	result := catanEventDraw{}
	if d.DrawPile[len(d.DrawPile)-1] == catanEventNewYear {
		if d.Cycle == ^uint64(0) {
			return result, errors.New("CATAN event deck cycle overflow")
		}
		d.shuffle()
		d.Cycle++
		result.NewYear = true
	}
	last := len(d.DrawPile) - 1
	result.Card = d.DrawPile[last]
	d.DrawPile = d.DrawPile[:last]
	d.Discard = append(d.Discard, result.Card)
	return result, nil
}

func (d catanEventDeck) view() (catanEventDeckView, error) {
	if err := d.validate(); err != nil {
		return catanEventDeckView{}, err
	}
	v := catanEventDeckView{
		Cycle: d.Cycle, Revealed: append([]int{}, d.Discard...),
		Remaining: len(d.DrawPile) - 1, UntilNewYear: len(d.DrawPile) - catanEventBottom - 1,
	}
	if len(d.Discard) > 0 {
		current := d.Discard[len(d.Discard)-1]
		v.Current = &current
	}
	return v, nil
}
