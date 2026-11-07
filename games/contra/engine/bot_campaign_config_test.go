package contra

import (
	"fmt"
	"strconv"
)

// Test-only orchestration. No production strategy or rule reads these flags.
type botCampaignConfig struct {
	Enabled          bool
	Mode             string
	InfiniteLives    bool
	Seats            []int
	Frames, Target   int
	Prefix, Evidence string
}

func (c botCampaignConfig) Acceptance() bool {
	return c.Mode == "standard-terminal" || c.Mode == "assisted-clear"
}
func (c botCampaignConfig) TerminalComplete(outcome CampaignOutcome, finalized bool) bool {
	if !finalized || outcome == CampaignOngoing {
		return false
	}
	if c.Mode == "standard-terminal" {
		return outcome == CampaignFailed || outcome == CampaignCleared
	}
	return outcome == CampaignCleared
}
func readBotCampaignConfig(get func(string) string) (botCampaignConfig, error) {
	c := botCampaignConfig{Mode: get("CONTRA_BOT_MODE"), Seats: []int{1, 2}, Frames: 120000, Target: 8}
	if c.Mode == "" {
		return c, nil
	}
	switch c.Mode {
	case "standard-terminal":
	case "assisted-clear":
		c.InfiniteLives = true
		c.Frames = 300000
	case "diagnostic":
	default:
		return c, fmt.Errorf("invalid CONTRA_BOT_MODE: choose standard-terminal, assisted-clear or diagnostic")
	}
	c.Enabled = true
	c.Prefix = get("CONTRA_BOT_PREFIX")
	c.Evidence = get("CONTRA_BOT_EVIDENCE")
	if value := get("CONTRA_BOT_INFINITE_LIVES"); value != "" {
		if value != "0" && value != "1" {
			return c, fmt.Errorf("infinite-lives test flag must be 0 or 1")
		}
		requested := value == "1"
		if c.Acceptance() && requested != c.InfiniteLives {
			return c, fmt.Errorf("reserve flag conflicts with explicit acceptance mode")
		}
		c.InfiniteLives = requested
	}
	for _, field := range []struct {
		name      string
		value     *int
		low, high int
	}{{"CONTRA_BOT_FRAMES", &c.Frames, 1, 300000}, {"CONTRA_BOT_TARGET", &c.Target, 1, 8}} {
		if value := get(field.name); value != "" {
			n, err := strconv.Atoi(value)
			if err != nil || n < field.low || n > field.high {
				return c, fmt.Errorf("invalid %s", field.name)
			}
			*field.value = n
		}
	}
	if value := get("CONTRA_BOT_SEATS"); value != "" {
		n, err := strconv.Atoi(value)
		if err != nil || n < 1 || n > 2 {
			return c, fmt.Errorf("invalid bot seat filter")
		}
		c.Seats = []int{n}
	}
	if c.Acceptance() && (c.Prefix != "" || c.Target != 8) {
		return c, fmt.Errorf("acceptance requires a fresh full-target run; prefix/short target require diagnostic mode")
	}
	return c, nil
}
