package server

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func palSummonTimerEpoch(data json.RawMessage) uint64 {
	_, tail, ok := bytes.Cut(data, []byte(`"epoch":`))
	if !ok {
		return 0
	}
	end := 0
	for end < len(tail) && tail[end] >= '0' && tail[end] <= '9' {
		end++
	}
	epoch, _ := strconv.ParseUint(string(tail[:end]), 10, 64)
	return epoch
}

func palSummonBrowserTrace(base gameapi.Module, out string) gameapi.Module {
	module := base
	record := func(name string, match gameapi.Match, c gameapi.Context) error {
		var saved struct {
			RemainingMS uint32          `json:"remainingMS"`
			TickLeftMS  int64           `json:"tickLeftMS"`
			ServerAt    int64           `json:"serverAt"`
			Public      json.RawMessage `json:"public"`
		}
		if err := json.Unmarshal(match.Data, &saved); err != nil {
			return err
		}
		trace, err := json.Marshal(map[string]any{
			"remainingMS": saved.RemainingMS,
			"tickLeftMS":  saved.TickLeftMS,
			"serverAt":    saved.ServerAt,
			"deadline":    match.SystemDeadline,
			"now":         c.Now.UnixMilli(),
			"wall":        time.Now().UnixMilli(),
			"paused":      match.Paused,
			"public":      saved.Public,
		})
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(out, name), trace, 0600)
	}
	module.SaveControl = func(c gameapi.Context, match gameapi.Match, saving bool) (gameapi.Match, error) {
		next, err := base.SaveControl(c, match, saving)
		if err == nil && saving {
			err = record("summon-saved.json", next, c)
		}
		return next, err
	}
	module.ResumeSaved = func(c gameapi.Context, match gameapi.Match) (gameapi.Match, error) {
		next, err := base.ResumeSaved(c, match)
		if err == nil {
			err = record("summon-resumed.json", next, c)
		}
		return next, err
	}
	return module
}
