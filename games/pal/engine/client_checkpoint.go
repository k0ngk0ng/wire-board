package engine

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"encoding/json"
)

type ReplayInput struct {
	At        uint32 `json:"at"`
	Direction int    `json:"direction"`
	Keys      uint32 `json:"keys"`
	Response  uint32 `json:"response,omitempty"`
}

type ReplayOperation struct {
	Kind        string        `json:"kind"`
	Response    uint32        `json:"response,omitempty"`
	RemainingMS uint32        `json:"remainingMS,omitempty"`
	ElapsedMS   uint32        `json:"elapsedMS,omitempty"`
	Inputs      []ReplayInput `json:"inputs,omitempty"`
}

func (c *Continuation) TakeReplay() []ReplayOperation {
	operations := c.replay
	c.replay = nil
	return operations
}

func (c *Continuation) MemoryID() string { return c.state.MemoryID }
func (c *Continuation) BuildID() string  { return c.state.BuildID }

func (c *Continuation) InputBoundary() string {
	v := c.state.View
	pc, messages := uint32(0), uint32(0)
	if c.state.Event.Kind == Dialogue {
		pc, messages = v.PC, v.Messages
	}
	b := v.Battle
	value, _ := json.Marshal([]any{c.state.Event.Kind, v.Scene, v.Mode, v.Entering, v.Menu.Kind, v.Menu.Depth, b.Active, b.Phase, b.State, b.Menu, b.Current, pc, messages})
	return string(value)
}

func (c *Continuation) ClientCheckpoint() ([]byte, error) {
	snapshot := *c.state
	snapshot.Memory = nil
	metadata, err := json.Marshal(struct {
		Snapshot Snapshot `json:"snapshot"`
		Words    []string `json:"words"`
		Messages []string `json:"messages"`
		Roles    []string `json:"roles"`
	}{snapshot, c.runtime.words, c.runtime.messages, c.runtime.roleNames})
	if err != nil {
		return nil, err
	}
	var result bytes.Buffer
	writer, err := gzip.NewWriterLevel(&result, gzip.BestSpeed)
	if err != nil {
		return nil, err
	}
	header := [16]byte{}
	copy(header[:8], "PALNET01")
	binary.LittleEndian.PutUint32(header[8:12], uint32(len(metadata)))
	binary.LittleEndian.PutUint32(header[12:16], uint32(len(c.state.Memory)))
	for _, part := range [][]byte{header[:], metadata, c.state.Memory} {
		if _, err := writer.Write(part); err != nil {
			return nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return result.Bytes(), nil
}
