package snooker

// Every frame keeps its original timestamp. Only changed ball states are
// retained after the first complete frame, preserving exact coordinates.
func compactFrames(frames []MotionFrame) []MotionFrame {
	out := make([]MotionFrame, len(frames))
	last := map[int]Ball{}
	for i, f := range frames {
		out[i] = MotionFrame{Time: f.Time, Balls: []Ball{}}
		for _, b := range f.Balls {
			old, exists := last[b.ID]
			if !exists || old != b {
				out[i].Balls = append(out[i].Balls, b)
				last[b.ID] = b
			}
		}
	}
	return out
}
