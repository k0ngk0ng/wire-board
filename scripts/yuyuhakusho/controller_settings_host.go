//go:build ignore

// Local acceptance host. Uses the actual platform authentication, SQLite,
// controller API and registered Yuyu module, with a fixture-only React shell.
package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"

	yuyu "github.com/k0ngk0ng/wire-board/games/yuyuhakusho/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"github.com/k0ngk0ng/wire-board/internal/server"
)

func main() {
	build := flag.String("build", "", "acceptance bundle")
	data := flag.String("data", "", "workspace SQLite directory")
	flag.Parse()
	s, err := server.New(server.Config{DataDir: *data, InviteCode: "controller-test", Modules: []gameapi.Module{yuyu.Module()}}, os.DirFS(*build))
	if err != nil {
		log.Fatal(err)
	}
	defer s.Close()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("http://" + listener.Addr().String() + "/")
	if err = http.Serve(listener, s.Handler()); err != nil {
		log.Fatal(err)
	}
}
