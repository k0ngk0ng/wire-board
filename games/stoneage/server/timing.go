package stoneage

import (
	"encoding/json"
	"math"
	"strconv"
	"sync"

	native "github.com/k0ngk0ng/wire-board/games/stoneage/motion"
)

type AttackMotion struct {
	Actor          int        `json:"actor"`
	From           [2]float64 `json:"from"`
	To             [2]float64 `json:"to"`
	Approach       int        `json:"approach"`
	AttackStart    int        `json:"attackStart"`
	AttackDuration int        `json:"attackDuration"`
	ContactOffset  int        `json:"contactOffset"`
	ContactHold    int        `json:"contactHold"`
	PostHold       int        `json:"postHold"`
	ReturnDuration int        `json:"returnDuration"`
}
type spriteTiming struct{ Duration, ContactOffset, Ticks, Frames int }
type actorTiming struct{ Attack, Hit, Dead spriteTiming }

var nativeTiming struct {
	FPS, ContactRadius, Speed, NormalHoldTicks, HeavyHoldTicks, PostHoldTicks int
	Points                                                                    [20][2]float64
	Place                                                                     [20]int
	Graphics                                                                  map[string]map[string]actorTiming
}
var timingOnce sync.Once

func timingCatalog() {
	timingOnce.Do(func() {
		if err := json.Unmarshal(native.Metadata, &nativeTiming); err != nil {
			panic(err)
		}
	})
}
func (s *State) logicalPosition(id int) [2]float64 {
	p := s.Players[id/2]
	row := 0
	for _, other := range s.Players {
		if other.Team == p.Team && other.Seat < p.Seat {
			row++
		}
	}
	index := row + id%2*5
	if p.Team == 1 {
		index += 10
	}
	return nativeTiming.Points[nativeTiming.Place[index]]
}
func (s *State) graphicTiming(id int) actorTiming {
	p := s.Players[id/2]
	graphic := s.unit(id).Graphic
	if id%2 == 0 && p.Riding && p.Loadout != nil {
		if ride, ok := petByID(s.Pool.Rides, p.Loadout.Ride); ok {
			for _, rider := range ride.Riders {
				if rider.Avatar == p.Loadout.Avatar {
					graphic = rider.Graphic
					break
				}
			}
		}
	}
	direction := "3"
	if p.Team == 1 {
		direction = "7"
	}
	return nativeTiming.Graphics[strconv.Itoa(graphic)][direction]
}
func (s *State) timeEvent(event *Event) {
	if event.Actor < 0 || event.Target < 0 || event.Actor >= len(s.Players)*2 || event.Target >= len(s.Players)*2 {
		return
	}
	if event.Kind != "attack" && event.Kind != "counter" && event.Kind != "miss" && event.Kind != "combo" {
		return
	}
	timingCatalog()
	actors := event.Actors
	if len(actors) == 0 {
		actors = []int{event.Actor}
	}
	tick := 1000.0 / float64(nativeTiming.FPS)
	target := s.logicalPosition(event.Target)
	heavy := event.Critical || s.unit(event.Target).HP <= 0
	hold := nativeTiming.NormalHoldTicks
	if heavy {
		hold = nativeTiming.HeavyHoldTicks
	}
	event.Motions = nil
	impact := 0
	for _, actor := range actors {
		from := s.logicalPosition(actor)
		dx, dy := target[0]-from[0], target[1]-from[1]
		distance := math.Hypot(dx, dy)
		travel := math.Max(0, distance-float64(nativeTiming.ContactRadius))
		fraction := 0.0
		if distance > 0 {
			fraction = travel / distance
		}
		attack := s.graphicTiming(actor).Attack
		m := AttackMotion{Actor: actor, From: from, To: [2]float64{from[0] + dx*fraction, from[1] + dy*fraction}, Approach: int(math.Ceil(math.Ceil(travel/float64(nativeTiming.Speed)) * tick)), AttackDuration: max(120, attack.Duration), ContactOffset: attack.ContactOffset, ContactHold: int(math.Ceil(float64(hold) * tick)), PostHold: int(math.Ceil(float64(nativeTiming.PostHoldTicks) * tick)), ReturnDuration: int(math.Ceil(math.Ceil(math.Max(0, travel-8)/float64(nativeTiming.Speed)) * tick))}
		impact = max(impact, m.Approach+m.ContactOffset)
		event.Motions = append(event.Motions, m)
	}
	event.ImpactAt = &impact
	event.Duration = 0
	for i := range event.Motions {
		m := &event.Motions[i]
		m.AttackStart = impact - m.ContactOffset
		event.Duration = max(event.Duration, m.AttackStart+m.AttackDuration+m.ContactHold+m.PostHold+m.ReturnDuration)
	}
	targetTiming := s.graphicTiming(event.Target)
	reaction := targetTiming.Hit.Duration
	if s.unit(event.Target).HP <= 0 {
		reaction = targetTiming.Dead.Duration
	}
	if event.Kind == "miss" {
		reaction = 0
	}
	event.ReactionDuration = reaction
	event.Duration = max(event.Duration, impact+reaction+int(math.Ceil(float64(hold)*tick)))
}

func (s *State) targetReason(requested, target, actual, weapon int) {
	if requested == actual {
		return
	}
	e := &s.Events[len(s.Events)-1]
	e.OriginalTarget = &requested
	switch {
	case actual != target:
		e.RedirectReason = "忠犬代挡"
	case weapon == 4:
		e.RedirectReason = "弓箭散射"
	case weapon == 17:
		e.RedirectReason = "回旋镖范围攻击"
	default:
		e.RedirectReason = "原目标已退场"
	}
}
