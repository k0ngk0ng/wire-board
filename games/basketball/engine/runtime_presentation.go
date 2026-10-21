package engine

import "fmt"

// PendingResult is the immutable expected settlement, not another live scene.
// During the exit fade Video is nil and Transition owns the scene. Activation
// transfers that scene into Video and discards Transition/Carry.
type RuntimePresentationState struct {
	PendingResult CampaignMatchResult   `json:"pendingResult"`
	Video         *PostResultVideoState `json:"video,omitempty"`
	Complete      bool                  `json:"complete"`
}

func RuntimeFinished(s RuntimeState) bool {
	return s.Outcome != nil && s.Outcome.Confirmed && (s.Presentation == nil || s.Presentation.Complete)
}
func RuntimePresentationActive(s RuntimeState) bool {
	return s.Presentation != nil && !s.Presentation.Complete
}
func RuntimePresentationNeedsConfirm(s RuntimeState) bool {
	if !RuntimePresentationActive(s) || s.Presentation.Video == nil {
		return false
	}
	v := s.Presentation.Video
	t := v.Scene.Transition
	return t.Scene == 7 && t.Match.Phase == 9 && t.Match.Entered && v.Resource.Kind == "" && (v.Clock.Stage == PostResultOuterThreshold || v.Clock.Stage == PostResultOuterChange)
}

func activateRuntimePresentation(r *RuntimeStep) error {
	s := &r.State
	if s.Presentation == nil || s.Presentation.Video != nil || s.Transition == nil {
		return fmt.Errorf("invalid pending presentation handoff")
	}
	video, e := NewOriginalPostResultVideoAfterResult(s.Transition.Scene, s.Display, *s.Config.Campaign, s.LogicFrame)
	if e != nil {
		return e
	}
	s.Presentation.Video = &video
	s.Transition = nil
	s.Carry = nil
	s.AwaitingNextPeriod = false
	s.FramesUntilLogic = 0
	s.WaitVideoFrames = 0
	r.PresentationStarted = true
	raster := postResultRaster(video)
	r.PresentationRaster = &raster
	return nil
}
func stepRuntimePresentation(r *RuntimeStep) error {
	s := &r.State
	p := s.Presentation
	if p == nil || p.Video == nil || p.Complete {
		return fmt.Errorf("inactive presentation video")
	}
	var input [4]ControllerFrame
	for i := range input {
		input[i] = ControllerFrame{Held: s.Held[i] | s.PendingPressed[i], Pressed: s.PendingPressed[i]}
	}
	step, e := StepOriginalPostResultVideo(*p.Video, input)
	if e != nil {
		return e
	}
	p.Video = &step.State
	s.LogicFrame = step.State.LogicByte
	if step.DispatchStarted {
		s.LogicSteps++
		s.PendingPressed = [4]Buttons{}
		r.LogicAdvanced = true
	}
	r.PresentationEffects = step.Effects
	raster := step.LiveRaster
	r.PresentationRaster = &raster
	for _, effect := range step.Effects {
		if effect.Kind == "sound" {
			r.SoundIDs = append(r.SoundIDs, effect.Value)
		}
	}
	if step.Handoff || step.Reset {
		if step.State.Progress != p.PendingResult.Progress {
			return fmt.Errorf("completed presentation progress differs from confirmed settlement")
		}
		p.Complete = true
	}
	return nil
}
