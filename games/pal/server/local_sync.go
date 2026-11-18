package pal

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/k0ngk0ng/wire-board/games/pal/engine"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

type ReplayRecord struct {
	Revision   uint64                   `json:"revision"`
	Operations []engine.ReplayOperation `json:"operations"`
	Digest     string                   `json:"digest"`
	AppliedSeq uint64                   `json:"appliedSeq"`
}

type LocalView struct {
	Direction    int            `json:"direction"`
	Until        int64          `json:"until"`
	Generation   uint64         `json:"generation"`
	Revision     uint64         `json:"revision"`
	Core         string         `json:"core"`
	Digest       string         `json:"digest"`
	Boundary     string         `json:"boundary"`
	RemainingMS  uint32         `json:"remainingMS"`
	ServerAt     int64          `json:"serverAt"`
	Seq          uint64         `json:"seq"`
	AppliedSeq   uint64         `json:"appliedSeq"`
	DiscardedSeq uint64         `json:"discardedSeq"`
	Records      []ReplayRecord `json:"records,omitempty"`
	Resync       bool           `json:"resync,omitempty"`
	Checkpoint   []byte         `json:"checkpoint,omitempty"`
}

type localCommand struct {
	Seq       uint64 `json:"seq"`
	Boundary  string `json:"boundary"`
	Direction int    `json:"direction"`
	Keys      uint32 `json:"keys"`
	At        int64  `json:"at"`
}

type localState struct {
	Generation   uint64         `json:"generation"`
	Revision     uint64         `json:"revision"`
	Core         string         `json:"core"`
	Digest       string         `json:"digest"`
	Boundary     string         `json:"boundary"`
	Seq          uint64         `json:"seq"`
	AppliedSeq   uint64         `json:"appliedSeq"`
	DiscardedSeq uint64         `json:"discardedSeq"`
	Direction    int            `json:"direction"`
	Until        int64          `json:"until"`
	Queue        []localCommand `json:"queue,omitempty"`
	Records      []ReplayRecord `json:"records,omitempty"`
}

func validLocalState(local *localState) bool {
	if local == nil {
		return true
	}
	if local.Generation == 0 || local.Generation >= 1<<53 || local.Revision >= 1<<53 || local.Seq >= 1<<53 || local.AppliedSeq > local.Seq || local.DiscardedSeq > local.Seq || local.Direction < 0 || local.Direction > 4 || len(local.Queue) > 64 || len(local.Records) > 64 || len(local.Core) != 64 || len(local.Digest) != 64 || len(local.Boundary) > 256 {
		return false
	}
	previous := local.AppliedSeq
	for _, command := range local.Queue {
		if command.Seq <= previous || command.Seq > local.Seq || len(command.Boundary) > 256 || command.Direction < 0 || command.Direction > 4 || command.Keys&^uint32(0x3ffff) != 0 || command.At <= 0 {
			return false
		}
		previous = command.Seq
	}
	previous = 0
	for _, record := range local.Records {
		if record.Revision == 0 || record.Revision > local.Revision || previous > 0 && record.Revision != previous+1 || len(record.Operations) == 0 || len(record.Operations) > 64 || len(record.Digest) != 64 || record.AppliedSeq > local.Seq {
			return false
		}
		previous = record.Revision
	}
	return len(local.Records) == 0 || previous == local.Revision
}

func (s *state) recordReplay(native *engine.Continuation) {
	if s.Local == nil {
		s.Local = &localState{Generation: s.Public.Epoch, Direction: 4}
	}
	l := s.Local
	l.Core, l.Digest, l.Boundary = native.BuildID(), native.MemoryID(), native.InputBoundary()
	operations := native.TakeReplay()
	if len(operations) == 0 {
		return
	}
	l.Revision++
	l.Records = append(l.Records, ReplayRecord{l.Revision, operations, l.Digest, l.AppliedSeq})
	if len(l.Records) > 64 {
		l.Records = l.Records[len(l.Records)-64:]
	}
}

func (s *state) resetLocal(native *engine.Continuation) {
	generation := s.Public.Epoch
	if s.Local != nil {
		generation = max(generation, s.Local.Generation+1)
	}
	s.Local = &localState{Generation: generation, Direction: 4}
	s.recordReplay(native)
}

func (s state) localView() *LocalView {
	if s.Local == nil {
		return nil
	}
	l := s.Local
	return &LocalView{Generation: l.Generation, Revision: l.Revision, Core: l.Core, Digest: l.Digest, Boundary: l.Boundary, Seq: l.Seq, AppliedSeq: l.AppliedSeq, DiscardedSeq: l.DiscardedSeq, RemainingMS: s.RemainingMS, ServerAt: s.ServerAt, Direction: l.Direction, Until: l.Until}
}

func (a adapter) syncView(m gameapi.Match, viewer int) (any, error) {
	s, err := decode(m)
	if err != nil {
		return nil, err
	}
	v := s.publicView(viewer)
	if viewer != 0 {
		return v, nil
	}
	native, err := a.runtime.Continue(s.Native)
	if err != nil {
		return nil, err
	}
	if s.Local == nil {
		s.recordReplay(native)
		v.Local = s.localView()
	}
	v.Local.Checkpoint, err = native.ClientCheckpoint()
	return v, err
}

func (a adapter) streamView(m gameapi.Match, viewer int, cursor string) (gameapi.StreamViewResult, error) {
	s, err := decode(m)
	if err != nil {
		return gameapi.StreamViewResult{}, err
	}
	v := s.publicView(viewer)
	if viewer != 0 || s.Local == nil {
		return gameapi.StreamViewResult{Data: v}, nil
	}
	generation, revision, found := strings.Cut(cursor, ":")
	after, parseErr := strconv.ParseUint(revision, 10, 64)
	l := s.Local
	valid := found && parseErr == nil && generation == strconv.FormatUint(l.Generation, 10) && after <= l.Revision
	if valid && after < l.Revision {
		valid = len(l.Records) > 0 && after+1 >= l.Records[0].Revision
	}
	if !valid {
		v.Local.Resync = true
	} else {
		floor := uint64(0)
		if after > 6 {
			floor = after - 6
		}
		for _, record := range l.Records {
			if record.Revision > floor {
				v.Local.Records = append(v.Local.Records, record)
			}
		}
		if l.Seq > 0 {
			v.Field, v.Raster, v.Draws, v.Overlay = nil, nil, nil, nil
		}
	}
	return gameapi.StreamViewResult{Data: v, Cursor: fmt.Sprintf("%d:%d", l.Generation, l.Revision)}, nil
}

func validLocalAction(c gameapi.Context, s state, action Action) error {
	l := s.Local
	if s.Saving || l == nil || c.Actor != 0 || action.Epoch != l.Generation || action.Seq <= l.Seq || action.Seq >= 1<<53 || len(l.Queue) >= 64 && action.Type != "local_release" || action.Direction < 0 || action.Direction > 4 || action.Keys&^uint32(0x3ffff) != 0 || action.Prompt != 0 || action.Target != 0 || len(action.Boundary) == 0 || len(action.Boundary) > 256 || action.Type == "local_release" && (action.Direction != 4 || action.Keys != 0) {
		return errors.New("游戏已暂停或本地操作已过期")
	}
	return nil
}

func (a adapter) queueLocal(c gameapi.Context, s *state, action Action) error {
	l := s.Local
	at := c.Now.UnixMilli()
	arrived := at
	if !c.InputReceivedAt.IsZero() {
		arrived = c.InputReceivedAt.UnixMilli()
	}
	l.Seq = action.Seq
	if action.Type == "local_release" {
		l.DiscardedSeq, l.AppliedSeq, l.Queue, l.Direction, l.Until = l.Seq, l.Seq, nil, 4, 0
		return a.releaseLocalInputs(s)
	}
	if action.Keys != 0 || action.Direction != 4 {
		if action.ControlAt <= 0 || action.ControlAt > arrived || arrived-action.ControlAt > 1000 || arrived > at || at-arrived > 1000 {
			l.DiscardedSeq, l.AppliedSeq, l.Queue, l.Direction, l.Until = l.Seq, l.Seq, nil, 4, 0
			return nil
		}
	}
	l.Queue = append(l.Queue, localCommand{action.Seq, action.Boundary, action.Direction, action.Keys, at})
	return a.drainLocal(s, at)
}

func (a adapter) expireLocal(s *state, now int64) error {
	l := s.Local
	if l == nil || len(l.Queue) == 0 || now-l.Queue[0].At <= 1000 {
		return nil
	}
	l.DiscardedSeq, l.AppliedSeq, l.Queue, l.Direction, l.Until = l.Seq, l.Seq, nil, 4, 0
	return a.releaseLocalInputs(s)
}

func (a adapter) releaseLocalInputs(s *state) error {
	ctx, cancel := execution()
	defer cancel()
	native, err := a.runtime.Continue(s.Native)
	if err != nil {
		return err
	}
	if err = native.ReleaseInputs(ctx); err != nil {
		return err
	}
	s.Native, err = native.Encode()
	if err == nil {
		s.recordReplay(native)
	}
	return err
}

func (s *state) localInput(boundary string, at int64) engine.Input {
	l := s.Local
	input := engine.Input{Direction: 4}
	if l.Until >= at {
		input.Direction = l.Direction
	}
	for len(l.Queue) > 0 {
		command := l.Queue[0]
		if command.At > at || command.Boundary != boundary && (command.Keys != 0 || command.Direction != 4) || command.Keys != 0 && input.Keys != 0 {
			break
		}
		l.Queue = l.Queue[1:]
		l.AppliedSeq, l.Direction, l.Until = command.Seq, command.Direction, at+300
		input.Direction = command.Direction
		input.Keys |= command.Keys
	}
	return input
}

func (a adapter) drainLocal(s *state, now int64) error {
	l := s.Local
	if l == nil {
		return nil
	}
	for len(l.Queue) > 0 {
		command := l.Queue[0]
		if command.Boundary != l.Boundary || s.Public.Phase != "dialogue" && s.Public.Phase != "system" {
			return nil
		}
		if command.Keys&3 == 0 {
			return nil
		}
		l.Queue, l.AppliedSeq, l.Direction, l.Until = l.Queue[1:], command.Seq, 4, 0
		ctx, cancel := execution()
		native, err := a.runtime.Continue(s.Native)
		if err == nil {
			err = native.ReleaseInputs(ctx)
		}
		if err == nil {
			err = native.Step(ctx, 1)
		}
		cancel()
		if err != nil {
			return err
		}
		if err = a.observe(s, native, true, now, nil); err != nil {
			return err
		}
		s.ServerAt = now
	}
	return nil
}
