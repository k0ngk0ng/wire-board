package fortressblue

import (
	"fmt"
	fbdata "github.com/k0ngk0ng/wire-board/games/fortressblue/data"
	fbmaps "github.com/k0ngk0ng/wire-board/games/fortressblue/maps"
	"math"
)

// Delivery graphics/contact come from the original client. The 8–12 turn cadence,
// item pool and flight milliseconds are explicit platform reconstruction values;
// the original server chooses the drop packet and is not in the retained client.
type Supply struct {
	PickupAt   int64   `json:"pickupAt,omitempty"`
	PickupSeat int     `json:"pickupSeat,omitempty"`
	ID         int64   `json:"id"`
	ItemID     int     `json:"itemId"`
	X          float64 `json:"x"`
	Y          float64 `json:"y"`
	FromX      float64 `json:"fromX"`
	At         int64   `json:"at"`
	DropAt     int64   `json:"dropAt"`
	LandAt     int64   `json:"landAt"`
	Landed     bool    `json:"landed,omitempty"`
}

const maxSupplies = 8

func scheduleSupply(s *State, at int64) {
	if s.Finished || s.Phase != PhasePlaying || s.Resolution != nil || s.NextSupplyTurn <= 0 || s.Turn < s.NextSupplyTurn {
		return
	}
	random := hashString(s.Epoch, "supply", fmt.Sprint(s.Turn))
	s.NextSupplyTurn = s.Turn + 8 + int(random%5)
	if len(s.Supplies) >= maxSupplies {
		return
	}
	stage, _ := fbmaps.ByID(s.MapID)
	mask := maskFor(stage, s.Craters)
	alive := []int{}
	for i, t := range s.Tanks {
		if t.Alive {
			alive = append(alive, i)
		}
	}
	if len(alive) == 0 {
		return
	}
	// Select a live part of the stage, never a fixed reward for the current player.
	anchor := s.Tanks[alive[int(random>>3)%len(alive)]]
	var x, y float64
	found := false
	for attempt := 0; attempt < 24; attempt++ {
		h := hashString(s.Epoch, "supply-position", fmt.Sprint(s.Turn), fmt.Sprint(attempt))
		offset := float64(30 + int(h%31))
		if h&64 != 0 {
			offset = -offset
		}
		x = clamp(anchor.X+offset, 20, float64(stage.Width)-20)
		y = groundY(stage, mask, x, 0)
		if y >= 65 && y < float64(stage.Height)-8 {
			found = true
			break
		}
	}
	if !found {
		return
	}
	pool := fbdata.ItemIDs()
	if len(pool) == 0 {
		return
	}
	flight := int64(math.Sqrt(2*(y-45)/300) * 1000)
	dropAt := at + 1000
	landAt := dropAt + max(int64(650), flight)
	d := Supply{ID: s.Sequence + 1, ItemID: pool[int(random>>12)%len(pool)], X: x, Y: y, FromX: clamp(x-float64(s.Wind)*8, 20, float64(stage.Width)-20), At: at, DropAt: dropAt, LandAt: landAt}
	s.Supplies = append(s.Supplies, d)
	s.emit("supply_drop", -1, -1, x, y, 0, "随机补给运输中", at)
	s.note("补给直升机正在运输物资，木箱落地后可移动拾取")
}
func (s *State) supplyInFlight() bool {
	for _, d := range s.Supplies {
		if !d.Landed {
			return true
		}
	}
	return false
}
func (s *State) supplyDeadline() int64 {
	next := int64(0)
	for _, d := range s.Supplies {
		if !d.Landed && (next == 0 || d.LandAt < next) {
			next = d.LandAt
		}
		if d.Landed && d.PickupAt > 0 && (next == 0 || d.PickupAt < next) {
			next = d.PickupAt
		}
	}
	return next
}
func processSupplies(s *State, at int64) {
	if s.Finished || len(s.Supplies) == 0 {
		return
	}
	stage, _ := fbmaps.ByID(s.MapID)
	mask := maskFor(stage, s.Craters)
	wasInFlight := s.supplyInFlight()
	keep := s.Supplies[:0]
	for _, d := range s.Supplies {
		if !d.Landed && at >= d.LandAt {
			d.Landed = true
			s.emit("supply_land", -1, -1, d.X, d.Y, 0, "补给已落地", d.LandAt)
		}
		if !d.Landed {
			keep = append(keep, d)
			continue
		}
		// The original destroys unsupported ground items. This reconstruction removes
		// them without inventing the original crate-explosion damage formula.
		if math.Abs(groundY(stage, mask, d.X, d.Y-1)-d.Y) > 4 {
			s.emit("supply_lost", -1, -1, d.X, d.Y, 0, "补给随地形损毁", at)
			continue
		}

		picked := false
		canReceive := func(seat int) bool {
			return seat >= 0 && seat < len(s.Tanks) && s.Tanks[seat].Alive && len(s.Tanks[seat].Items) < MaxItems
		}
		touching := func(seat int) bool {
			if !canReceive(seat) || s.Resolution != nil {
				return false
			}
			t := s.Tanks[seat]
			return math.Abs(t.X-d.X) <= 26 && t.Y >= d.Y-8 && t.Y <= d.Y+2
		}
		if d.PickupAt > 0 && !canReceive(d.PickupSeat) {
			d.PickupAt = 0
			d.PickupSeat = 0
		}
		if d.PickupAt == 0 {
			for seat := range s.Tanks {
				if touching(seat) {
					d.PickupSeat = seat
					d.PickupAt = at + 180
					break
				}
			}
		}
		// Contact is latched immediately. The 180ms delay only synchronizes the
		// inventory with movement rendering; leaving the box cannot undo a touch.
		if d.PickupAt > 0 && at >= d.PickupAt && canReceive(d.PickupSeat) {
			seat := d.PickupSeat
			t := &s.Tanks[seat]
			t.Items = append(t.Items, d.ItemID)
			item, _ := fbdata.ItemByID(d.ItemID)
			s.emit("supply_pickup", seat, -1, d.X, d.Y, d.ItemID, item.Name, d.PickupAt)
			s.note(fmt.Sprintf("%d 号位拾取补给：%s", seat+1, item.Name))
			picked = true
		}
		if !picked {
			keep = append(keep, d)
		}
	}
	s.Supplies = keep
	if wasInFlight && !s.supplyInFlight() && s.Deadline == 0 && s.Resolution == nil {
		s.Deadline = at + int64(s.Options.TurnSeconds)*1000
		s.resumeSupply = true
	}
}
func botSupplyMove(s *State, seat int, at int64) int {
	t := s.Tanks[seat]
	if len(t.Items) >= MaxItems || t.Fuel <= 0 {
		return 0
	}
	nearest := math.Inf(1)
	direction := 0
	risk := botGroundRisk(s, seat, at)
	for _, d := range s.Supplies {
		distance := math.Abs(d.X - t.X)
		if !d.Landed || distance <= 26 || distance > t.Fuel+26 || distance >= nearest || math.Abs(d.Y-t.Y) > 8 {
			continue
		}
		dir := 1
		if d.X < t.X {
			dir = -1
		}
		candidate := *s
		candidate.Tanks = append([]Tank(nil), s.Tanks...)
		candidate.Events = nil
		if moveTank(&candidate, seat, dir, at) && candidate.Resolution == nil && botGroundRisk(&candidate, seat, at) <= risk && math.Abs(candidate.Tanks[seat].X-d.X) < distance {
			nearest, direction = distance, dir
		}
	}
	return direction
}
