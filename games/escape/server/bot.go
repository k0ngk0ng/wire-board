package escape

import (
	"encoding/json"
	"fmt"
	"math"

	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

// The planner receives the same detached, visibility-restricted projection as
// a human. No generation seed, hidden room or puzzle solution reaches plan.
func botAction(ctx gameapi.Context, m gameapi.Match) (json.RawMessage, error) {
	s, e := decode(m)
	if e != nil {
		return nil, e
	}
	if ctx.Actor < 0 || ctx.Actor >= len(s.Players) || s.Phase != "playing" || s.Paused {
		return nil, nil
	}
	v := s.view(ctx.Actor)
	a := plan(v, ctx.Actor)
	if a == nil {
		return nil, nil
	}
	a.Epoch = v.Epoch
	a.Seq = v.Players[ctx.Actor].Seq + 1
	return rawAction(*a), nil
}

type botMap struct {
	v   View
	who int
	me  Player
}

func (b botMap) room(id string) *Room {
	for i := range b.v.Rooms {
		if b.v.Rooms[i].ID == id {
			return &b.v.Rooms[i]
		}
	}
	return nil
}
func (b botMap) done(key string) bool {
	for _, o := range b.v.Objectives {
		if o.ID == key {
			return o.Done
		}
	}
	return false
}
func (b botMap) device(kind string) (*Room, *Device) {
	for i := range b.v.Rooms {
		r := &b.v.Rooms[i]
		if !r.Known {
			continue
		}
		for j := range r.Objects {
			if r.Objects[j].Kind == kind && !r.Objects[j].Done {
				return r, &r.Objects[j]
			}
		}
	}
	return nil, nil
}
func (b botMap) path(to string) []string {
	queue := [][]string{{b.me.Room}}
	seen := map[string]bool{b.me.Room: true}
	for len(queue) > 0 {
		path := queue[0]
		queue = queue[1:]
		last := path[len(path)-1]
		if last == to {
			return path
		}
		r := b.room(last)
		if r == nil || !r.Known {
			continue
		}
		for _, d := range r.Doors {
			if !seen[d.Room] {
				seen[d.Room] = true
				next := append(append([]string{}, path...), d.Room)
				queue = append(queue, next)
			}
		}
	}
	return nil
}
func (b botMap) toward(room string) *Action {
	path := b.path(room)
	if len(path) > 1 {
		return &Action{Type: "travel", Target: path[1]}
	}
	return nil
}
func (b botMap) work(kind string) *Action {
	r, d := b.device(kind)
	if r == nil {
		return b.explore()
	}
	if r.ID != b.me.Room {
		return b.toward(r.ID)
	}
	return &Action{Type: "interact", Target: d.ID}
}
func (b botMap) explore() *Action {
	best := ""
	score := math.Inf(1)
	for i := 0; i < len(b.v.Rooms); i++ {
		r := b.v.Rooms[(i+b.who*3)%len(b.v.Rooms)]
		if r.Known {
			continue
		}
		path := b.path(r.ID)
		if len(path) < 2 {
			continue
		}
		s := float64(len(path)*10) + float64(i)*.1
		if s < score {
			score = s
			best = r.ID
		}
	}
	if best != "" {
		return b.toward(best)
	}
	return nil
}
func (b botMap) solve(kind string) *Action {
	var clue *Clue
	for i := range b.v.Journal {
		if b.v.Journal[i].Key == kind {
			clue = &b.v.Journal[i]
		}
	}
	if clue == nil {
		return b.work("clue-" + kind)
	}
	r, d := b.device(kind)
	if r == nil {
		return b.explore()
	}
	if r.ID != b.me.Room {
		return b.toward(r.ID)
	}
	if distance(b.me.X, b.me.Y, d.X, d.Y) > 75 {
		return &Action{Type: "inspect", Target: d.ID}
	}
	for i, n := range d.Dials {
		if i < len(clue.Values) && n != clue.Values[i] {
			return &Action{Type: "turn", Target: d.ID, Index: i}
		}
	}
	return &Action{Type: "interact", Target: d.ID}
}
func (b botMap) circuit(key string) *Action {
	var room *Room
	var d *Device
	for i := range b.v.Rooms {
		r := &b.v.Rooms[i]
		if !r.Known {
			continue
		}
		for j := range r.Objects {
			if r.Objects[j].Kind == "power" {
				room = r
				d = &r.Objects[j]
			}
		}
	}
	if room == nil {
		return b.explore()
	}
	if room.ID != b.me.Room {
		return b.toward(room.ID)
	}
	if distance(b.me.X, b.me.Y, d.X, d.Y) > 75 {
		return &Action{Type: "inspect", Target: d.ID}
	}
	target := key
	if len(b.v.Circuits) >= 2 {
		for _, active := range b.v.Circuits {
			if active != "lights" && active != key {
				target = active
				break
			}
		}
	}
	return &Action{Type: "circuit", Target: d.ID, Circuit: target}
}
func plan(v View, who int) *Action {
	b := botMap{v, who, v.Players[who]}
	if b.me.HP <= 0 || b.me.Goal != nil {
		return nil
	}
	if b.me.HP < 65 && v.Supplies.Meds > 0 {
		return &Action{Type: "med"}
	}
	if v.Power < 45 && v.Supplies.Cells > 0 {
		return &Action{Type: "cell"}
	}
	for _, p := range v.Players {
		if p.Seat != who && p.HP <= 0 {
			if p.Room != b.me.Room {
				return b.toward(p.Room)
			}
			return &Action{Type: "rescue", Target: fmt.Sprint("player-", p.Seat)}
		}
	}
	if v.Incident != nil && v.Incident.Room == b.me.Room && v.Tick >= v.Incident.Begins-3000 && v.Tick < v.Incident.Ends {
		r := b.room(b.me.Room)
		if len(r.Doors) > 0 {
			return &Action{Type: "travel", Target: r.Doors[who%len(r.Doors)].Room}
		}
	}
	if b.me.Carry != "" {
		return b.work("extract")
	}
	if v.Rally != nil && v.Tick < v.Rally.Until {
		if b.me.Room != v.Rally.Room {
			return b.toward(v.Rally.Room)
		}
		return nil
	}
	wanted := "lift"
	if v.Options.Scenario == "archive" && v.Stats.Route != "lift" || v.Stats.Route == "passage" || v.Stats.Route == "boat" {
		wanted = "pump"
	}
	hasCircuit := false
	for _, key := range v.Circuits {
		hasCircuit = hasCircuit || key == wanted
	}
	if !hasCircuit {
		return b.circuit(wanted)
	}
	available := false
	for _, r := range v.Routes {
		available = available || r.Available && (v.Stats.Route == "" || v.Stats.Route == r.ID)
	}
	if available {
		if b.me.Ready {
			return nil
		}
		return b.work("extract")
	}
	if len(v.Journal) == 0 {
		return b.work("briefing")
	}
	if v.Options.Scenario == "archive" && v.Stats.Route != "lift" {
		if v.Supplies.Documents < 2 {
			if _, d := b.device("document"); d != nil {
				return b.work("document")
			}
			if a := b.explore(); a != nil {
				return a
			}
		}
		if !b.done("pump") {
			return b.solve("pump")
		}
	} else {
		if v.Stats.Route == "passage" {
			if !b.done("pump") {
				return b.solve("pump")
			}
			if !b.done("signal") {
				return b.solve("signal")
			}
		} else if !b.done("power") {
			if v.Options.Scenario == "station" && v.Supplies.Fuses < 2 {
				return b.work("supply")
			}
			return b.solve("power")
		}
	}
	// Independent latches in independent rooms; a bot holds its end until its
	// teammate arrives. Re-arming expires after twelve seconds by ordinary rules.
	if !b.done("relay") {
		kind := "relay"
		if who%2 == 1 {
			kind = "exit"
		}
		var room *Room
		var device *Device
		for i := range v.Rooms {
			r := &v.Rooms[i]
			if !r.Known || r.Kind != kind {
				continue
			}
			for j := range r.Objects {
				d := &r.Objects[j]
				if d.Kind == "relay" {
					room = r
					device = d
				}
			}
		}
		if room == nil {
			return b.explore()
		}
		if room.ID != b.me.Room {
			return b.toward(room.ID)
		}
		if device.HeldUntil > v.Tick+3000 && distance(b.me.X, b.me.Y, device.X, device.Y) < 90 {
			return nil
		}
		return &Action{Type: "interact", Target: device.ID}
	}
	if a := b.work("extract"); a != nil {
		return a
	}
	return b.explore()
}
