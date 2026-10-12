// Package gameapi defines the host boundary, without game rules or a turn model.
package gameapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"time"
)

type Context struct {
	Actor, Players            int
	Version, RequestedVersion int
	Now                       time.Time
}

// Match is a persisted envelope. Data belongs exclusively to the game. Actors
// can act now; TimeoutActors owe a decision at Deadline (Unix milliseconds).
// These lists differ for optional responses and simultaneous actions.
type Match struct {
	Schema        int             `json:"schema"`
	Data          json.RawMessage `json:"data"`
	Actors        []int           `json:"actors"`
	TimeoutActors []int           `json:"timeoutActors"`
	Deadline      int64           `json:"deadline"`
	Log           []string        `json:"log,omitempty"`
	Result        *Result         `json:"result,omitempty"`
}

// Ranks use competition ranking (1,1,3); Winners can be a whole team.
// Team selects the platform's existing team rating policy (+20 / -10).
type Result struct {
	Ranks   []int           `json:"ranks"`
	Winners []int           `json:"winners"`
	Scores  []int           `json:"scores,omitempty"`
	Team    bool            `json:"team,omitempty"`
	Details json.RawMessage `json:"details,omitempty"`
}

// Module callbacks must not perform I/O or mutate their inputs. The host owns
// authentication, transactions, clocks, deduplication and publication. Keep all
// game-specific types, decoding and snapshot upgrades in the game's package.
type Module struct {
	ID         string                                                               `json:"id"`
	Name       string                                                               `json:"name"`
	MinPlayers int                                                                  `json:"minPlayers"`
	MaxPlayers int                                                                  `json:"maxPlayers"`
	Configure  func(capacity int, options json.RawMessage) (json.RawMessage, error) `json:"-"`
	Start      func(Context, json.RawMessage) (Match, error)                        `json:"-"`
	Apply      func(Context, Match, json.RawMessage) (Match, error)                 `json:"-"`
	View       func(Match, int) (any, error)                                        `json:"-"`
	BotAction  func(Context, Match) (json.RawMessage, error)                        `json:"-"`
	// Reclaim restores an expired decision window without extending unrelated
	// players' windows. Omit BotAction/Reclaim together to disable autoplay.
	Reclaim func(Context, Match) (Match, error) `json:"-"`
	// AcceptStale may admit an older version only after authenticating the actor.
	// Revalidate the game's response token inside Apply as well.
	AcceptStale func(Context, Match, json.RawMessage) bool `json:"-"`
}

func (m Module) Validate() error {
	if !regexp.MustCompile(`^[a-z][a-z0-9_-]*$`).MatchString(m.ID) || m.Name == "" || m.MinPlayers < 1 || m.MaxPlayers < m.MinPlayers || m.MaxPlayers > 32 {
		return fmt.Errorf("invalid game registration: %q", m.ID)
	}
	if m.Configure == nil || m.Start == nil || m.Apply == nil || m.View == nil || (m.BotAction == nil) != (m.Reclaim == nil) {
		return fmt.Errorf("incomplete game registration: %s", m.ID)
	}
	return nil
}

func (m Match) Validate(players int) error {
	if m.Schema < 0 || !json.Valid(m.Data) || string(m.Data) == "null" || m.Deadline < 0 {
		return errors.New("invalid game snapshot")
	}
	for _, actors := range [][]int{m.Actors, m.TimeoutActors} {
		seen := map[int]bool{}
		for _, actor := range actors {
			if actor < 0 || actor >= players || seen[actor] {
				return errors.New("invalid game actor")
			}
			seen[actor] = true
		}
	}
	for _, actor := range m.TimeoutActors {
		if !slices.Contains(m.Actors, actor) {
			return errors.New("timeout actor cannot act")
		}
	}
	if m.Result == nil {
		if (m.Deadline == 0) != (len(m.TimeoutActors) == 0) {
			return errors.New("invalid decision deadline")
		}
		return nil
	}
	r := m.Result
	if len(r.Ranks) != players || len(r.Winners) == 0 || len(m.Actors) != 0 || len(m.TimeoutActors) != 0 || m.Deadline != 0 || (len(r.Scores) != 0 && len(r.Scores) != players) {
		return errors.New("invalid game result")
	}
	for _, rank := range r.Ranks {
		if rank < 1 || rank > players {
			return errors.New("invalid game rank")
		}
	}
	seen := map[int]bool{}
	for _, winner := range r.Winners {
		if winner < 0 || winner >= players || r.Ranks[winner] != 1 || seen[winner] {
			return errors.New("invalid game winner")
		}
		seen[winner] = true
	}
	if len(r.Details) != 0 && !json.Valid(r.Details) {
		return errors.New("invalid result details")
	}
	return nil
}
