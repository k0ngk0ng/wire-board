package engine

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"testing"
)

func TestAuthorityArchiveRejectsUnknownMissingExtraPathsAndOversizedFiles(t *testing.T) {
	if os.Getenv("PAL_AUTHORITY_E2E") != "1" {
		t.Skip("explicit original prepared authority corruption and package-boundary cases")
	}
	program, err := os.ReadFile("../../../.local/pal/runtime-candidates/audio/original/rules.wasm")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile("../../../.local/pal/authority-prepared/authority.zip")
	if err != nil {
		t.Fatal(err)
	}
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{}
	for _, file := range reader.File {
		files[file.Name], err = fs.ReadFile(reader, file.Name)
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, variant := range []string{"missing", "extra-client-pixels", "escaping", "symlink", "oversized"} {
		t.Run(variant, func(t *testing.T) {
			var buffer bytes.Buffer
			writer := zip.NewWriter(&buffer)
			for name, payload := range files {
				if name == "m.msg" && variant == "missing" {
					continue
				}
				header := &zip.FileHeader{Name: name, Method: zip.Deflate}
				header.SetMode(0644)
				if name == "m.msg" {
					switch variant {
					case "escaping":
						header.Name = "../m.msg"
					case "symlink":
						header.SetMode(fs.ModeSymlink | 0644)
					case "oversized":
						payload = bytes.Repeat([]byte{1}, 8*1024*1024+1)
					}
				}
				file, err := writer.CreateHeader(header)
				if err != nil {
					t.Fatal(err)
				}
				if _, err = file.Write(payload); err != nil {
					t.Fatal(err)
				}
			}
			if variant == "extra-client-pixels" {
				file, _ := writer.Create("cover.png")
				file.Write([]byte("client pixels"))
			}
			if err := writer.Close(); err != nil {
				t.Fatal(err)
			}
			encoded := buffer.Bytes()
			hash := sha256.Sum256(encoded)
			id := hex.EncodeToString(hash[:])
			authorityResourceIDs[id] = "58b3fddfd1bcdc37b6816cfd3ddba8a821683ff6f13b4cd5cbbab6f8ffe7255c"
			t.Cleanup(func() { delete(authorityResourceIDs, id) })
			if runtime, err := NewAuthorityRuntime(context.Background(), program, encoded); err == nil {
				runtime.Close(context.Background())
				t.Fatal("invalid registered authority package accepted", variant)
			}
		})
	}
	corrupt := bytes.Clone(data)
	corrupt[len(corrupt)/2] ^= 1
	if runtime, err := NewAuthorityRuntime(context.Background(), program, corrupt); err == nil {
		runtime.Close(context.Background())
		t.Fatal("unregistered changed archive accepted")
	}
}
