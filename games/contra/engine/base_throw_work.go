package contra

// BaseThrowWork is the named cross-phase creation input copied by Base rollers.
// Only recovered producers set it: player/shot work, enemy visual/creation work,
// collision heights and the selected screen stream. Unknown is not zero. This
// carries one compatibility value, never a CPU register file or a RAM reader.
type BaseThrowWork struct {
	Known bool
	Value uint8
}

func (w *BaseThrowWork) record(value uint8) {
	if w != nil {
		w.Known, w.Value = true, value
	}
}

// Original Japanese 859B dispatch-table identities; the high bit of projectile
// Kind is a trajectory flag, not part of the family selection. The original
// low-byte metadata remains observable even for delayed and impact routines.
var projectileFamilyIdentity = [2][5]uint8{
	{0xde, 0xe4, 0xea, 0xf0, 0xf6},
	{0xfc, 0x02, 0x08, 0x0e, 0x14},
}

// Original 872C's vertical offset-table identities, indexed by visible flame
// phase after the original timer comparisons. A final-center frame writes none.
var indoorFlameIdentity = [3]uint8{0x60, 0x4c, 0x38}

func (c EnemyStepContext) recordThrowWork(value uint8) { c.ThrowWork.record(value) }
