package resources_test

// Written before the resource reader. Failure cases: truncated/non-monotonic
// MKF indices; offsets outside the file; negative or oversized chunk selection;
// DOS data accepted as 98; misaligned scenes/objects/scripts/role tables;
// message offsets outside M.MSG; source mutation changing loaded rules; a
// missing/corrupt original file silently replaced with invented game content.

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/k0ngk0ng/wire-board/games/pal/resources"
)

func archive(chunks ...[]byte) []byte {
	n := 4 * (len(chunks) + 1)
	b := make([]byte, n)
	for i, c := range chunks {
		binary.LittleEndian.PutUint32(b[i*4:], uint32(n))
		n += len(c)
	}
	binary.LittleEndian.PutUint32(b[len(chunks)*4:], uint32(n))
	for _, c := range chunks {
		b = append(b, c...)
	}
	return b
}

func TestArchiveBoundaries(t *testing.T) {
	input := archive([]byte{1, 2}, nil, []byte{3})
	a, err := resources.OpenArchive(input)
	if err != nil {
		t.Fatal(err)
	}
	input[len(input)-1] = 99
	if a.Count() != 3 {
		t.Fatal("wrong chunk count")
	}
	for i, want := range [][]byte{{1, 2}, {}, {3}} {
		got, err := a.Chunk(i)
		if err != nil || !bytes.Equal(got, want) {
			t.Fatal("chunk changed", i, got, err)
		}
		if len(got) > 0 {
			got[0] = 100
			again, _ := a.Chunk(i)
			if !bytes.Equal(again, want) {
				t.Fatal("chunk aliases mutable rules")
			}
		}
	}
	for _, i := range []int{-1, 3, 1 << 30} {
		if _, err := a.Chunk(i); err == nil {
			t.Fatal("invalid chunk accepted", i)
		}
	}
	for name, data := range map[string][]byte{"short": {0, 0, 0}, "misaligned": {5, 0, 0, 0, 0}, "past_eof": {40, 0, 0, 0}, "descending": {12, 0, 0, 0, 11, 0, 0, 0, 12, 0, 0, 0}, "trailing_bytes": append(archive([]byte{1}), 2)} {
		t.Run(name, func(t *testing.T) {
			if _, err := resources.OpenArchive(data); err == nil {
				t.Fatal("corrupt archive accepted")
			}
		})
	}
}

func Test98TablesRejectIncompleteData(t *testing.T) {
	for name, files := range map[string]fstest.MapFS{
		"missing":         {},
		"empty":           {"sss.mkf": {Data: archive()}, "data.mkf": {Data: archive()}, "word.dat": {}, "m.msg": {}},
		"corrupt_archive": {"sss.mkf": {Data: []byte{1, 2}}, "data.mkf": {Data: archive()}, "word.dat": {}, "m.msg": {}},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := resources.Load98Tables(files); err == nil {
				t.Fatal("incomplete original data accepted")
			}
		})
	}
	valid := func() fstest.MapFS {
		return fstest.MapFS{
			"sss.mkf":  {Data: archive(make([]byte, 32), make([]byte, 8), make([]byte, 14), []byte{0, 0, 0, 0, 1, 0, 0, 0}, make([]byte, 8))},
			"data.mkf": {Data: archive(nil, nil, nil, make([]byte, 900))},
			"word.dat": {Data: make([]byte, 10)},
			"m.msg":    {Data: []byte{1}},
		}
	}
	if _, err := resources.Load98Tables(valid()); err != nil {
		t.Fatal("structurally valid fixture rejected", err)
	}
	for _, tc := range []struct {
		name, file string
		chunk      int
		data       []byte
	}{
		{"partial_event", "sss.mkf", 0, make([]byte, 31)},
		{"partial_scene", "sss.mkf", 1, make([]byte, 7)},
		{"dos_objects", "sss.mkf", 2, make([]byte, 12)},
		{"partial_script", "sss.mkf", 4, make([]byte, 7)},
		{"partial_roles", "data.mkf", 3, make([]byte, 898)},
		{"bad_message_offsets", "sss.mkf", 3, []byte{0, 0, 0, 0, 2, 0, 0, 0}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			files := valid()
			old, _ := resources.OpenArchive(files[tc.file].Data)
			chunks := make([][]byte, old.Count())
			for i := range chunks {
				chunks[i], _ = old.Chunk(i)
			}
			chunks[tc.chunk] = tc.data
			files[tc.file] = &fstest.MapFile{Data: archive(chunks...)}
			if _, err := resources.Load98Tables(files); err == nil {
				t.Fatal("corrupt table accepted")
			}
		})
	}
}

func TestOriginal98Tables(t *testing.T) {
	root := filepath.Join("..", "..", "..", ".local", "pal", "original-98")
	if _, err := os.Stat(root); err != nil {
		t.Skip("run python3 -B games/pal/assets/fetch.py to obtain pinned original resources")
	}
	manifestBytes, err := os.ReadFile(filepath.Join("..", "assets", "source-manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Commit string `json:"commit"`
		Files  []struct {
			Path   string `json:"path"`
			Size   int    `json:"size"`
			SHA256 string `json:"sha256"`
		} `json:"files"`
	}
	if err = json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatal(err)
	}
	for _, f := range manifest.Files {
		data, err := os.ReadFile(filepath.Join(root, f.Path))
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(data)
		if len(data) != f.Size || hex.EncodeToString(sum[:]) != f.SHA256 {
			t.Fatal("original resource identity changed", f.Path)
		}
	}
	tables, err := resources.Load98Tables(os.DirFS(root))
	if err != nil {
		t.Fatal(err)
	}
	if len(tables.Events) != 5077 || len(tables.Scenes) != 295 || len(tables.Objects) != 565 || len(tables.Scripts) != 43503 || len(tables.Words) != 565 || len(tables.Messages) != 13513 {
		t.Fatal("98 table counts changed", len(tables.Events), len(tables.Scenes), len(tables.Objects), len(tables.Scripts), len(tables.Words), len(tables.Messages))
	}
	first := tables.Scenes[0]
	if first.Map != 20 || first.OnEnter != 4 || tables.Scripts[4].Opcode != 0x0046 || tables.Scripts[4].Operands != [3]uint16{32, 64, 0} {
		t.Fatal("original opening scene changed", first, tables.Scripts[4])
	}
	if !bytes.Equal(tables.Words[3], []byte{0xd7, 0xb4, 0xcc, 0xac}) {
		t.Fatal("original GBK word table changed")
	}
	out := filepath.Join("..", "..", "..", ".local", "pal", "verification")
	if err = os.MkdirAll(out, 0755); err != nil {
		t.Fatal(err)
	}
	report, _ := json.MarshalIndent(map[string]any{"test": t.Name(), "passed": true, "sourceCommit": manifest.Commit, "verifiedFiles": len(manifest.Files), "events": len(tables.Events), "scenes": len(tables.Scenes), "objects": len(tables.Objects), "instructions": len(tables.Scripts), "messages": len(tables.Messages), "openingScene": first, "scope": "original 98 resource identity and decoded rule tables; not gameplay completion"}, "", "  ")
	if err = os.WriteFile(filepath.Join(out, "tables.json"), report, 0600); err != nil {
		t.Fatal(err)
	}
}
