package game

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestRailDestinationProgressIsPrivateAndReadOnly(t *testing.T) {
	s := railLogGame(t)
	routes := MapData().Routes
	var first, second Route
	for _, a := range routes {
		for _, b := range routes {
			if a.B == b.A && a.A != b.B {
				first, second = a, b
				break
			}
		}
		if first.ID != 0 {
			break
		}
	}
	ticket := Ticket{ID: 999, A: first.A, B: second.B, Points: 7}
	s.Rail.Players[0].Tickets = []Ticket{ticket}
	s.Rail.Players[1].Tickets = []Ticket{ticket}
	s.Rail.Owners[first.ID] = 0
	s.Rail.Owners[second.ID] = 1 // Another player's track cannot complete our destination.
	view := func(player int) map[string]any {
		return s.View(player)["rail"].(map[string]any)["players"].([]any)[0].(map[string]any)
	}
	if view(0)["tickets"].([]Ticket)[0].Complete {
		t.Fatal("opponent's track counted")
	}
	s.Rail.Owners[second.ID] = 0
	before, _ := json.Marshal(s)
	own := view(0)
	if !own["tickets"].([]Ticket)[0].Complete || own["completed"] != 1 {
		t.Fatal("completed connection missing", own)
	}
	for _, viewer := range []int{1, -1} {
		other := view(viewer)
		if _, ok := other["tickets"]; ok {
			t.Fatal("tickets leaked")
		}
		if _, ok := other["completed"]; ok {
			t.Fatal("completion count leaked")
		}
	}
	after, _ := json.Marshal(s)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("view mutated saved game")
	}
	// A changed private destination cannot change any public player's fields.
	publicBefore := view(1)
	s.Rail.Players[0].Tickets[0].B = first.A
	if !reflect.DeepEqual(publicBefore, view(1)) {
		t.Fatal("private progress exposed in public fields")
	}
	s.Rail.Players[0].Tickets[0] = ticket
	s.railFinish()
	if s.Rail.Players[0].Completed != 1 || s.Rail.Players[0].TicketScore != 7 {
		t.Fatal("progress affected final scoring")
	}
	finished := view(1)
	if finished["completed"] != float64(1) || !finished["tickets"].([]any)[0].(map[string]any)["complete"].(bool) {
		t.Fatal("final results not public")
	}
}
