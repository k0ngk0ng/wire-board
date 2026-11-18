package engine

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"
)

func TestCatalogDataIncludesCompleteReferenceFacts(t *testing.T) {
	catalog := CatalogData()
	if catalog.Version != "1.7.3" || len(catalog.Species) != 517 || len(catalog.Enemies) != 347 || len(catalog.Routes) != 47 {
		t.Fatalf("incomplete reference catalog: version=%s species=%d enemies=%d routes=%d", catalog.Version, len(catalog.Species), len(catalog.Enemies), len(catalog.Routes))
	}
	waves := 0
	for _, route := range catalog.Routes {
		waves += len(route.Waves)
	}
	if waves != 4007 {
		t.Fatalf("wave count=%d", waves)
	}
}

func TestCatalogInstallAndSingleRouteMergeAreAtomic(t *testing.T) {
	before, err := json.Marshal(CatalogData())
	if err != nil {
		t.Fatal(err)
	}
	for _, invalid := range [][]byte{[]byte("{"), []byte(`{"version":"unknown"}`), []byte(`{"version":"1.7.3","species":{}}`)} {
		if InstallCatalog(invalid) == nil {
			t.Fatal("invalid catalog was accepted")
		}
		after, _ := json.Marshal(CatalogData())
		if !bytes.Equal(before, after) {
			t.Fatal("failed installation changed the active catalog")
		}
	}
	core, err := os.ReadFile("../assets/public/pokepath/v1/catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := InstallCatalog(core); err != nil {
		t.Fatal(err)
	}
	if len(CatalogData().Routes["0"].Waves) != 100 {
		t.Fatal("installing metadata dropped native route rules")
	}
	route, err := os.ReadFile("../assets/public/pokepath/v1/routes/0.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := InstallRoute(route); err != nil {
		t.Fatal(err)
	}
	before, _ = json.Marshal(CatalogData())
	if InstallRoute([]byte(`{"id":999,"width":720,"height":624}`)) == nil {
		t.Fatal("unknown route was accepted")
	}
	after, _ := json.Marshal(CatalogData())
	if !bytes.Equal(before, after) {
		t.Fatal("invalid route changed catalog data")
	}
}

func TestCatalogMusicReferencesAndInvalidPlaylistRemainAtomic(t *testing.T) {
	catalog := CatalogData()
	if len(catalog.Songs) != 41 || len(catalog.Playlist) != 41 || catalog.Playlist[3] != "route10" || catalog.Songs["route10"].ID != 3 || catalog.Songs["route10"].Audio != "src/assets/sounds/music/route10.mp3" || catalog.Routes["9"].MusicKey != "route10" || catalog.Routes["46"].MusicKey != "halloween" {
		t.Fatal("music facts do not preserve the source playlist and route associations")
	}
	before, err := json.Marshal(catalog)
	if err != nil {
		t.Fatal(err)
	}
	for _, invalid := range []string{"playlist", "route", "audio"} {
		var candidate Catalog
		if err := json.Unmarshal(before, &candidate); err != nil {
			t.Fatal(err)
		}
		switch invalid {
		case "playlist":
			candidate.Playlist[1] = candidate.Playlist[0]
		case "route":
			route := candidate.Routes["0"]
			route.MusicKey = "unknown-song"
			candidate.Routes["0"] = route
		case "audio":
			song := candidate.Songs["route1"]
			song.Audio = "../outside.mp3"
			candidate.Songs["route1"] = song
		}
		raw, _ := json.Marshal(candidate)
		if InstallCatalog(raw) == nil {
			t.Fatalf("invalid music %s was accepted", invalid)
		}
		after, _ := json.Marshal(CatalogData())
		if !bytes.Equal(before, after) {
			t.Fatal("failed music installation changed the active catalog")
		}
	}
}

func TestCatalogChessPlacementsKeepGridCoordinates(t *testing.T) {
	catalog := CatalogData()
	route := catalog.Routes["35"]
	if len(route.ChessPlacements) != 16 || route.ChessPlacements[0] != (ChessPlacement{Key: "p1", Species: "pawniard", X: 456, Y: 144}) {
		t.Fatal("chess pieces do not preserve the board grid")
	}
	before, _ := json.Marshal(catalog)
	route.ChessPlacements = append([]ChessPlacement{}, route.ChessPlacements...)
	route.ChessPlacements[0].Species = "unknown-species"
	raw, _ := json.Marshal(route)
	if InstallRoute(raw) == nil {
		t.Fatal("unknown chess species was accepted")
	}
	after, _ := json.Marshal(CatalogData())
	if !bytes.Equal(before, after) {
		t.Fatal("failed chess installation changed the active catalog")
	}
}
