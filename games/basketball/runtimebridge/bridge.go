// Package runtimebridge exposes a single-match protocol to browser Wasm and
// native acceptance tools. Game rules and clock advancement remain in engine.
// A confirmed result keeps advancing its owned reward/ending presentation.
// The session parks only at the original terminal handoff.
package runtimebridge

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"github.com/k0ngk0ng/wire-board/games/basketball/engine"
	"github.com/k0ngk0ng/wire-board/games/basketball/syncwire"
)

const MaxRequestBytes = 2 << 20
const maxSessions = 16

type Checkpoint = syncwire.Checkpoint

type Request struct {
	Latches       []uint32                       `json:"latches,omitempty"`
	Compact       bool                           `json:"compact,omitempty"`
	CaptureFrames bool                           `json:"captureFrames,omitempty"`
	Verify        bool                           `json:"verify,omitempty"`
	Target        uint64                         `json:"target,omitempty"`
	Window        *syncwire.InputWindow          `json:"window,omitempty"`
	Automated     []bool                         `json:"automated,omitempty"`
	Projection    *PresentationProjectionRequest `json:"projection,omitempty"`
	Op            string                         `json:"op"`
	Session       uint64                         `json:"session,omitempty"`
	Config        *engine.RuntimeConfig          `json:"config,omitempty"`
	Held          []engine.Buttons               `json:"held,omitempty"`
	Ticks         *int                           `json:"ticks,omitempty"`
	Checkpoint    *Checkpoint                    `json:"checkpoint,omitempty"`
	Controller    *uint8                         `json:"controller,omitempty"`
	Buttons       *engine.Buttons                `json:"buttons,omitempty"`
	Inputs        []engine.RecordedVideoInput    `json:"inputs,omitempty"`
}
type Failure struct {
	Code   string `json:"code"`
	Detail string `json:"detail"`
}
type Response struct {
	OK    bool     `json:"ok"`
	Data  any      `json:"data,omitempty"`
	Error *Failure `json:"error,omitempty"`
}
type View struct {
	renderFrames []CompactVideoFrame
	Session      uint64              `json:"session"`
	State        engine.RuntimeState `json:"state"`
	// JSON encodes []uint8 as base64. The browser needs numeric event IDs.
	SoundIDs     []int        `json:"soundIDs,omitempty"`
	LogicUpdates int          `json:"logicUpdates"`
	Frames       []VideoFrame `json:"frames,omitempty"`
}
type VideoFrame struct {
	Frame   uint64              `json:"frame"`
	Display engine.DisplayState `json:"display"`
}
type Bridge struct {
	next     uint64
	sessions map[uint64]engine.RuntimeState
}

func New() *Bridge { return &Bridge{next: 1, sessions: make(map[uint64]engine.RuntimeState)} }
func failed(code string, err error) Response {
	return Response{Error: &Failure{Code: code, Detail: err.Error()}}
}
func success(data any) Response { return Response{OK: true, Data: data} }

func (b *Bridge) create(state engine.RuntimeState) Response {
	if len(b.sessions) >= maxSessions {
		return failed("session_limit", fmt.Errorf("close an existing basketball session before creating another"))
	}
	if b.next > 1<<53-1 {
		return failed("session_limit", fmt.Errorf("session counter exhausted"))
	}
	id := b.next
	b.next++
	b.sessions[id] = state
	return success(View{Session: id, State: state})
}

// Handle has one caller at a time (the Wasm event loop or native input loop).
// A failed multi-tick operation commits none of its partial state or sounds.
func (b *Bridge) Handle(raw []byte) (result Response) {
	defer func() {
		if recovered := recover(); recovered != nil {
			result = failed("internal_error", fmt.Errorf("basketball engine panic: %v", recovered))
		}
	}()
	if len(raw) > MaxRequestBytes {
		return failed("bad_request", fmt.Errorf("basketball request exceeds size limit"))
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var request Request
	if err := decoder.Decode(&request); err != nil {
		return failed("bad_request", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return failed("bad_request", fmt.Errorf("request must contain one JSON object"))
	}
	switch request.Op {
	case "projectPresentation":
		if request.Projection == nil {
			return failed("bad_projection", fmt.Errorf("presentation projection is required"))
		}
		data, err := projectPresentation(*request.Projection)
		if err != nil {
			return failed("bad_projection", err)
		}
		return success(data)
	case "defaults":
		return success(engine.DefaultRuntimeConfig())
	case "create":
		config := engine.DefaultRuntimeConfig()
		if request.Config != nil {
			config = *request.Config
		}
		state, err := engine.NewRuntime(config)
		if err != nil {
			return failed("bad_config", err)
		}
		return b.createRequest(state, request)
	case "restore":
		if request.Checkpoint == nil || request.Checkpoint.Version != 1 {
			return failed("invalid_snapshot", fmt.Errorf("unsupported basketball snapshot version"))
		}
		state := request.Checkpoint.State
		if err := engine.ValidateRuntimeState(state); err != nil {
			return failed("invalid_snapshot", err)
		}
		return b.createRequest(state, request)
	case "advance", "snapshot", "dispose", "cancelInput", "replay", "fork", "automate":
	default:
		return failed("bad_operation", fmt.Errorf("unknown basketball operation %q", request.Op))
	}
	state, ok := b.sessions[request.Session]
	if !ok {
		return failed("bad_session", fmt.Errorf("basketball session is closed or does not exist"))
	}
	switch request.Op {
	case "fork":
		if request.Target == 0 {
			return b.createRequest(state, request)
		}
		if request.Target == request.Session {
			return failed("bad_target", fmt.Errorf("prediction must not replace its source"))
		}
		if _, ok := b.sessions[request.Target]; !ok {
			return failed("bad_target", fmt.Errorf("target session does not exist"))
		}
		// Engine operations are immutable; sharing the native value here avoids a
		// complete JSON export/import and preserves independent subsequent steps.
		b.sessions[request.Target] = state
		return b.reply(request, View{Session: request.Target, State: state})
	case "automate":
		if len(request.Automated) != 4 {
			return failed("bad_request", fmt.Errorf("supply four automation flags"))
		}
		next := state
		if state.Outcome == nil {
			var flags [4]bool
			copy(flags[:], request.Automated)
			var err error
			next, err = engine.SetRuntimeAutomation(state, flags)
			if err != nil {
				return failed("bad_request", err)
			}
		}
		b.sessions[request.Session] = next
		return b.reply(request, View{Session: request.Session, State: next})
	case "replay":
		if request.Window != nil {
			if len(request.Inputs) != 0 {
				return failed("bad_request", fmt.Errorf("supply one input representation"))
			}
			var err error
			request.Inputs, err = syncwire.Unpack(*request.Window)
			if err != nil {
				return failed("invalid_replay", err)
			}
			fresh := request.Inputs[:0]
			for _, input := range request.Inputs {
				if input.Frame > state.VideoFrames {
					fresh = append(fresh, input)
				}
			}
			request.Inputs = fresh
		}
		if len(request.Latches) != 0 && len(request.Latches) != 2 {
			return failed("bad_request", fmt.Errorf("supply both input latches"))
		}
		if len(request.Automated) != 0 && len(request.Automated) != 4 {
			return failed("bad_request", fmt.Errorf("supply four automation flags"))
		}
		if (len(request.Inputs) < 1 && request.Window == nil) || len(request.Inputs) > 120 {
			return failed("bad_request", fmt.Errorf("replay requires 1 through 120 consecutive video inputs"))
		}
		view := View{Session: request.Session, State: state}
		for _, input := range request.Inputs {
			step, err := engine.StepRecordedVideo(view.State, input)
			if err != nil {
				return failed("invalid_replay", err)
			}
			view.State = step.State
			if !request.Compact {
				view.Frames = append(view.Frames, VideoFrame{Frame: step.State.VideoFrames, Display: step.State.Display})
			} else if request.CaptureFrames {
				view.renderFrames = append(view.renderFrames, CompactVideoFrame{Frame: step.State.VideoFrames, Display: visibleDisplay(step.State.Display)})
			}
			if step.LogicAdvanced {
				view.LogicUpdates++
			}
		}
		if len(request.Automated) == 4 && view.State.Outcome == nil {
			var flags [4]bool
			copy(flags[:], request.Automated)
			next, err := engine.SetRuntimeAutomation(view.State, flags)
			if err != nil {
				return failed("invalid_replay", err)
			}
			view.State = next
		}
		if len(request.Latches) == 2 {
			view.State = syncwire.ApplyLatches(view.State, [2]uint32{request.Latches[0], request.Latches[1]})
		}
		// Confirmed playback consumes the server's sound-event stream; emitting
		// replay sounds here would replay old audio during reconciliation.
		b.sessions[request.Session] = view.State
		return b.reply(request, view)
	case "cancelInput":
		if request.Controller == nil {
			return failed("bad_request", fmt.Errorf("input cancellation requires a controller"))
		}
		mask := engine.Buttons(255)
		if request.Buttons != nil {
			mask = *request.Buttons
		}
		next, err := engine.ClearRuntimeButtons(state, *request.Controller, mask)
		if err != nil {
			return failed("bad_request", err)
		}
		b.sessions[request.Session] = next
		return b.reply(request, View{Session: request.Session, State: next})
	case "dispose":
		delete(b.sessions, request.Session)
		return success(struct {
			Session uint64 `json:"session"`
		}{request.Session})
	case "snapshot":
		return success(struct {
			Session    uint64     `json:"session"`
			Checkpoint Checkpoint `json:"checkpoint"`
		}{request.Session, Checkpoint{Version: 1, State: state}})
	}
	ticks := 1
	if request.Ticks != nil {
		ticks = *request.Ticks
	}
	if ticks < 1 || ticks > 600 {
		return failed("bad_request", fmt.Errorf("advance requires 1 through 600 video ticks"))
	}
	if len(request.Held) != 0 && len(request.Held) != 4 {
		return failed("bad_request", fmt.Errorf("supply exactly four controller button values"))
	}
	var held [4]engine.Buttons
	copy(held[:], request.Held)
	view := View{Session: request.Session, State: state}
	for i := 0; i < ticks; i++ {
		if engine.RuntimeFinished(view.State) {
			break
		}
		step, err := engine.StepVideo(view.State, held)
		if err != nil {
			return failed("rule_unavailable", fmt.Errorf("video tick %d: %w", i, err))
		}
		view.State = step.State
		for _, id := range step.SoundIDs {
			view.SoundIDs = append(view.SoundIDs, int(id))
		}
		if step.LogicAdvanced {
			view.LogicUpdates++
		}
	}
	if engine.RuntimeFinished(view.State) && view.State.Presentation == nil {
		// Legacy and versus sessions have no post-result route. Confirmation
		// staged the last result image before changing destination. Flush only
		// that submission; another StepVideo
		// would start the original menu/reward route and may discard the batch.
		// This commit is idempotent and emits neither input nor sound events.
		display, err := engine.CommitDisplayVideo(view.State.Display, 0, 0)
		if err != nil {
			return failed("display_unavailable", err)
		}
		view.State.Display = display
	}
	b.sessions[request.Session] = view.State
	return b.reply(request, view)
}

func (b *Bridge) JSON(raw string) string {
	data, err := json.Marshal(b.Handle([]byte(raw)))
	if err != nil {
		return `{"ok":false,"error":{"code":"encode_error","detail":"cannot serialize basketball response"}}`
	}
	return string(data)
}
