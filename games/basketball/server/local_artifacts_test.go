package basketball_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	game "github.com/k0ngk0ng/wire-board/games/basketball/server"
	"github.com/k0ngk0ng/wire-board/internal/gametest"
)

func TestBasketballHTTPArtifactsUseGameDirectory(t *testing.T) {
	t.Run("session", func(t *testing.T) {
		gametest.NewHTTP(t, 1, game.Module(), "basketball")
	})
	name := strings.ReplaceAll(t.Name()+"/session", "/", "-") + ".json"
	file := filepath.Join("..", "..", "..", ".local/basketball/verification/http", name)
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var record struct {
		Passed bool `json:"passed"`
	}
	if err := json.Unmarshal(data, &record); err != nil || !record.Passed {
		t.Fatalf("HTTP artifact failed: %s, %v", data, err)
	}
}
