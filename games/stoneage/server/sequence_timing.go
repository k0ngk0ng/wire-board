package stoneage

import "math"

// A projectile is separate from the stationary shooter's SPR. Its arrival,
// rather than the shooter's release frame, is the authoritative damage time.
type Projectile struct {
	Kind           string     `json:"kind"`
	Actor          int        `json:"actor"`
	Target         int        `json:"target"`
	From           [2]float64 `json:"from"`
	To             [2]float64 `json:"to"`
	LaunchAt       int        `json:"launchAt"`
	FlightDuration int        `json:"flightDuration"`
	ImpactAt       int        `json:"impactAt"`
	LingerDuration int        `json:"lingerDuration"`
	ReturnDuration int        `json:"returnDuration,omitempty"`
}

func (s *State) timeAttackSequence(first, actor, weapon int) {
	if first == len(s.Events) {
		return
	}
	timingCatalog()
	tick := 1000.0 / float64(nativeTiming.FPS)
	travelTime := func(distance float64) int {
		if distance < .01 {
			return 0
		}
		return int(math.Ceil(math.Ceil(distance/float64(nativeTiming.Speed)) * tick))
	}
	current := s.logicalPosition(actor)
	at := s.Events[first].At
	for index := first; index < len(s.Events); index++ {
		e := &s.Events[index]
		if e.Kind != "attack" && e.Kind != "miss" && e.Kind != "counter" {
			continue
		}
		snapshot := *s
		snapshot.Players = e.Players
		home := snapshot.logicalPosition(e.Actor)
		start := home
		if e.Actor == actor {
			start = current
		}
		target := snapshot.logicalPosition(e.Target)
		if e.Target == actor {
			target = current
		}
		to := start
		ranged := e.Actor == actor && actor%2 == 0 && isThrow(weapon)
		if !ranged && e.Kind != "counter" {
			dx, dy := target[0]-start[0], target[1]-start[1]
			distance := math.Hypot(dx, dy)
			if distance > float64(nativeTiming.ContactRadius)+.001 {
				fraction := (distance - float64(nativeTiming.ContactRadius)) / distance
				to = [2]float64{start[0] + dx*fraction, start[1] + dy*fraction}
			}
		}
		attack := snapshot.graphicTiming(e.Actor).Attack
		hold := nativeTiming.NormalHoldTicks
		if e.Critical || snapshot.unit(e.Target).HP <= 0 {
			hold = nativeTiming.HeavyHoldTicks
		}
		m := AttackMotion{Actor: e.Actor, From: home, Start: &start, To: to, Approach: travelTime(math.Hypot(to[0]-start[0], to[1]-start[1])), AttackDuration: max(120, attack.Duration), ContactOffset: attack.ContactOffset, ContactHold: int(math.Ceil(float64(hold) * tick)), PostHold: int(math.Ceil(float64(nativeTiming.PostHoldTicks) * tick))}
		final := index == len(s.Events)-1
		m.AttackStart = m.Approach
		m.Stay = !final || e.Actor != actor || ranged
		if !final {
			m.PostHold = 100
		}
		if !m.Stay {
			m.ReturnDuration = travelTime(math.Max(0, math.Hypot(to[0]-home[0], to[1]-home[1])-8))
		}
		impact := m.AttackStart + m.ContactOffset
		e.Projectiles = nil
		if ranged {
			kind := map[int]string{4: "arrow", 17: "boomerang", 18: "axe", 19: "stone"}[weapon]
			flight := max(100, min(800, travelTime(math.Max(0, math.Hypot(target[0]-start[0], target[1]-start[1])-116))))
			p := Projectile{Kind: kind, Actor: actor, Target: e.Target, From: start, To: target, LaunchAt: impact, FlightDuration: flight, ImpactAt: impact + flight, LingerDuration: int(math.Ceil(float64(hold) * tick))}
			if weapon == 17 {
				p.ReturnDuration = flight
			}
			e.Projectiles = []Projectile{p}
			impact = p.ImpactAt
			m.ContactHold = 0
		}
		reaction := snapshot.graphicTiming(e.Target).Hit.Duration
		if snapshot.unit(e.Target).HP <= 0 {
			reaction = snapshot.graphicTiming(e.Target).Dead.Duration
		}
		if e.Kind == "miss" {
			reaction = 0
		}
		activeEnd := m.AttackStart + m.AttackDuration + m.ContactHold + m.PostHold
		e.Motions = []AttackMotion{m}
		e.Duration = max(activeEnd+m.ReturnDuration, impact+reaction+int(math.Ceil(float64(hold)*tick)))
		for _, p := range e.Projectiles {
			e.Duration = max(e.Duration, p.ImpactAt+p.LingerDuration+p.ReturnDuration)
		}
		if e.Actor == actor {
			current = to
		} else {
			// ATT_COUNTER executes beside the approaching attacker. Keep the original
			// actor there during the exchange, including its final return home.
			origin := snapshot.logicalPosition(actor)
			park := current
			wait := max(activeEnd, impact+reaction+int(math.Ceil(float64(hold)*tick)))
			passive := AttackMotion{Actor: actor, From: origin, Start: &park, To: park, Idle: true, Stay: !final, ReturnStart: &wait}
			if final {
				passive.ReturnDuration = travelTime(math.Max(0, math.Hypot(park[0]-origin[0], park[1]-origin[1])-8))
				e.Duration = max(e.Duration, wait+passive.ReturnDuration)
			}
			e.Motions = append(e.Motions, passive)
		}
		e.Sequence = s.Events[first].ID
		e.ImpactAt = &impact
		e.ReactionDuration = reaction
		e.At = at
		at += e.Duration
	}
}

// Store the faster clock in the server timeline so reconnects, sound, HP and
// command deadlines agree. No client-only timer or combat random draw changes.
func (s *State) speedEventTimeline() {
	scale := func(n int) int { return int(math.Round(float64(n) / 1.5)) }
	at := 0
	for i := range s.Events {
		e := &s.Events[i]
		e.At = at
		e.Duration = scale(e.Duration)
		e.ReactionDuration = scale(e.ReactionDuration)
		if e.ImpactAt != nil {
			n := scale(*e.ImpactAt)
			e.ImpactAt = &n
		}
		for j := range e.Motions {
			m := &e.Motions[j]
			m.Approach = scale(m.Approach)
			contact := scale(m.AttackStart + m.ContactOffset)
			m.AttackStart = scale(m.AttackStart)
			m.AttackDuration = scale(m.AttackDuration)
			m.ContactOffset = contact - m.AttackStart
			m.ContactHold = scale(m.ContactHold)
			m.PostHold = scale(m.PostHold)
			m.ReturnDuration = scale(m.ReturnDuration)
			if m.ReturnStart != nil {
				n := scale(*m.ReturnStart)
				m.ReturnStart = &n
			}
		}
		for j := range e.Projectiles {
			p := &e.Projectiles[j]
			p.LaunchAt = scale(p.LaunchAt)
			p.FlightDuration = *e.ImpactAt - p.LaunchAt
			p.ImpactAt = *e.ImpactAt
			p.LingerDuration = scale(p.LingerDuration)
			p.ReturnDuration = scale(p.ReturnDuration)
		}
		at += e.Duration
	}
}
