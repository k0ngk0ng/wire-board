package contra

import (
	_ "embed"
	"encoding/json"
	"errors"
	"sync"
)

// SnowParticle preserves the original scene effect's byte coordinates and
// flags. Bit 1 selects velocity; bit 0 controls whether its OAM slot is visible.
type SnowParticle struct {
	X, Y, Flags uint8
}

// SpriteCompositionState belongs to a match and survives frame/scene restores.
// OAM is presentation data, not a CPU address space. Hidden slots retain their
// other bytes, and the next display boundary submits the completed buffer.
type SpriteCompositionState struct {
	OAM      [256]uint8
	Rotation uint8
	Snow     [16]SnowParticle
}

// SpriteCompositionInput is the world projection after its gameplay phases.
// Objects 0..9 are CombatFrame.Display; 10..25 are the sixteen enemy slots.
// The original lives HUD does not read arcade scores (scores are background
// graphics). SnowControl is supplied by the world's $CE59 visibility program.
type SpriteCompositionInput struct {
	Ending                          bool
	Objects                         [26]LogicalSprite
	Flicker, HUD, TwoPlayers, Demo  bool
	Level, FrameScroll, SnowControl uint8
	Lives, GameOver                 [2]uint8
}

// SpriteCompositionResult exposes the one recovered cross-phase dependency.
// A shared-component marker writes its pointer low byte even if unchanged.
// The world applies this event to generation compatibility state immediately;
// OAM itself waits for the next presentation submission boundary.
type SpriteCompositionResult struct {
	GenerationBiasChanged bool
	GenerationBiasValue   uint8
}

type compositionPart struct {
	DX, DY         int8
	Tile, Attr     uint8
	Recoil         bool
	GenerationBias *uint8
}

type compositionFrame struct {
	Code  uint8
	Kind  string
	Parts []compositionPart
}

type compositionData struct {
	Schema                       int
	SourceRomSHA256              string
	Frames                       []compositionFrame
	InitialSnow                  [16]SnowParticle
	SnowVelocityY, SnowVelocityX [2]uint8
	HUDTiles, HUDX               [8]uint8
}

// Named descriptors and scalar tables extracted from the authenticated source;
// this data contains no executable ROM or interpreted instruction stream.
//
//go:embed presentation_sprites_data.json
var compositionJSON []byte

var spriteCompositionData = sync.OnceValues(func() (*compositionData, error) {
	var data compositionData
	if err := json.Unmarshal(compositionJSON, &data); err != nil {
		return nil, err
	}
	if data.Schema != 1 || data.SourceRomSHA256 != "6ba53139fa88b8de1ae527c438bda6f1541d1ee7df26d63dec5164a32d166bfe" || len(data.Frames) != 255 {
		return nil, errors.New("invalid source-bound sprite composition data")
	}
	for i, frame := range data.Frames {
		if int(frame.Code) != i+1 {
			return nil, errors.New("invalid sprite code order")
		}
		switch frame.Kind {
		case "blank":
			if len(frame.Parts) != 0 {
				return nil, errors.New("invalid blank sprite")
			}
		case "compact":
			if len(frame.Parts) != 1 || frame.Parts[0].DX != -4 || frame.Parts[0].DY != -8 || frame.Parts[0].GenerationBias != nil {
				return nil, errors.New("invalid compact sprite")
			}
		case "regular":
			if len(frame.Parts) < 1 || len(frame.Parts) > 63 {
				return nil, errors.New("invalid sprite part count")
			}
		default:
			return nil, errors.New("unknown sprite composition format")
		}
	}
	return &data, nil
})

// NewSpriteCompositionState starts a clean platform presentation with hidden
// OAM slots and the original A816 snow-table initialization. Scene changes use
// ResetSnow while preserving an existing match's OAM and cycling origin.
func NewSpriteCompositionState() (SpriteCompositionState, error) {
	var state SpriteCompositionState
	if err := state.ResetSnow(); err != nil {
		return state, err
	}
	for i := 0; i < 256; i += 4 {
		state.OAM[i] = 0xf4
	}
	return state, nil
}

func (s *SpriteCompositionState) ResetSnow() error {
	data, err := spriteCompositionData()
	if err != nil {
		return err
	}
	s.Snow = data.InitialSnow
	return nil
}

// Compose reproduces complete $919F/$9167 output on a private state copy.
// Only a successful whole composition commits. It does not advance gameplay,
// scores, lives, camera, the visibility program, or display submission time.
func (s *SpriteCompositionState) Compose(input SpriteCompositionInput) (SpriteCompositionResult, error) {
	if input.Level > 8 || input.Level == 8 && !input.Ending || input.Ending && input.Level != 8 || input.Flicker && s.Rotation&3 != 0 {
		return SpriteCompositionResult{}, errors.New("invalid sprite composition scene or origin")
	}
	data, err := spriteCompositionData()
	if err != nil {
		return SpriteCompositionResult{}, err
	}
	next := *s
	builder := sceneSpriteBuilder{state: &next, data: data, remaining: 63}
	if !input.Flicker {
		builder.stride = 4
		for _, object := range input.Objects {
			builder.sprite(object)
		}
		// Linear mode always runs this loop, including cursor == 0. It neither
		// changes Rotation nor runs the regular HUD/snow/capacity branches.
		for {
			next.OAM[builder.cursor] = 0xf4
			builder.cursor += 4
			if builder.cursor == 0 {
				break
			}
		}
	} else {
		builder.stride = 0xc4
		next.Rotation += 0x4c
		builder.cursor = next.Rotation
		if input.HUD {
			builder.hud(input)
		}
		for index := len(input.Objects) - 1; index >= 0; index-- {
			builder.sprite(input.Objects[index])
			if builder.remaining&0x80 != 0 {
				break
			}
		}
		if builder.remaining&0x80 == 0 {
			if input.Level == 4 {
				builder.snow(input)
			}
			if builder.remaining&0x80 == 0 {
				for count := int(builder.remaining) + 1; count > 0; count-- {
					next.OAM[builder.cursor] = 0xf4
					builder.cursor += builder.stride
				}
			}
		}
	}
	*s = next
	return builder.result, nil
}

type sceneSpriteBuilder struct {
	state                     *SpriteCompositionState
	data                      *compositionData
	cursor, remaining, stride uint8
	result                    SpriteCompositionResult
}

func (b *sceneSpriteBuilder) advance() {
	b.remaining--
	b.cursor += b.stride
}

func (b *sceneSpriteBuilder) sprite(sprite LogicalSprite) {
	if sprite.Code == 0 {
		return
	}
	frame := &b.data.Frames[int(sprite.Code)-1]
	for _, part := range frame.Parts {
		if part.GenerationBias != nil {
			b.result.GenerationBiasChanged = true
			b.result.GenerationBiasValue = *part.GenerationBias
		}
		var y, attributes uint8
		var x int
		if frame.Kind == "compact" {
			y, attributes, x = sprite.Y-8, part.Attr|sprite.Attributes, int(sprite.X)-4
		} else {
			effects := sprite.Attributes & 0xc8
			if !part.Recoil {
				effects &^= 8
			}
			dy := uint8(part.DY)
			if effects&8 != 0 {
				dy++
			}
			if effects&0x80 != 0 {
				dy = 0xf0 - dy
			}
			y = dy + sprite.Y
			mask, override := uint8(0xff), sprite.Attributes&0x20
			if sprite.Attributes&4 != 0 {
				mask, override = 0xfc, sprite.Attributes&0x23
			}
			attributes = (part.Attr&mask | override) ^ effects
			dx := uint8(part.DX)
			if effects&0x40 != 0 {
				dx = 0xf8 - dx
			}
			x = int(sprite.X) + int(int8(dx))
		}
		at := int(b.cursor)
		b.state.OAM[at], b.state.OAM[at+1], b.state.OAM[at+2] = y, part.Tile, attributes
		if x >= 0 && x < 256 {
			b.state.OAM[at+3] = uint8(x)
			b.advance()
		}
	}
}

func (b *sceneSpriteBuilder) hud(input SpriteCompositionInput) {
	last := 0
	if input.TwoPlayers || input.Demo {
		last = 1
	}
	for actor := last; actor >= 0; actor-- {
		start, count := 0, min(int(input.Lives[actor]), 4)
		if input.Demo || input.GameOver[actor]&1 != 0 {
			start, count = 4, 4
		}
		for i := 0; i < count; i++ {
			at := int(b.cursor)
			b.state.OAM[at], b.state.OAM[at+1] = 0x10, b.data.HUDTiles[start+i]
			b.state.OAM[at+2] = uint8(actor)
			// The ADC #AF includes the carry from LSR actor: P2 adds B0.
			b.state.OAM[at+3] = b.data.HUDX[start+i] + uint8(actor*0xb0)
			b.advance()
		}
	}
}

func (b *sceneSpriteBuilder) snow(input SpriteCompositionInput) {
	for index := len(b.state.Snow) - 1; index >= 0; index-- {
		particle := &b.state.Snow[index]
		at := int(b.cursor)
		b.state.OAM[at+1], b.state.OAM[at+2] = 0x61, 2
		group := (particle.Flags >> 1) & 1
		down := b.data.SnowVelocityY[group]
		left := b.data.SnowVelocityX[group] + input.FrameScroll
		oldX := particle.X
		particle.X -= left
		b.state.OAM[at+3] = particle.X
		if oldX < left {
			// $926A writes the visibility bit through the same local value used
			// for this tick's Y advance; preserve that original dependency.
			down = input.SnowControl & 1
			particle.Flags = particle.Flags&0xfe | down
		}
		particle.Y += down
		if particle.Y >= 0xee {
			particle.Y = 0
			particle.Flags = particle.Flags&0xfe | input.SnowControl&1
		}
		b.state.OAM[at] = particle.Y
		if particle.Flags&1 == 0 {
			b.state.OAM[at] = 0xf4
		}
		b.advance()
		if b.remaining&0x80 != 0 {
			break
		}
	}
}
