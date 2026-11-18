package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/games/pal/engine"
	pal "github.com/k0ngk0ng/wire-board/games/pal/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func palCampaignBrowserModule(t *testing.T, label string) (gameapi.Module, []byte) {
	t.Helper()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	read := func(name string) []byte {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	digest := func(data []byte) string {
		value := sha256.Sum256(data)
		return hex.EncodeToString(value[:])
	}
	var catalog struct {
		Version    int
		CoreSHA256 string
		Entries    map[string]struct {
			Root, Snapshot, SnapshotSHA256, Report, ReportSHA256, SavedLabel string
		}
	}
	if err := json.Unmarshal(read("games/pal/tests/browser-campaign-roots.json"), &catalog); err != nil {
		t.Fatal(err)
	}
	entry, ok := catalog.Entries[label]
	valid := regexp.MustCompile(`^[a-z][a-z0-9-]*\.json$`)
	if catalog.Version != 1 || !ok || !valid.MatchString(entry.Root) || !valid.MatchString(entry.Snapshot) || !valid.MatchString(entry.Report) || entry.SavedLabel == "" {
		t.Fatal("unregistered genuine campaign browser entry", label)
	}
	var roots struct {
		Candidates map[string]struct {
			Profile, CoreSHA256 string
			Roots               map[string]struct{ SHA256, ReportSHA256 string }
		}
	}
	if err := json.Unmarshal(read("games/pal/tests/campaign-roots.json"), &roots); err != nil {
		t.Fatal(err)
	}
	registered := roots.Candidates["audio"]
	accepted, ok := registered.Roots[entry.Root]
	if !ok || registered.Profile != "hp-9999" || registered.CoreSHA256 != catalog.CoreSHA256 || accepted.ReportSHA256 != entry.ReportSHA256 {
		t.Fatal("campaign browser entry is not bound to the accepted candidate and full report")
	}
	base := ".local/pal/verification/runtime-candidates/audio/development-health/hp-9999/"
	if digest(read(base+strings.TrimSuffix(entry.Root, ".json")+"-1.json")) != accepted.SHA256 {
		t.Fatal("accepted complete campaign root changed")
	}
	reportBytes, snapshot := read(base+entry.Report), read(base+entry.Snapshot)
	if digest(reportBytes) != entry.ReportSHA256 || digest(snapshot) != entry.SnapshotSHA256 {
		t.Fatal("genuine campaign report or browser snapshot changed")
	}
	var report struct {
		Passed, ResumedTestRoute bool
		Proof                    struct{ Saved map[string]bool }
	}
	if err := json.Unmarshal(reportBytes, &report); err != nil || !report.Passed || report.ResumedTestRoute || !report.Proof.Saved[entry.SavedLabel] {
		t.Fatal("campaign entry requires its complete non-debug story acceptance", err)
	}
	program := read(".local/pal/runtime-candidates/audio/hp-9999/rules.wasm")
	if digest(program) != catalog.CoreSHA256 {
		t.Fatal("actual campaign browser native core changed")
	}
	runtime, err := engine.NewAuthorityRuntime(context.Background(), program, read(".local/pal/authority-prepared/authority.zip"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { runtime.Close(context.Background()) })
	if runtime.Identity().Profile != "development-health:9999" || runtime.Identity().Purpose != "campaign" {
		t.Fatal("genuine browser entry requires the actual isolated development campaign core")
	}
	module := pal.NewModule(runtime)
	resume := module.ResumeSaved
	module.Start = func(c gameapi.Context, _ json.RawMessage) (gameapi.Match, error) {
		match, err := palBrowserImportClock(c, gameapi.Match{Schema: 1, Data: snapshot})
		if err != nil {
			return gameapi.Match{}, err
		}
		return resume(c, match)
	}
	return module, snapshot
}

func palBrowserImportClock(c gameapi.Context, match gameapi.Match) (gameapi.Match, error) {
	var imported map[string]json.RawMessage
	if err := json.Unmarshal(match.Data, &imported); err != nil {
		return gameapi.Match{}, err
	}
	var serverAt int64
	if err := json.Unmarshal(imported["serverAt"], &serverAt); err != nil {
		return gameapi.Match{}, err
	}
	var public pal.View
	if err := json.Unmarshal(imported["public"], &public); err != nil {
		return gameapi.Match{}, err
	}
	delta := c.Now.UnixMilli() - serverAt
	if public.Audio != nil {
		if public.Audio.Music != nil {
			public.Audio.Music.StartedAt += delta
		}
		for index := range public.Audio.Sounds {
			public.Audio.Sounds[index].StartedAt += delta
		}
	}
	var err error
	imported["serverAt"], err = json.Marshal(c.Now.UnixMilli())
	if err != nil {
		return gameapi.Match{}, err
	}
	imported["public"], err = json.Marshal(public)
	if err != nil {
		return gameapi.Match{}, err
	}
	match.Data, err = json.Marshal(imported)
	return match, err
}

func TestPALAcceptedCampaignEntryStartsAndProjects(t *testing.T) {
	if os.Getenv("PAL_CAMPAIGN_ENTRY") != "1" {
		t.Skip("explicit accepted genuine campaign entry projection")
	}
	module, source := palCampaignBrowserModule(t, "prayer-ritual")
	original := append([]byte(nil), source...)
	var stored struct {
		ServerAt int64
		Public   pal.View
	}
	if err := json.Unmarshal(source, &stored); err != nil {
		t.Fatal(err)
	}
	match, err := module.Start(gameapi.Context{Now: time.Now(), Players: 1, Actor: 0, Version: 1}, nil)
	if err != nil {
		t.Fatal("start", err)
	}
	for name, view := range map[string]func(gameapi.Match, int) (any, error){"view": module.View, "snapshot": module.SyncSnapshot} {
		value, err := view(match, 0)
		if err != nil {
			t.Fatal(name, err)
		}
		if value.(pal.View).Scene != 242 {
			t.Fatal(name, "actual altar entry changed")
		}
		current := value.(pal.View)
		if stored.Public.Audio == nil || stored.Public.Audio.Music == nil || current.Audio == nil || current.Audio.Music == nil || current.Audio.Music.StartedAt-current.ControlAt != stored.Public.Audio.Music.StartedAt-stored.ServerAt {
			t.Fatal(name, "campaign import changed its relative music position")
		}
	}
	if _, err := module.StreamView(match, 0, ""); err != nil {
		t.Fatal("stream", err)
	}
	if string(original) != string(source) {
		t.Fatal("browser import changed the accepted campaign checkpoint")
	}
}
