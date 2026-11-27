package racing

import "math"

const gapGatesPerLap = 64

func trackTotalLength(s *State) float64 {
	if s.Physics == 3 {
		if info := classicTrackInfoFor(s.Track); info != nil && info.length > 0 {
			return info.length
		}
	}
	if s.Track.Length > 0 {
		return s.Track.Length
	}
	_, total := pathLengths(s.Track)
	return total
}

func recordGapGateCrossing(s *State, c *Car, beforeProgress, afterProgress float64, at int64) {
	total := trackTotalLength(s)
	if total <= 1e-6 || afterProgress <= beforeProgress || afterProgress <= 0 {
		return
	}
	maxGate := max(1, s.Options.Laps) * gapGatesPerLap
	startProgress := math.Max(0, beforeProgress)
	startGate := int(math.Floor(startProgress * float64(gapGatesPerLap) / total))
	endGate := int(math.Floor(afterProgress * float64(gapGatesPerLap) / total))
	if endGate > maxGate {
		endGate = maxGate
	}
	if endGate <= startGate {
		return
	}
	if len(c.GateTimes) < endGate {
		grown := make([]int64, endGate)
		copy(grown, c.GateTimes)
		c.GateTimes = grown
	}
	span := afterProgress - beforeProgress
	for gate := startGate + 1; gate <= endGate; gate++ {
		if c.GateTimes[gate-1] > 0 {
			continue
		}
		gateProgress := float64(gate) * total / float64(gapGatesPerLap)
		frac := clamp((gateProgress-beforeProgress)/math.Max(span, 1e-8), 0, 1)
		crossAt := at - stepMS + int64(math.Round(frac*float64(stepMS)))
		if crossAt < s.StartAt {
			crossAt = s.StartAt
		}
		c.GateTimes[gate-1] = crossAt
	}
}

func interpolateLeaderCrossingAt(leader Car, startAt int64, progress, total float64) (int64, bool) {
	if progress <= 0 || total <= 1e-6 || len(leader.GateTimes) == 0 {
		return 0, false
	}
	pos := progress * float64(gapGatesPerLap) / total
	g0 := int(math.Floor(pos))
	frac := pos - float64(g0)
	if g0 >= len(leader.GateTimes) {
		return 0, false
	}
	var t0 int64
	if g0 == 0 {
		t0 = startAt
	} else {
		t0 = leader.GateTimes[g0-1]
	}
	t1 := leader.GateTimes[g0]
	if t0 <= 0 || t1 <= 0 || t1 < t0 {
		return 0, false
	}
	return t0 + int64(math.Round(frac*float64(t1-t0))), true
}

func updateTimeGaps(s *State) {
	if len(s.Cars) == 0 {
		return
	}
	if s.Phase == "countdown" {
		for i := range s.Cars {
			s.Cars[i].GapMS = 0
			s.Cars[i].GapLaps = 0
		}
		return
	}
	leaderIdx := 0
	for i := 1; i < len(s.Cars); i++ {
		a, b := s.Cars[leaderIdx], s.Cars[i]
		if b.Rank < a.Rank || (b.Rank == a.Rank && ((b.FinishAt > 0 && (a.FinishAt == 0 || b.FinishAt < a.FinishAt)) || (b.FinishAt == a.FinishAt && b.Progress > a.Progress))) {
			leaderIdx = i
		}
	}
	leader := s.Cars[leaderIdx]
	total := trackTotalLength(s)
	totalMM := int64(math.Round(total * 100))
	leaderRefAt := s.ServerAt
	if leader.FinishAt > 0 {
		leaderRefAt = leader.FinishAt
	}
	elapsedMS := leaderRefAt - s.StartAt
	if elapsedMS < 1000 {
		elapsedMS = 1000
	}
	leaderAvg := clamp(math.Max(0, leader.Progress)/(float64(elapsedMS)/1000.0), 120, 1200)
	for i := range s.Cars {
		c := &s.Cars[i]
		if i == leaderIdx {
			c.GapMS = 0
			c.GapLaps = 0
			continue
		}
		if leader.FinishAt > 0 && c.FinishAt > 0 {
			diff := c.FinishAt - leader.FinishAt
			if diff < 0 {
				diff = 0
			}
			c.GapMS = diff
			c.GapLaps = 0
			continue
		}
		deltaProgress := leader.Progress - c.Progress
		deltaMM := int64(math.Round(deltaProgress * 100))
		if deltaMM <= 0 {
			c.GapMS = 0
			c.GapLaps = 0
			continue
		}
		gapLaps := 0
		if totalMM > 0 && deltaMM >= totalMM {
			gapLaps = int(deltaMM / totalMM)
		}
		c.GapLaps = gapLaps
		if passedAt, ok := interpolateLeaderCrossingAt(leader, s.StartAt, c.Progress, total); ok {
			nowRef := s.ServerAt
			if c.FinishAt > 0 {
				nowRef = c.FinishAt
			}
			gap := nowRef - passedAt
			if gap < 0 {
				gap = 0
			}
			c.GapMS = gap
			continue
		}
		gap := int64(math.Round(deltaProgress / leaderAvg * 1000.0))
		if gap < 0 {
			gap = 0
		}
		c.GapMS = gap
	}
}
