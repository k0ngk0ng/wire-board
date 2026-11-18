package engine

import (
	"context"
	"errors"
)

type Continuation struct {
	runtime *Runtime
	state   *Snapshot
}

type ContinuationAdvance struct {
	Audio       []TimedAudioCommand
	RemainingMS uint32
	Steps       int
	InputReset  bool
}

func newContinuation(r *Runtime, s Snapshot) *Continuation {
	return &Continuation{runtime: r, state: &s}
}

func (r *Runtime) Continue(encoded []byte) (*Continuation, error) {
	s, err := r.Decode(encoded)
	if err != nil {
		return nil, err
	}
	return newContinuation(r, s), nil
}

func (r *Runtime) StartCampaignContinuationForPlayers(ctx context.Context, seed uint32, players int) (*Continuation, error) {
	s, err := r.StartCampaignForPlayers(ctx, seed, players)
	if err != nil {
		return nil, err
	}
	return newContinuation(r, s), nil
}

func (c *Continuation) Event() Event            { return c.state.Event }
func (c *Continuation) View() Projection        { return c.runtime.project(*c.state) }
func (c *Continuation) Encode() ([]byte, error) { return encodeValidated(*c.state) }

func (c *Continuation) Step(ctx context.Context, response uint32) error {
	if c.state.Event.Kind == SceneComplete {
		return errors.New("opening script already completed")
	}
	module, err := c.runtime.restoreValidated(ctx, *c.state)
	if err != nil {
		return err
	}
	defer module.Close(context.Background())
	event, err := c.runtime.run(ctx, module, true, c.state.Entry, Input{Direction: 4, Response: response})
	if err != nil {
		return err
	}
	next, err := c.runtime.captureWithMemory(ctx, module, c.state.Entry, event, c.state.Memory)
	if err == nil {
		*c.state = next
	}
	return err
}

func (c *Continuation) ReleaseInputs(ctx context.Context) error {
	module, err := c.runtime.restoreValidated(ctx, *c.state)
	if err != nil {
		return err
	}
	defer module.Close(context.Background())
	if _, err = call(ctx, module, "wb_release"); err != nil {
		return err
	}
	next, err := c.runtime.captureWithMemory(ctx, module, c.state.Entry, c.state.Event, c.state.Memory)
	if err == nil {
		*c.state = next
	}
	return err
}

func (c *Continuation) AdvanceScheduled(ctx context.Context, remainingMS, elapsedMS uint32, inputAt func(uint32) Input) (ContinuationAdvance, error) {
	next, err := c.runtime.advanceScheduledValidated(ctx, *c.state, remainingMS, elapsedMS, inputAt, c.state.Memory)
	if err != nil {
		return ContinuationAdvance{}, err
	}
	*c.state = next.Snapshot
	return ContinuationAdvance{Audio: next.Audio, RemainingMS: next.RemainingMS, Steps: next.Steps, InputReset: next.InputReset}, nil
}
