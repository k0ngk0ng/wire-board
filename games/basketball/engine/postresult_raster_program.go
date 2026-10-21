package engine

import "fmt"

// OriginalPostResultRasterSource is sampled at the source NMI boundary, after
// resource/queue commits and before the resumed mainline changes scene state.
// Camera is displayed $0397..039A, not the pending next-camera coordinates.
type OriginalPostResultRasterSource struct {
	Scene          uint8    `json:"scene"`
	Phase          uint8    `json:"phase"`
	MachineControl uint8    `json:"machineControl"`
	CameraX        uint16   `json:"cameraX"`
	CameraY        uint16   `json:"cameraY"`
	StageOffset    uint8    `json:"stageOffset"`
	Credits        [4]uint8 `json:"credits"`
	BackgroundCHR  [2]uint8 `json:"backgroundCHR"`
	SpriteCHR      [4]uint8 `json:"spriteCHR"`
	Control        uint8    `json:"control"`
	Mask           uint8    `json:"mask"`
}

type OriginalRasterEffectKind string

const (
	OriginalRasterAddress   OriginalRasterEffectKind = "address"
	OriginalRasterScrollX   OriginalRasterEffectKind = "scrollX"
	OriginalRasterScrollY   OriginalRasterEffectKind = "scrollY"
	OriginalRasterControl   OriginalRasterEffectKind = "control"
	OriginalRasterMirroring OriginalRasterEffectKind = "mirroring"
	OriginalRasterCHR0      OriginalRasterEffectKind = "chr0"
	OriginalRasterCHR1      OriginalRasterEffectKind = "chr1"
	OriginalRasterCHR2      OriginalRasterEffectKind = "chr2"
	OriginalRasterCHR3      OriginalRasterEffectKind = "chr3"
	OriginalRasterCHR4      OriginalRasterEffectKind = "chr4"
	OriginalRasterCHR5      OriginalRasterEffectKind = "chr5"
)

// Cycle is relative to instruction-start at fixed F18F (ending NMI display),
// F206 (reward NMI display), or F2D4 (IRQ). DMA/queue time precedes the NMI
// display origin. It is not an absolute frame clock or a captured timestamp.
type OriginalRasterEffect struct {
	Cycle uint16                   `json:"cycle"`
	Kind  OriginalRasterEffectKind `json:"kind"`
	Value uint16                   `json:"value"`
}
type OriginalRasterHandler struct {
	InputStage   uint8                  `json:"inputStage,omitempty"`
	Stage        uint8                  `json:"stage"`
	Control      uint8                  `json:"control"`
	Latch        *uint8                 `json:"latch"`
	ReloadCycle  *uint16                `json:"reloadCycle"`
	DisableCycle *uint16                `json:"disableCycle"`
	Effects      []OriginalRasterEffect `json:"effects"`
}
type OriginalPostResultRasterProgram struct {
	NMI    OriginalRasterHandler   `json:"nmi"`
	Stages []OriginalRasterHandler `json:"stages"`
}

func rasterByte(v uint8) *uint8    { return &v }
func rasterCycle(v uint16) *uint16 { return &v }

// BuildOriginalPostResultRasterProgram restores only original display values,
// source ordering and handler-relative cycles. The caller supplies IRQ arrival
// timing separately. It must not infer exact arrival from phase/actors alone.
// This convenience composition assumes source fields stay unchanged through
// the chain. Live playback calls NMI and IRQ helpers separately as mainline
// logic can change credits/06D9 between IRQs in the same video frame.
// Uploads, resource waits, OAM submission and audio execution are not owned here.
func BuildOriginalPostResultRasterProgram(s OriginalPostResultRasterSource) (OriginalPostResultRasterProgram, error) {
	var result OriginalPostResultRasterProgram
	phase := s.Phase & 127
	if s.Scene == 1 {
		if phase != 7 || s.MachineControl&192 != 192 {
			return result, fmt.Errorf("reward raster requires Scene1 phase7 normal NMI")
		}
	} else if s.Scene == 7 {
		if phase > 11 || s.MachineControl&192 != 128 {
			return result, fmt.Errorf("ending raster requires Scene7 phase0..11 normal NMI")
		}
	} else {
		return result, fmt.Errorf("unrestored post-result raster scene%d", s.Scene)
	}
	for _, page := range append(s.BackgroundCHR[:], s.SpriteCHR[:]...) {
		if page >= 128 {
			return result, fmt.Errorf("post-result CHR page%d is outside source art", page)
		}
	}
	control := s.Control&252 | uint8((s.CameraX>>8)&1) | uint8((s.CameraY>>7)&2)
	nmi := OriginalRasterHandler{Control: control, Effects: make([]OriginalRasterEffect, 0, 10)}
	add := func(cycle uint16, kind OriginalRasterEffectKind, value uint16) {
		nmi.Effects = append(nmi.Effects, OriginalRasterEffect{cycle, kind, value})
	}
	if s.Scene == 1 {
		nmi.Stage = 0
		nmi.Latch = rasterByte(191)
		nmi.ReloadCycle = rasterCycle(191)
		add(20, OriginalRasterAddress, 0)
		add(32, OriginalRasterScrollX, uint16(uint8(s.CameraX)))
		add(40, OriginalRasterScrollY, uint16(uint8(s.CameraY)))
		add(78, OriginalRasterControl, uint16(control))
		add(97, OriginalRasterCHR0, uint16(s.BackgroundCHR[0]))
		add(110, OriginalRasterCHR1, uint16(s.BackgroundCHR[1]))
		for i, kind := range []OriginalRasterEffectKind{OriginalRasterCHR2, OriginalRasterCHR3, OriginalRasterCHR4, OriginalRasterCHR5} {
			add(uint16(123+i*13), kind, uint16(s.SpriteCHR[i]))
		}
	} else {
		// F18F has distinct branch lengths for phase11, phases0..4, and5..10.
		base := uint16(0)
		if phase == 11 {
			add(17, OriginalRasterMirroring, 0)
		} else if phase < 5 {
			base = 4
			add(21, OriginalRasterMirroring, 0)
		} else {
			base = 6
			add(20, OriginalRasterMirroring, 1)
		}
		pairs := [12][2]uint8{{14, 8}, {14, 8}, {14, 8}, {2, 190}, {2, 190}, {3, 10}, {3, 10}, {3, 10}, {15, 8}, {1, 192}, {2, 190}, {2, 190}}
		nmi.Stage = pairs[phase][0]
		nmi.Latch = rasterByte(pairs[phase][1])
		nmi.ReloadCycle = rasterCycle(154 + base)
		add(30+base, OriginalRasterCHR0, uint16(s.BackgroundCHR[0]))
		add(43+base, OriginalRasterCHR1, uint16(s.BackgroundCHR[1]))
		add(63+base, OriginalRasterAddress, 0)
		add(75+base, OriginalRasterScrollX, uint16(uint8(s.CameraX)))
		add(83+base, OriginalRasterScrollY, uint16(uint8(s.CameraY)))
		add(121+base, OriginalRasterControl, uint16(control))
	}
	result.NMI = nmi
	stage := nmi.Stage
	for count := 0; count < 8; count++ {
		next, err := originalPostResultIRQ(stage, control, s)
		if err != nil {
			return OriginalPostResultRasterProgram{}, err
		}
		result.Stages = append(result.Stages, next)
		control = next.Control
		if stage == 1 {
			return result, nil
		}
		stage = next.Stage
	}
	return OriginalPostResultRasterProgram{}, fmt.Errorf("post-result IRQ chain did not terminate")
}

func originalPostResultIRQ(stage, control uint8, s OriginalPostResultRasterSource) (OriginalRasterHandler, error) {
	h := OriginalRasterHandler{InputStage: stage, Control: control, Effects: make([]OriginalRasterEffect, 0, 5)}
	add := func(cycle uint16, kind OriginalRasterEffectKind, value uint16) {
		h.Effects = append(h.Effects, OriginalRasterEffect{cycle, kind, value})
	}
	scroll := func(address uint16, offset uint8, at, x, y uint16) {
		add(at, OriginalRasterAddress, address)
		add(x, OriginalRasterScrollX, uint16(offset))
		add(y, OriginalRasterScrollY, 0)
	}
	arm := func(next, latch uint8, cycle uint16) {
		h.Stage = next
		h.Latch = rasterByte(latch)
		h.ReloadCycle = rasterCycle(cycle)
	}
	switch stage {
	case 0: // F317: reward font rows, distinct from ending text address2300.
		arm(1, 8, 268)
		add(220, OriginalRasterCHR0, 96)
		phase := s.Phase & 127
		if phase > 8 {
			return h, fmt.Errorf("unrestored source font table index%d", phase)
		}
		font := uint16(98)
		if phase == 6 {
			font = 14
		}
		add(234, OriginalRasterCHR1, font)
		scroll(0x2000, 0, 250, 260, 264)
	case 1: // F365: disable, then the caller's original audio-driver boundary.
		h.Stage = 1
		h.DisableCycle = rasterCycle(115)
	case 2: // F36E: ending text.
		arm(1, 8, 258)
		add(212, OriginalRasterCHR0, 96)
		add(224, OriginalRasterCHR1, 98)
		scroll(0x2300, 0, 240, 250, 254)
	case 3: // F3AE: airplane upper strip.
		arm(4, 134, 158)
		scroll(0x28e0, 2, 138, 148, 154)
	case 4: // F3D7: airplane lower moving strip, full06D9 byte.
		arm(2, 43, 160)
		scroll(0x2b00, s.StageOffset, 138, 150, 156)
	case 14: // F6AA: two stage nametables alternate with06D9 bit0.
		arm(2, 182, 260)
		scroll(0x2020, 0, 218, 228, 232)
		h.Control = control&254 | (s.StageOffset & 1)
		add(256, OriginalRasterControl, uint16(h.Control))
	case 15:
		arm(16, 56, 240)
		scroll(0x2020, s.Credits[0], 218, 230, 236)
	case 16:
		arm(17, 32, 162)
		scroll(0x2100, s.Credits[1], 140, 152, 158)
	case 17:
		arm(18, 40, 162)
		scroll(0x2180, s.Credits[2], 140, 152, 158)
	case 18:
		arm(2, 40, 240)
		scroll(0x2220, s.Credits[3], 218, 230, 236)
	default:
		return h, fmt.Errorf("unrestored post-result IRQ stage%d", stage)
	}
	return h, nil
}

// BuildOriginalPostResultRasterNMI samples only the NMI display setup. Future
// IRQ values must be rebuilt from live logical state at each IRQ boundary.
func BuildOriginalPostResultRasterNMI(source OriginalPostResultRasterSource) (OriginalRasterHandler, error) {
	program, err := BuildOriginalPostResultRasterProgram(source)
	return program.NMI, err
}

// BuildOriginalPostResultRasterIRQ uses live source fields. Mainline animation
// can update credits/06D9 between two IRQs in one video frame; a frozen NMI
// snapshot is not a substitute. This does not advance the mainline or clock.
func BuildOriginalPostResultRasterIRQ(stage, control uint8, source OriginalPostResultRasterSource) (OriginalRasterHandler, error) {
	return originalPostResultIRQ(stage, control, source)
}
