package basketball

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/basketball/engine"
)

// Failure inventory before the continuation hook: silently opt an old campaign
// into recovery after Continue, lose true policy on next opponent, infer policy
// from entry password/new-room defaults, or fail to carry it into playing after
// JSON restore. The completed-match helper is real engine play; changing only
// its policy models an old compatible save, not an original full-match claim.
func TestCampaignRecoveryPolicyPersistsAcrossContinue(t *testing.T) {
	for _, edited := range []bool{false, true} {
		outcome := "victory"
		if edited {
			outcome = "defeat"
		}
		for _, policy := range []bool{false, true} {
			name := outcome + "-original"
			if policy {
				name = outcome + "-repaired"
			}
			t.Run(name, func(t *testing.T) {
				ctx, match := campaignFinishedHost(t, edited)
				state, err := decode(match)
				if err != nil {
					t.Fatal(err)
				}
				state.Runtime.Config.RecoverUnsupportedBall = policy
				if state.Runtime.Playing != nil {
					playing := *state.Runtime.Playing
					state.Runtime.Playing = &playing
					state.Runtime.Playing.Context.Actions.RecoverUnsupportedBall = policy
				}
				match, err = state.match()
				if err != nil {
					t.Fatal(err)
				}
				if !policy && bytes.Contains(match.Data, []byte("recoverUnsupportedBall")) {
					t.Fatal("legacy false must remain omitted")
				}
				match, err = restore(ctx, match)
				if err != nil {
					t.Fatal(err)
				}
				ctx.Actor = 0
				if state.Campaign.Stage != "prepare" {
					match, err = apply(ctx, match, campaignRequest(t, match, "continue", nil))
					if err != nil {
						t.Fatal(err)
					}
				}
				state, err = decode(match)
				if err != nil {
					t.Fatal(err)
				}
				if state.Runtime.Config.RecoverUnsupportedBall != policy {
					t.Fatal("Continue changed campaign policy")
				}
				if !edited {
					view, err := state.campaignView(0)
					if err != nil {
						t.Fatal(err)
					}
					if len(view.Opponents) == 0 {
						t.Fatal("completed nonfinal match has no next opponent")
					}
					opponent := view.Opponents[len(view.Opponents)-1]
					match, err = apply(ctx, match, campaignRequest(t, match, "opponent", map[string]any{"opponent": opponent}))
					if err != nil {
						t.Fatal(err)
					}
				}
				match, err = apply(ctx, match, campaignRequest(t, match, "start", nil))
				if err != nil {
					t.Fatal(err)
				}
				state, err = decode(match)
				if err != nil {
					t.Fatal(err)
				}
				if state.Runtime.Config.RecoverUnsupportedBall != policy {
					t.Fatalf("next campaign match silently changed policy: want%t got%t", policy, state.Runtime.Config.RecoverUnsupportedBall)
				}
				raw, err := json.Marshal(state.Runtime)
				if err != nil {
					t.Fatal(err)
				}
				var runtime engine.RuntimeState
				if err = json.Unmarshal(raw, &runtime); err != nil {
					t.Fatal(err)
				}
				for n := 0; n < 600 && runtime.Playing == nil; n++ {
					step, err := engine.StepVideo(runtime, [4]engine.Buttons{})
					if err != nil {
						t.Fatal(err)
					}
					runtime = step.State
				}
				if runtime.Playing == nil || runtime.Config.RecoverUnsupportedBall != policy || runtime.Playing.Context.Actions.RecoverUnsupportedBall != policy {
					t.Fatal("restored next opening lost policy")
				}
			})
		}
	}
}

func TestNewCampaignAndPasswordUseApprovedRecoveryDefault(t *testing.T) {
	for _, request := range []string{`{"mode":"solo","campaign":{"opponent":1}}`, `{"mode":"solo","campaign":{"password":"020010","opponent":1}}`} {
		raw, err := configure(1, json.RawMessage(request))
		if err != nil {
			t.Fatal(err)
		}
		var options Options
		if err = json.Unmarshal(raw, &options); err != nil {
			t.Fatal(err)
		}
		runtime, err := newCampaignMatch(options, 0x12345678)
		if err != nil || !runtime.Config.RecoverUnsupportedBall {
			t.Fatal("new campaign/password entry did not opt into approved repair", err)
		}
	}
}
