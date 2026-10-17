package engine

const (
	actionSearchRoots uint8 = iota
	actionSearchDependents
	actionTail
	actionScheduleDone
)

// ActionScheduler restores fixed $E130/$E15A for one world action phase.
// Its zero value is ready. Call Next with the live links after each action:
// bit 6 means processed, a nonzero unprocessed link's low nibble names its
// parent, and an exact zero denotes a root ready for processing.
// It schedules actions only; accepting a request and executing its physics
// belong to the object handler called between Next invocations.
type ActionScheduler struct {
	phase uint8
	last  uint8
	tail  uint8
}

// Next returns the next object, the flags after the scheduler marks it, and
// whether there is an action to execute. Objects 0..5 can be revisited or run
// through dependency chains; the final ball/hoop tail always runs 6..12.
func (s *ActionScheduler) Next(links [7]uint8) (uint8, [7]uint8, bool) {
	for {
		switch s.phase {
		case actionSearchRoots:
			links[6] |= 0x40
			for actor := uint8(0); actor < 6; actor++ {
				if links[actor] != 0 {
					continue
				}
				links[actor] |= 0x40
				s.last, s.phase = actor, actionSearchDependents
				return actor, links, true
			}
			s.phase, s.tail = actionTail, 6
		case actionSearchDependents:
			for actor := uint8(0); actor < 7; actor++ {
				link := links[actor]
				if link == 0 || link&0x40 != 0 || link&15 != s.last {
					continue
				}
				links[actor] |= 0x40
				s.last = actor
				return actor, links, true
			}
			s.phase = actionSearchRoots
		case actionTail:
			actor := s.tail
			s.tail++
			if s.tail == 13 {
				s.phase = actionScheduleDone
			}
			return actor, links, true
		default:
			return 0, links, false
		}
	}
}
