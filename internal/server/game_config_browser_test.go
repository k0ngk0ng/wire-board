package server

import (
	"fmt"
	"net/http/httptest"
	"os"
	"os/exec"
	"testing"
	"testing/fstest"

	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func TestAdminGameConfigBrowserE2E(t *testing.T) {
	runAdminGameBrowser(t, "web/tests/admin-game-config-browser.test.mjs")
}

func TestAdminGameListBrowserE2E(t *testing.T) {
	runAdminGameBrowser(t, "web/tests/admin-game-list-browser.test.mjs")
}

func runAdminGameBrowser(t *testing.T, script string) {
	t.Helper()
	if os.Getenv("WIRE_BOARD_GAME_CONFIG_BROWSER") != "1" {
		t.Skip("explicit browser acceptance only")
	}
	modules := make([]gameapi.Module, 36)
	for index := range modules {
		module := acceptanceModule()
		module.ID = fmt.Sprintf("configfixture%d", index)
		module.Name = fmt.Sprintf("基础配置长名称游戏与游戏列表换行验收%d", index)
		modules[index] = module
	}
	s, err := New(Config{DataDir: t.TempDir(), InviteCode: "test-invite", ClientLoading: true, AdminUsername: "ConfigRoot", AdminPassword: "config-test-password-123", Modules: modules}, fstest.MapFS{})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()
	command := exec.Command("node", "--test", script)
	command.Dir = "../.."
	command.Env = append(os.Environ(), "GAME_CONFIG_API_URL="+ts.URL)
	command.Stdout, command.Stderr = os.Stdout, os.Stderr
	if err := command.Run(); err != nil {
		t.Fatal(err)
	}
}
