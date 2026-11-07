package contra

// TerrainWork retains the recovered cross-operation compatibility value read
// by later terrain edits. Its byte roles depend on the named producer: query
// column/index, stream packing position, perspective offset or graphic location.
// It is not a CPU register file or a general memory reader. World integration
// must call these methods at the real operation boundaries, never from a trace.
type TerrainWork struct{ Value uint16 }

func (w *TerrainWork) RecordQuery(sample TerrainSample) {
	w.Value = uint16(sample.Column) | uint16(sample.SourceIndex)<<8
}
func (w *TerrainWork) RecordIndexRead(index uint8) {
	w.Value = w.Value&0xff | uint16(index)<<8
}
func (w *TerrainWork) RecordEntityProbeOverflow(x uint8) {
	w.RecordIndexRead(x)
}
func (w *TerrainWork) RecordGraphicLocation(address uint16) {
	w.Value = address
}
func (w *TerrainWork) RecordStreamColumn(address uint16) {
	if uint8(address)&1 != 0 {
		w.Value = w.Value&0xff00 | uint16(uint8(address>>8))
		return
	}
	index := uint8(((address>>8)&4)<<4) | uint8(address)>>3
	index += 56
	column := (uint8(address) >> 1) & 3
	w.Value = uint16(index) | uint16(column)<<8
}
func (w *TerrainWork) RecordStreamRow(axis uint8) {
	index := (axis >> 1) * 4
	if axis&1 == 0 {
		index += 4
	}
	w.Value = uint16(index)
}
func (w *TerrainWork) RecordPerspectiveX(x uint8) {
	w.Value = w.Value&0xff00 | uint16(x-128)
}
