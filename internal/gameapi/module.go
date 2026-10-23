// Package gameapi defines the host boundary, without game rules or a turn model.
package gameapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"time"
)

// BotSearch is an optional bounded worker. Input is a fast, pure projection
// that MUST remove information unavailable to the acting seat. Think receives
// only that detached projection outside the host lock, may run a local engine,
// and must respect cancellation. BotAction remains the fast legal fallback.
// A nil successful proposal means idle, just as for BotAction. Budget defaults
// to one second and cannot exceed ten seconds. Search is never persisted.
type BotSearch struct {
	Input  func(Context, Match) (json.RawMessage, error)
	Think  func(context.Context, json.RawMessage) (json.RawMessage, error)
	Budget time.Duration
}

type Context struct {
	// Automated is detached host-derived seat metadata, not client input. The
	// platform supplies exactly Players entries: true for an active bot or
	// autoplay seat. Modules may define nil semantics for direct engine callers.
	// Values reflect the current transaction: Reclaim can still see its actor
	// automated because the host clears AutoPlay after that callback succeeds.
	Automated                 []bool
	Actor, Players            int
	Version, RequestedVersion int
	Now                       time.Time
}

// PreparationContext describes an authenticated waiting-room participant.
// Bots contains one detached entry per occupied seat, from Seat.Bot only;
// it is neither padded to Capacity nor inferred from client/autoplay metadata.
type PreparationContext struct {
	Actor, Capacity int
	Host            bool
	Bots            []bool
}

// Match is a persisted envelope. Data belongs exclusively to the game. Actors
// can act now; TimeoutActors owe a decision at Deadline (Unix milliseconds).
// These lists differ for optional responses and simultaneous actions.
type Match struct {
	// Paused explicitly suspends automatic decisions and opts this saved match
	// into the host's 30-day inactive retention policy. The game owns pause /
	// resume authorization and saves its remaining action budget in Data.
	// A paused match has no deadlines, timeout actors, attention or result;
	// Actors may still accept manual resume votes through Apply.
	Paused bool `json:"paused,omitempty"`
	// HasComputerPlayers includes game-owned AI characters which do not occupy
	// platform seats. Such matches retain results/history but never earn ratings.
	HasComputerPlayers bool `json:"hasComputerPlayers,omitempty"`
	// CheckpointKey changes only at game-owned critical phases/round boundaries.
	// Realtime hosts persist these transitions before publishing them.
	CheckpointKey string          `json:"checkpointKey,omitempty"`
	Schema        int             `json:"schema"`
	Data          json.RawMessage `json:"data"`
	Actors        []int           `json:"actors"`
	TimeoutActors []int           `json:"timeoutActors"`
	Deadline      int64           `json:"deadline"`
	// Attention is optional: nil preserves the old Actors-based prompt; an
	// explicit empty slice means nobody owes a decision. Preserve [] in JSON.
	Attention []int `json:"attention"`
	// Eliminated seats have no remaining decisions or liabilities and may leave.
	// Their original index and identity remain in the match roster and results.
	Eliminated []int `json:"eliminated,omitempty"`
	// SystemDeadline invokes OnTimer, without attributing inactivity to a player.
	SystemDeadline int64    `json:"systemDeadline,omitempty"`
	Log            []string `json:"log,omitempty"`
	Result         *Result  `json:"result,omitempty"`
}

// Ranks use competition ranking (1,1,3); Winners can be a whole team.
// Team selects the platform's existing team rating policy (+20 / -10).
// Cooperative games share one outcome: all seats win, or nobody wins. They
// never award competitive ratings and all seats retain rank 1.
type Result struct {
	// Draw is an explicit competitive draw, never a shared victory. Every seat
	// has rank 1, Winners is empty, and no competitive rating points are moved.
	Draw bool `json:"draw,omitempty"`
	// Unrated marks a practice/casual match. History and scores are preserved,
	// but the host writes no competitive rating ledger entries.
	Unrated     bool            `json:"unrated,omitempty"`
	Ranks       []int           `json:"ranks"`
	Winners     []int           `json:"winners"`
	Scores      []int           `json:"scores,omitempty"`
	Team        bool            `json:"team,omitempty"`
	Cooperative bool            `json:"cooperative,omitempty"`
	Details     json.RawMessage `json:"details,omitempty"`
}

// Module callbacks must not perform I/O or mutate their inputs, except that
// BotSearch.Think may run a cancellable local engine outside the host lock.
// The host owns
// authentication, transactions, clocks, deduplication and publication. Keep all
// game-specific types, decoding and snapshot upgrades in the game's package.
type Module struct {
	ID         string                                                               `json:"id"`
	Name       string                                                               `json:"name"`
	MinPlayers int                                                                  `json:"minPlayers"`
	MaxPlayers int                                                                  `json:"maxPlayers"`
	Configure  func(capacity int, options json.RawMessage) (json.RawMessage, error) `json:"-"`
	// Prepare optionally transforms waiting-room options for an authenticated seat.
	// The host re-runs Configure on its result before atomically saving it.
	Prepare   func(PreparationContext, json.RawMessage, json.RawMessage) (json.RawMessage, error) `json:"-"`
	Start     func(Context, json.RawMessage) (Match, error)                                       `json:"-"`
	Apply     func(Context, Match, json.RawMessage) (Match, error)                                `json:"-"`
	View      func(Match, int) (any, error)                                                       `json:"-"`
	BotSearch *BotSearch                                                                          `json:"-"`
	// PreciseTimers opts non-realtime SystemDeadline events into the 50ms clock.
	// Player timeouts and bot scheduling retain their ordinary cadence.
	PreciseTimers bool `json:"-"`
	// Realtime opts into the fast SystemDeadline clock. Each due frame submits
	// all controlled actors through BotAction/Apply, then OnTimer, atomically.
	// Callbacks must stay bounded; asynchronous BotSearch cannot run in a frame.
	Realtime bool `json:"-"`
	// BotAction is a fast pure fallback; use BotSearch for expensive computation.
	// BotAction may return nil, nil when no decision is available. The host
	// tries the remaining controlled actors without persisting an idle action.
	BotAction func(Context, Match) (json.RawMessage, error) `json:"-"`
	// Reclaim restores an expired decision window without extending unrelated
	// players' windows. Realtime modules also receive Reclaim whenever autoplay
	// is cancelled, so they can clear the returning seat's previous bot controls.
	// Omit BotAction/Reclaim together to disable autoplay.
	Reclaim func(Context, Match) (Match, error) `json:"-"`
	// ReclaimOnCancel opts into Reclaim on every transition from autoplay to
	// manual control, even when this seat is not in the earliest TimeoutActors.
	// Use for independent seat deadlines. Reclaim must preserve live, submitted
	// and paused windows and every other seat's deadline. Existing modules keep
	// the normal earliest-expired-window dispatch unless they explicitly opt in.
	ReclaimOnCancel bool `json:"-"`
	// Restore optionally migrates a persisted active match at server startup.
	// It must preserve gameplay and existing deadlines, and return unchanged
	// input when already current. The host validates and persists any change.
	Restore func(Context, Match) (Match, error) `json:"-"`
	// AcceptStale may admit an older version only after authenticating the actor.
	// Revalidate the game's response token inside Apply as well.
	AcceptStale func(Context, Match, json.RawMessage) bool `json:"-"`
	// OnTimer runs with Actor == -1. Clear SystemDeadline or advance it beyond
	// Context.Now; the host commits the event atomically with history and audit.
	OnTimer func(Context, Match) (Match, error) `json:"-"`
}

func (m Module) Validate() error {
	if !regexp.MustCompile(`^[a-z][a-z0-9_-]*$`).MatchString(m.ID) || m.Name == "" || m.MinPlayers < 1 || m.MaxPlayers < m.MinPlayers || m.MaxPlayers > 32 {
		return fmt.Errorf("invalid game registration: %q", m.ID)
	}
	if m.Configure == nil || m.Start == nil || m.Apply == nil || m.View == nil || (m.BotAction == nil) != (m.Reclaim == nil) {
		return fmt.Errorf("incomplete game registration: %s", m.ID)
	}
	if m.ReclaimOnCancel && m.Reclaim == nil {
		return fmt.Errorf("reclaim-on-cancel requires autoplay callbacks: %s", m.ID)
	}
	if b := m.BotSearch; b != nil && (m.BotAction == nil || b.Input == nil || b.Think == nil || b.Budget < 0 || b.Budget > 10*time.Second) {
		return fmt.Errorf("invalid bot search registration: %s", m.ID)
	}
	if m.Realtime && (m.OnTimer == nil || m.BotSearch != nil) {
		return fmt.Errorf("invalid realtime registration: %s", m.ID)
	}
	return nil
}

func (m Match) Validate(players int) error {
	if m.Schema < 0 || !json.Valid(m.Data) || string(m.Data) == "null" || m.Deadline < 0 || m.SystemDeadline < 0 {
		return errors.New("invalid game snapshot")
	}
	if m.Paused && (m.Deadline != 0 || m.SystemDeadline != 0 || len(m.TimeoutActors) != 0 || m.Attention == nil || len(m.Attention) != 0 || m.Result != nil) {
		return errors.New("paused game must clear timers, attention and result")
	}
	for _, actors := range [][]int{m.Actors, m.TimeoutActors, m.Attention, m.Eliminated} {
		seen := map[int]bool{}
		for _, actor := range actors {
			if actor < 0 || actor >= players || seen[actor] {
				return errors.New("invalid game actor")
			}
			seen[actor] = true
		}
	}
	for _, actor := range m.Attention {
		if !slices.Contains(m.Actors, actor) {
			return errors.New("attention actor cannot act")
		}
	}
	for _, actor := range m.Eliminated {
		if slices.Contains(m.Actors, actor) {
			return errors.New("eliminated player cannot act")
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
	if len(r.Ranks) != players || (!r.Cooperative && !r.Draw && len(r.Winners) == 0) || len(m.Actors) != 0 || len(m.TimeoutActors) != 0 || m.Deadline != 0 || m.SystemDeadline != 0 || (len(r.Scores) != 0 && len(r.Scores) != players) {
		return errors.New("invalid game result")
	}
	if r.Draw && (players < 2 || r.Cooperative || len(r.Winners) != 0) {
		return errors.New("invalid drawn outcome")
	}
	if r.Cooperative && (r.Team || (len(r.Winners) != 0 && len(r.Winners) != players)) {
		return errors.New("invalid cooperative outcome")
	}
	for _, rank := range r.Ranks {
		if rank < 1 || rank > players || ((r.Cooperative || r.Draw) && rank != 1) {
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

// ValidateMatch also checks that the host can service requested capabilities.
func (module Module) ValidateMatch(match Match, players int) error {
	if err := match.Validate(players); err != nil {
		return err
	}
	if match.SystemDeadline > 0 && module.OnTimer == nil {
		return errors.New("system deadline requires OnTimer")
	}
	return nil
}
