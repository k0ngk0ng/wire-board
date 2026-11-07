package contra

// Failure inventory before mode split: ordinary go test runs a long campaign;
// inherited grants/prefix/short targets contaminate acceptance; natural standard
// defeat is rejected as incomplete; assisted failure or diagnostic short target
// is accepted as eight-stage coverage; unknown modes and budgets silently pass.
import (
	"reflect"
	"testing"
)

func TestBotCampaignModeAdmission(t *testing.T) {
	parse := func(values map[string]string) (botCampaignConfig, error) {
		return readBotCampaignConfig(func(key string) string { return values[key] })
	}
	disabled, err := parse(map[string]string{"CONTRA_BOT_INFINITE_LIVES": "1", "CONTRA_BOT_PREFIX": "old.gz"})
	if err != nil || disabled.Enabled {
		t.Fatal("ordinary tests must not run inherited diagnostics", disabled, err)
	}
	for _, mode := range []string{"standard-terminal", "assisted-clear", "diagnostic"} {
		c, err := parse(map[string]string{"CONTRA_BOT_MODE": mode})
		if err != nil || !c.Enabled || c.Mode != mode || !reflect.DeepEqual(c.Seats, []int{1, 2}) || c.Target != 8 {
			t.Fatal(mode, c, err)
		}
		if c.InfiniteLives != (mode == "assisted-clear") {
			t.Fatal("mode did not own reserve policy")
		}
	}
	for _, values := range []map[string]string{
		{"CONTRA_BOT_MODE": "unknown"},
		{"CONTRA_BOT_MODE": "standard-terminal", "CONTRA_BOT_INFINITE_LIVES": "1"},
		{"CONTRA_BOT_MODE": "assisted-clear", "CONTRA_BOT_INFINITE_LIVES": "0"},
		{"CONTRA_BOT_MODE": "standard-terminal", "CONTRA_BOT_PREFIX": "prefix.gz"},
		{"CONTRA_BOT_MODE": "assisted-clear", "CONTRA_BOT_TARGET": "4"},
		{"CONTRA_BOT_MODE": "diagnostic", "CONTRA_BOT_SEATS": "3"},
		{"CONTRA_BOT_MODE": "standard-terminal", "CONTRA_BOT_FRAMES": "0"},
		{"CONTRA_BOT_MODE": "diagnostic", "CONTRA_BOT_FRAMES": "300001"},
	} {
		if _, err := parse(values); err == nil {
			t.Fatal("invalid mode configuration accepted", values)
		}
	}
	c, err := parse(map[string]string{"CONTRA_BOT_MODE": "diagnostic", "CONTRA_BOT_INFINITE_LIVES": "1", "CONTRA_BOT_TARGET": "4", "CONTRA_BOT_PREFIX": "prefix.gz", "CONTRA_BOT_SEATS": "2", "CONTRA_BOT_FRAMES": "10"})
	if err != nil || !c.InfiniteLives || c.Target != 4 || c.Prefix != "prefix.gz" || !reflect.DeepEqual(c.Seats, []int{2}) || c.Acceptance() {
		t.Fatal(c, err)
	}
}
func TestBotCampaignModeCompletion(t *testing.T) {
	for _, tc := range []struct {
		mode      string
		outcome   CampaignOutcome
		finalized bool
		want      bool
	}{
		{"standard-terminal", CampaignFailed, true, true}, {"standard-terminal", CampaignCleared, true, true}, {"standard-terminal", CampaignOngoing, false, false}, {"standard-terminal", CampaignFailed, false, false},
		{"assisted-clear", CampaignCleared, true, true}, {"assisted-clear", CampaignFailed, true, false}, {"assisted-clear", CampaignCleared, false, false},
	} {
		c := botCampaignConfig{Mode: tc.mode, Target: 8}
		if c.TerminalComplete(tc.outcome, tc.finalized) != tc.want {
			t.Fatal(tc)
		}
	}
}
