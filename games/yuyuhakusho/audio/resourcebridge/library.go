// Package resourcebridge connects immutable score/PCM resources and accepted
// command semantics. It owns no playback, queue, priority, clock or history.
package resourcebridge

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/pcmassets"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/pcmcommands"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/score"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/engine"
)

// Library borrows immutable decoded metadata and owns encoded payload bytes
// through pcm only. The raw JSON inputs and derived Device config are not kept.
type Library struct {
	identity string
	pcm      *pcmassets.Catalog
	score    *score.Catalog
	parser   *score.ParserMetadata
}

// Decode requires already authenticated, decompressed resource bytes. Content
// hashes bind checkpoints and dependencies; they do not authenticate a download
// against an independently trusted deployment manifest on the caller's behalf.
func Decode(scoreJSON, pcmJSON, parserJSON []byte) (*Library, error) {
	return decodeResources(scoreJSON, pcmJSON, parserJSON, nil, nil)
}

// DecodeWithInstrumentWork binds the selected instrument's sound/work identity
// without adding a second dynamic tone owner. All four exact resources enter
// the outer identity so PCM/audio checkpoint restore cannot mix resource sets.
func DecodeWithInstrumentWork(scoreJSON, pcmJSON, parserJSON, instrumentJSON []byte) (*Library, error) {
	if len(instrumentJSON) == 0 {
		return nil, errors.New("missing instrument work resource")
	}
	return decodeResources(scoreJSON, pcmJSON, parserJSON, instrumentJSON, nil)
}

// DecodeWithEnvelopeWork binds all five immutable resources, preserving the
// existing three/four-resource constructors and their exact identities.
func DecodeWithEnvelopeWork(scoreJSON, pcmJSON, parserJSON, instrumentJSON, envelopeJSON []byte) (*Library, error) {
	if len(instrumentJSON) == 0 || len(envelopeJSON) == 0 {
		return nil, errors.New("missing instrument/envelope work resource")
	}
	return decodeResources(scoreJSON, pcmJSON, parserJSON, instrumentJSON, envelopeJSON)
}

func decodeResources(scoreJSON, pcmJSON, parserJSON, instrumentJSON, envelopeJSON []byte) (*Library, error) {
	catalog, err := score.DecodeCatalog(bytes.NewReader(scoreJSON))
	if err != nil {
		return nil, fmt.Errorf("score resource: %w", err)
	}
	if instrumentJSON != nil {
		catalog, err = score.BindInstrumentWork(catalog, bytes.NewReader(instrumentJSON))
		if err != nil {
			return nil, fmt.Errorf("instrument work resource: %w", err)
		}
	}
	if envelopeJSON != nil {
		catalog, err = score.BindEnvelopeWork(catalog, bytes.NewReader(envelopeJSON))
		if err != nil {
			return nil, fmt.Errorf("envelope work resource: %w", err)
		}
	}
	pcm, err := pcmassets.Decode(bytes.NewReader(pcmJSON), scoreJSON)
	if err != nil {
		return nil, fmt.Errorf("PCM resource: %w", err)
	}
	parser, err := score.DecodeParserMetadata(catalog, bytes.NewReader(parserJSON))
	if err != nil {
		return nil, fmt.Errorf("parser resource: %w", err)
	}
	music := 0
	for _, descriptor := range pcm.Descriptors() {
		if descriptor.Protocol != pcmassets.TrackPanPCM {
			continue
		}
		other, ok := catalog.Descriptor(descriptor.ID)
		if !ok || descriptor.SampleID != other.SampleID || descriptor.NativeRate != other.NativeRate || descriptor.Flags != other.Flags {
			return nil, errors.New("score and PCM music descriptor identities differ")
		}
		music++
	}
	if music != 6 {
		return nil, errors.New("incomplete score/PCM music resource closure")
	}
	hash := sha256.New()
	hash.Write([]byte("yyh-audio-resource-bridge-v1\x00"))
	inputs := [][]byte{scoreJSON, pcmJSON, parserJSON}
	if instrumentJSON != nil {
		inputs = append(inputs, instrumentJSON)
	}
	if envelopeJSON != nil {
		inputs = append(inputs, envelopeJSON)
	}
	for _, raw := range inputs {
		digest := sha256.Sum256(raw)
		hash.Write(digest[:])
	}
	return &Library{identity: hex.EncodeToString(hash.Sum(nil)), pcm: pcm, score: catalog, parser: parser}, nil
}

func (l *Library) Identity() string {
	if l == nil {
		return ""
	}
	return l.identity
}
func (l *Library) Catalog() *score.Catalog {
	if l == nil {
		return nil
	}
	return l.score
}
func (l *Library) Parser() *score.ParserMetadata {
	if l == nil {
		return nil
	}
	return l.parser
}

// PCMConfig derives detached descriptor values and borrows the catalog's only
// payload owner through read-only callbacks. Device.New authenticates each
// payload when this config is first used; its typed Fork shares that library.
func (l *Library) PCMConfig(origin uint64, restorePan uint8) (pcmcommands.Config, error) {
	if l == nil || l.pcm == nil {
		return pcmcommands.Config{}, errors.New("missing audio resource library")
	}
	descriptors := l.pcm.Descriptors()
	config := pcmcommands.Config{Origin: origin, RestorePan: restorePan, IncludeYMAddresses: true,
		Resources: pcmcommands.Resources{Identity: l.identity,
			Length: func(id string) (uint32, bool) { n, ok := l.pcm.PayloadLength(id); return uint32(n), ok },
			ByteAt: func(id string, at uint32) (byte, bool) { return l.pcm.PayloadByte(id, int(at)) }},
		Descriptors: make([]pcmcommands.Descriptor, 0, len(descriptors)),
	}
	for _, descriptor := range descriptors {
		protocol := pcmcommands.OrdinaryProtocol
		switch descriptor.Protocol {
		case pcmassets.OrdinaryPCM:
		case pcmassets.TrackPanPCM:
			protocol = pcmcommands.MusicProtocol
		default:
			return pcmcommands.Config{}, errors.New("unsupported PCM resource protocol")
		}
		config.Descriptors = append(config.Descriptors, pcmcommands.Descriptor{ID: descriptor.ID, PayloadID: descriptor.SampleID,
			NativeRate: descriptor.NativeRate, Flags: descriptor.Flags, RoutingOrPan: descriptor.RoutingOrPan, Protocol: protocol})
	}
	return config, nil
}

type Selection struct {
	Accepted   bool
	ChoiceBits uint8
	Input      pcmcommands.Input
}

// PrepareOrdinary samples the provided live view exactly once at the real
// arbitration stage. The caller commits ChoiceBits and later publishes Input
// while holding the real bus; it must preserve this sampled StartDual decision.
// No bus acquisition, publication time or Root priority mutation is invented.
func (l *Library) PrepareOrdinary(code uint8, status pcmcommands.Status, choiceBits uint8) (Selection, error) {
	if l == nil || l.pcm == nil {
		return Selection{}, errors.New("missing audio resource library")
	}
	descriptor, ok := l.pcm.Ordinary(code)
	if !ok {
		return Selection{}, errors.New("unknown or nonordinary fight PCM command")
	}
	if choiceBits > 1 || status.ModeFlags&0x20 != 0 || descriptor.Flags&0x20 != 0 {
		return Selection{}, errors.New("unsupported PCM choice/duck profile")
	}
	decision := engine.SelectPCMCommand(engine.PCMArbitrationCommand{Flags: descriptor.Flags, RoutingOrPan: descriptor.RoutingOrPan},
		engine.PCMArbitrationView{ModeFlags: status.ModeFlags, SlotFlags: status.SlotFlags, SlotBusy: status.SlotBusy, ChoiceBits: choiceBits})
	result := Selection{ChoiceBits: decision.ChoiceBits}
	if decision.Destination == engine.PCMReject {
		return result, nil
	}
	result.Accepted = true
	result.Input = pcmcommands.Input{Kind: pcmcommands.Ordinary, DescriptorID: descriptor.ID, StartDual: decision.StartDual}
	switch decision.Destination {
	case engine.PCMSingle:
		result.Input.Destination = pcmcommands.Single
	case engine.PCMSlotA:
		result.Input.Destination = pcmcommands.SlotA
	case engine.PCMSlotB:
		result.Input.Destination = pcmcommands.SlotB
	default:
		return Selection{}, errors.New("unknown PCM arbitration destination")
	}
	return result, nil
}

// PrepareMusic translates only an already accepted score DAC or DAC-pan
// intent. It does not repeat score's earlier blocked/priority arbitration.
// Ordinary seventh-byte routing never supplies a music track's stereo pan.
func (l *Library) PrepareMusic(intent score.Intent) (pcmcommands.Input, error) {
	if l == nil || l.pcm == nil {
		return pcmcommands.Input{}, errors.New("missing audio resource library")
	}
	if intent.Order < 0 || intent.Pan == nil || intent.Bank != 0 || intent.Register != 0 || intent.Value != 0 || intent.Action != "" || intent.SoundID != 0 {
		return pcmcommands.Input{}, errors.New("invalid accepted music PCM intent")
	}
	pan := uint8(0)
	if intent.Pan.Left {
		pan |= 128
	}
	if intent.Pan.Right {
		pan |= 64
	}
	switch intent.Kind {
	case "dac":
		if _, ok := l.pcm.Music(intent.DescriptorID); !ok {
			return pcmcommands.Input{}, errors.New("unknown music PCM descriptor")
		}
		return pcmcommands.Input{Kind: pcmcommands.Music, DescriptorID: intent.DescriptorID, Pan: pan}, nil
	case "dac-pan":
		if intent.DescriptorID != "" {
			return pcmcommands.Input{}, errors.New("DAC pan unexpectedly carries a sample")
		}
		return pcmcommands.Input{Kind: pcmcommands.SetRestorePan, Pan: pan}, nil
	default:
		return pcmcommands.Input{}, errors.New("not a music PCM intent")
	}
}

// Feedback is a temporary stage-local projection of the unique PCM owner.
// Blocked uses bit7 while PanRedirect tests the entire request marker.
func Feedback(status pcmcommands.Status) score.DACFeedback {
	return score.DACFeedback{Blocked: status.Blocked(), Priority: status.Priority(), PanRedirect: status.PanRedirect()}
}
