package engine

import (
	"fmt"
	"github.com/k0ngk0ng/wire-board/games/basketball/campaign"
)

type PostResultDispatchStage string
type PostResultResourceKind string
type PostResultResourceStage string
type PostResultPhaseContinuation uint8
type PostResultVideoEffectKind string

const (
	PostResultReady          PostResultDispatchStage = "ready"
	PostResultPhase          PostResultDispatchStage = "phase"
	PostResultActors         PostResultDispatchStage = "actors"
	PostResultOuterThreshold PostResultDispatchStage = "outer-threshold"
	PostResultOuterChange    PostResultDispatchStage = "outer-change"
	PostResultHandoff        PostResultDispatchStage = "handoff"
	PostResultReset          PostResultDispatchStage = "reset"
	PostResultBackground     PostResultResourceKind  = "background"
	PostResultFade           PostResultResourceKind  = "fade"
	PostResultMessage        PostResultResourceKind  = "message"
	PostResultWaitEntry      PostResultResourceStage = "wait-entry"
	PostResultBlankEnter     PostResultResourceStage = "blank-enter"
	PostResultWork           PostResultResourceStage = "work"
	PostResultBlankExit      PostResultResourceStage = "blank-exit"
	PostResultWaitExit       PostResultResourceStage = "wait-exit"
	PostResultPaletteCommit  PostResultResourceStage = "palette"
	PostResultPaletteDelay   PostResultResourceStage = "delay"
	PostResultCounterChange  PostResultResourceStage = "counter"
)

// PostResultVideoState owns live presentation sources, not the confirmed match
// outcome. Queues0/1 stay in Context; TailUploads owns physical0351/0374 once.
type PostResultVideoState struct {
	Scene                                       OpeningState
	Context                                     EndingSceneContext
	Progress                                    campaign.Progress
	TailUploads                                 [2]EndingUpload
	FieldCHR                                    [2]uint8
	PaletteBuildLevel, Control, Mask, LogicByte uint8
	Clock                                       PostResultVideoClock
	Resource                                    PostResultVideoResource
}
type PostResultVideoClock struct {
	Counter, ObservedCounter uint8
	Stage                    PostResultDispatchStage
	Input                    [4]ControllerFrame
	OriginScene              uint8
	OriginPhase              MatchPhase
	OriginEntered            bool
	Resume                   PostResultPhaseContinuation
	SavedPalette             uint8
	SavedFieldCHR            [2]uint8
	VideoTicks               uint64
}
type PostResultVideoResource struct {
	Kind                                           PostResultResourceKind
	Stage                                          PostResultResourceStage
	ID, Delay, Flags, Level, Counter, SavedControl uint8
	RemainingCycles                                uint32
}

// Effects describe source commits and submission boundaries. NMI and live
// snapshots remain separate: IRQ handlers can read offsets changed by mainline.
type PostResultVideoEffect struct {
	Kind                 PostResultVideoEffectKind
	Value, Slot, Control uint8
	Upload               EndingUpload
	Background           PresentationBackground
	CHR                  [4]uint8
	FieldCHR             [2]uint8
	Raster               OriginalPostResultRasterSource
	Actors               OpeningState
}
type PostResultVideoStep struct {
	State                                              PostResultVideoState
	Effects                                            []PostResultVideoEffect
	DispatchStarted, DispatchCompleted, Handoff, Reset bool
	LiveRaster                                         OriginalPostResultRasterSource
}

func postResultRaster(s PostResultVideoState) OriginalPostResultRasterSource {
	t := s.Scene.Transition
	phase := uint8(t.Match.Phase)
	if t.Match.Entered {
		phase |= 128
	}
	return OriginalPostResultRasterSource{Scene: t.Scene, Phase: phase, MachineControl: s.Context.Ending.MachineControl, CameraX: t.CameraScroll[0], CameraY: t.CameraScroll[1], StageOffset: t.Passages[2], Credits: [4]uint8(t.Passages[3:7]), BackgroundCHR: s.FieldCHR, SpriteCHR: s.Context.Ending.CHRPages, Control: s.Control, Mask: s.Mask}
}
func postResultThreshold(scene uint8) uint8 {
	if scene == 1 {
		return 2
	}
	return 0
}

// WaitAfterOriginalPostResultDispatch entersCAE9 after the preceding actor tail.
// This boundary does not advance video time or start a new scene invocation.
func WaitAfterOriginalPostResultDispatch(s PostResultVideoState) PostResultVideoState {
	s.Clock.Stage = "outer-threshold"
	if s.Clock.Counter >= postResultThreshold(s.Scene.Transition.Scene) {
		s.Clock.Stage = "outer-change"
		s.Clock.ObservedCounter = s.Clock.Counter
	}
	return s
}

// BeginOriginalPostResultDispatch has no video advance. Input is sampled only
// here; changes on blocked video ticks cannot replace the running sample.
func BeginOriginalPostResultDispatch(original PostResultVideoState, input [4]ControllerFrame) (PostResultVideoStep, error) {
	initial := PostResultVideoStep{State: original}
	if original.Resource.Kind != "" || (original.Clock.Stage != "" && original.Clock.Stage != "ready") {
		return initial, fmt.Errorf("postresult dispatch already active")
	}
	r := initial
	if e := beginPostResultDispatch(&r, input); e != nil {
		return initial, e
	}
	if e := runPostResultMainline(&r); e != nil {
		return initial, e
	}
	r.LiveRaster = postResultRaster(r.State)
	return r, nil
}
func beginPostResultDispatch(r *PostResultVideoStep, input [4]ControllerFrame) error {
	s := &r.State
	t := &s.Scene.Transition
	if !(t.Scene == 1 && t.Match.Phase == 7) && !(t.Scene == 7 && t.Match.Phase <= 10) {
		return fmt.Errorf("unsupported postresult dispatch scene%d phase%d", t.Scene, t.Match.Phase)
	}
	s.Clock.Counter = 0
	s.Clock.Input = input
	s.LogicByte++
	t.CameraScroll[0], t.CameraScroll[1] = t.CameraScroll[2], t.CameraScroll[3]
	r.Effects = append(r.Effects, PostResultVideoEffect{Kind: "actors-submit", Actors: s.Scene})
	s.Scene.Projection = resetOriginalWorldDrawList(s.Scene.Projection)
	s.Clock.OriginScene, s.Clock.OriginPhase, s.Clock.OriginEntered = t.Scene, t.Match.Phase, t.Match.Entered
	s.Clock.Resume = 0
	s.Clock.Stage = "phase"
	if t.Scene == 1 {
		t.Random, _ = NextRandom(t.Random)
		bank, e := SelectOriginalBallCHR(s.Context.Ending.CHRPages[3], t.Objects[6].Pose.Frame, uint8(t.Match.Phase))
		if e != nil {
			return e
		}
		s.Context.Ending.CHRPages[3] = bank
	} else {
		c := &s.Context
		text, e := StepOriginalSceneText(t.Scene, c.Ending.Actor, c.Ending.TextActive, c.Text, [2]EndingUpload{c.Ending.Upload, c.SecondUpload})
		if e != nil {
			return e
		}
		c.Ending.Actor, c.Ending.TextActive, c.Text = text.Context, text.Active, text.Progress
		c.Ending.Upload, c.SecondUpload = text.Uploads[0], text.Uploads[1]
		if text.ClearUploadControl {
			c.UploadControl = [4]uint8{}
		}
		for slot, u := range text.Uploads {
			if text.QueueMask&(1<<slot) != 0 {
				r.Effects = append(r.Effects, PostResultVideoEffect{Kind: "upload-queued", Slot: uint8(slot), Upload: u})
			}
		}
	}
	r.DispatchStarted = true
	return nil
}
func runPostResultMainline(r *PostResultVideoStep) error {
	for count := 0; count < 32; count++ {
		s := &r.State
		if s.Resource.Kind != "" || s.Clock.Stage == "outer-threshold" || s.Clock.Stage == "outer-change" || s.Clock.Stage == "handoff" || s.Clock.Stage == "reset" {
			return nil
		}
		switch s.Clock.Stage {
		case "phase":
			var e error
			if s.Clock.OriginScene == 1 {
				e = advancePostResultReward(r)
			} else {
				e = advancePostResultEnding(r)
			}
			if e != nil {
				return e
			}
		case "actors":
			n := uint8(4)
			if s.Clock.OriginScene == 1 {
				n = 3
			}
			for actor := uint8(0); actor < n; actor++ {
				a, e := StepOriginalResultActor(s.Scene, s.Context.Ending.Actor, actor)
				if e != nil {
					return e
				}
				s.Scene, s.Context.Ending.Actor = a.State, a.Context
				for _, id := range a.SoundIDs {
					postResultSound(r, id)
				}
			}
			r.DispatchCompleted = true
			*s = WaitAfterOriginalPostResultDispatch(*s)
		default:
			return fmt.Errorf("unknown postresult continuation %q", s.Clock.Stage)
		}
	}
	return fmt.Errorf("postresult semantic continuation did not yield")
}
func postResultSound(r *PostResultVideoStep, id uint8) {
	r.Effects = append(r.Effects, PostResultVideoEffect{Kind: "sound", Value: id})
}
func postResultQueue(s *PostResultVideoState, slot int) *EndingUpload {
	if slot == 0 {
		return &s.Context.Ending.Upload
	}
	if slot == 1 {
		return &s.Context.SecondUpload
	}
	return &s.TailUploads[slot-2]
}

// StepOriginalPostResultVideo advances exactly one video boundary. It services
// normal NMI queues/counters or the source blanking/work boundary, then resumes
// semantic mainline chunks until the next wait. It executes no CPU bytecode.
func StepOriginalPostResultVideo(original PostResultVideoState, input [4]ControllerFrame) (PostResultVideoStep, error) {
	initial := PostResultVideoStep{State: original}
	r := initial
	s := &r.State
	if s.Clock.Stage == "reset" || s.Clock.Stage == "handoff" {
		return initial, fmt.Errorf("postresult video reached terminal boundary")
	}
	s.Clock.VideoTicks++
	special := s.Resource.Kind == "background" && (s.Resource.Stage == "blank-enter" || s.Resource.Stage == "blank-exit")
	work := s.Resource.Kind == "background" && s.Resource.Stage == "work"
	if special {
		s.Control &= 127
		s.Context.Ending.MachineControl = s.Resource.SavedControl
		r.Effects = append(r.Effects, PostResultVideoEffect{Kind: "blanking-handoff"})
	} else if !work {
		for slot := 0; slot < 4; slot++ {
			u := postResultQueue(s, slot)
			if u.Address&0x8000 != 0 {
				continue
			}
			if u.Length > 32 {
				return initial, fmt.Errorf("postresult upload%d exceeds buffer", slot)
			}
			r.Effects = append(r.Effects, PostResultVideoEffect{Kind: "upload-commit", Slot: uint8(slot), Control: s.Context.UploadControl[slot], Upload: *u})
			u.Address = 0xff00 | u.Address&255
			s.Context.UploadControl[slot] = 0
		}
		if s.Context.Ending.MachineControl&0xc0 == 0xc0 {
			s.Scene.Transition.Match = AdvanceMatchVideoTick(s.Scene.Transition.Match).State
		}
		s.Clock.Counter++
		camera := s.Scene.Transition.CameraScroll
		s.Control = s.Control&0xfc | uint8(((camera[0]>>8)&1)|((camera[1]>>8)<<1))&3
		raster := postResultRaster(*s)
		r.Effects = append(r.Effects, PostResultVideoEffect{Kind: "normal-nmi", Raster: raster})
		// F268/F002 reverses working OAM after the DMA commit. It affects
		// the next submission's retained tail, not this frame's visible OAM.
		if s.Context.Ending.MachineControl&0xc0 == 0xc0 && s.Scene.Transition.DisplayFlags&128 != 0 && s.Scene.DetachedCount == 0 {
			r.Effects = append(r.Effects, PostResultVideoEffect{Kind: "oam-reverse"})
		}
		// The stage-screen first IRQ persists056 bit0 before the resumed
		// phase1 timer changes06D9. Later IRQ offsets remain live inputs.
		if raster.Scene == 7 && raster.MachineControl&0xc0 == 0x80 && raster.Phase&127 < 3 && !(s.Resource.Kind == "background" && s.Resource.Stage == "wait-entry") {
			irq, e := BuildOriginalPostResultRasterIRQ(14, s.Control, raster)
			if e != nil {
				return initial, e
			}
			s.Control = irq.Control
			r.Effects = append(r.Effects, PostResultVideoEffect{Kind: "early-irq", Raster: raster, Value: 14})
		}
	}
	if s.Resource.Kind != "" {
		if e := advancePostResultResource(&r); e != nil {
			return initial, e
		}
	}
	if s.Resource.Kind == "" {
		if s.Clock.Stage == "outer-threshold" && s.Clock.Counter >= postResultThreshold(s.Scene.Transition.Scene) {
			s.Clock.Stage = "outer-change"
			s.Clock.ObservedCounter = s.Clock.Counter
		} else if s.Clock.Stage == "outer-change" && s.Clock.Counter != s.Clock.ObservedCounter {
			if s.Scene.Transition.Scene != 1 && s.Scene.Transition.Scene != 7 {
				s.Clock.Stage = "handoff"
				r.Handoff = true
			} else {
				s.Clock.Stage = "ready"
				if e := beginPostResultDispatch(&r, input); e != nil {
					return initial, e
				}
			}
		}
		if e := runPostResultMainline(&r); e != nil {
			return initial, e
		}
	}
	r.LiveRaster = postResultRaster(*s)
	return r, nil
}
