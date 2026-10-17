package engine

// MotionSurfaceWrite is the original terrain-cache lookup at a real physical
// step, using its old floor point. It is an ordered display event, not a result
// inferred from the object's final position. Attached following emits none.
type MotionSurfaceWrite struct {
	Object uint8    `json:"object"`
	Origin Position `json:"origin"`
}

// A recorder exists only inside one transactional WorldActions call. Courts
// carry local copies of this sink; neither world state nor global state owns it.
// Failed steps discard the entire event slice together with their state.
type motionSurfaceRecorder struct {
	object uint8
	writes *[]MotionSurfaceWrite
}

func (c FlatCourt) withMotionSurfaceWrites(object uint8, writes *[]MotionSurfaceWrite) FlatCourt {
	c.motionSurface = &motionSurfaceRecorder{object: object, writes: writes}
	return c
}

func (c FlatCourt) recordMotionSurface(origin Position) {
	if c.motionSurface != nil {
		r := c.motionSurface
		*r.writes = append(*r.writes, MotionSurfaceWrite{Object: r.object, Origin: origin})
	}
}
