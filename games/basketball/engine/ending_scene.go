package engine

// EndingSceneContext has one authoritative copy of each Scene7 source field.
// Ending owns the actor text flags/ID, active flag and first pending upload;
// Text owns only06AE..06B7. The second queue and0307..030A live here.
// UploadControl bytes are ORed into PPUCTRL for their upload queue at CC23.
type EndingSceneContext struct {
	Ending        EndingPhaseContext `json:"ending"`
	Text          SceneTextProgress  `json:"text"`
	SecondUpload  EndingUpload       `json:"secondUpload"`
	UploadControl [4]uint8           `json:"uploadControl"`
}
type EndingSceneRequestKind string

const (
	EndingSceneTextUpload   EndingSceneRequestKind = "text-upload"
	EndingScenePresentation EndingSceneRequestKind = "presentation"
)

// Upload snapshots are events, not another persistent queue. They preserve
// text-before-phase request order when the phase subsequently writes buffer0.
type EndingSceneRequest struct {
	Kind         EndingSceneRequestKind `json:"kind"`
	Slot         uint8                  `json:"slot,omitempty"`
	Upload       EndingUpload           `json:"upload,omitempty"`
	Presentation EndingPhaseRequest     `json:"presentation,omitempty"`
}
type EndingSceneStep struct {
	State    OpeningState
	Context  EndingSceneContext
	Requests []EndingSceneRequest
	Reset    bool
}

// StepOriginalEndingScene restores the logical CAE3 order: FD0A text followed
// by FC83 ending phase and actors. The caller owns camera/draw finalization,
// NMI queue consumption, resource waits and video scheduling. This call neither
// consumes pending uploads nor makes an outer RNG draw.
func StepOriginalEndingScene(original OpeningState, context EndingSceneContext, pressed Buttons) (EndingSceneStep, error) {
	initial := EndingSceneStep{State: original, Context: context}
	r := initial
	c := &r.Context
	text, e := StepOriginalSceneText(original.Transition.Scene, c.Ending.Actor, c.Ending.TextActive, c.Text, [2]EndingUpload{c.Ending.Upload, c.SecondUpload})
	if e != nil {
		return initial, e
	}
	c.Ending.Actor, c.Ending.TextActive, c.Text = text.Context, text.Active, text.Progress
	c.Ending.Upload, c.SecondUpload = text.Uploads[0], text.Uploads[1]
	if text.ClearUploadControl {
		c.UploadControl = [4]uint8{}
	}
	for slot, upload := range text.Uploads {
		if text.QueueMask&(1<<slot) != 0 {
			r.Requests = append(r.Requests, EndingSceneRequest{Kind: EndingSceneTextUpload, Slot: uint8(slot), Upload: upload})
		}
	}
	presentation, e := StepOriginalEndingPresentation(r.State, c.Ending, pressed)
	if e != nil {
		return initial, e
	}
	r.State, c.Ending, r.Reset = presentation.State, presentation.Context, presentation.Reset
	for _, request := range presentation.Requests {
		r.Requests = append(r.Requests, EndingSceneRequest{Kind: EndingScenePresentation, Presentation: request})
	}
	return r, nil
}
