// Local HTTP fixture for accept_wishes.py. It exercises production server/UI
// without initializing unrelated native game runtimes or publishing assets.
package main

import (
	"log"
	"net/http"
	"os"

	starfort "github.com/k0ngk0ng/wire-board/games/starfort/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"github.com/k0ngk0ng/wire-board/internal/server"
)

func main() {
	s, err := server.New(server.Config{
		DataDir: os.Getenv("DATA_DIR"), InviteCode: os.Getenv("INVITE_CODE"),
		AdminUsername: os.Getenv("ADMIN_USERNAME"), AdminPassword: os.Getenv("ADMIN_PASSWORD"),
		OSSEndpoint: os.Getenv("OSS_ENDPOINT"), OSSBucket: os.Getenv("OSS_BUCKET"),
		OSSAccessKeyID: os.Getenv("OSS_ACCESS_KEY_ID"), OSSAccessKeySecret: os.Getenv("OSS_ACCESS_KEY_SECRET"),
		WishImageBaseURL: os.Getenv("WISH_IMAGE_BASE_URL"), WishImagePrefix: os.Getenv("WISH_IMAGE_PREFIX"),
		WishAgentToken: os.Getenv("WISH_AGENT_TOKEN"), Version: os.Getenv("WIRE_BOARD_VERSION"),
		Modules: []gameapi.Module{starfort.Module()},
	}, os.DirFS("web/dist"))
	if err != nil {
		log.Fatal(err)
	}
	defer s.Close()
	log.Fatal(http.ListenAndServe(os.Getenv("ADDR"), s.Handler()))
}
