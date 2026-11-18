package engine

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/k0ngk0ng/wire-board/games/pal/resources"
)

var authorityNativeFiles = []string{"abc.mkf", "ball.mkf", "data.mkf", "f.mkf", "fbp.mkf", "fire.mkf", "gop.mkf", "map.mkf", "mgo.mkf", "pat.mkf", "rgm.mkf", "rng.mkf", "sss.mkf"}

func NewAuthorityRuntime(ctx context.Context, program, archive []byte) (*Runtime, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	buildID := identity(program)
	provenance, stamped, err := readBuildStamp(program, buildID)
	if err != nil {
		return nil, err
	}
	if len(archive) == 0 || len(archive) > 4*1024*1024 {
		return nil, errors.New("PAL authority archive exceeds its compressed limit")
	}
	owned := bytes.Clone(archive)
	resourceID, ok := authorityResourceIDs[identity(owned)]
	if !ok {
		return nil, errors.New("PAL authority archive is not registered")
	}
	reader, err := zip.NewReader(bytes.NewReader(owned), int64(len(owned)))
	if err != nil {
		return nil, err
	}
	expected := map[string]bool{"word.dat": true, "m.msg": true}
	for _, name := range authorityNativeFiles {
		expected[name] = true
	}
	if len(reader.File) != len(expected) {
		return nil, errors.New("PAL authority archive contains missing or extra files")
	}
	native := map[string][]byte{}
	total := uint64(0)
	for _, file := range reader.File {
		if !expected[file.Name] || !file.Mode().IsRegular() || file.UncompressedSize64 == 0 || file.UncompressedSize64 > 8*1024*1024 {
			return nil, fmt.Errorf("invalid PAL authority file %s", file.Name)
		}
		expected[file.Name] = false
		total += file.UncompressedSize64
		if total > 32*1024*1024 {
			return nil, errors.New("PAL authority archive exceeds its expanded limit")
		}
		input, err := file.Open()
		if err != nil {
			return nil, err
		}
		data, err := io.ReadAll(io.LimitReader(input, 8*1024*1024+1))
		closeErr := input.Close()
		if err != nil || closeErr != nil || uint64(len(data)) != file.UncompressedSize64 {
			return nil, fmt.Errorf("PAL authority file %s is corrupt", file.Name)
		}
		if file.Name != "word.dat" && file.Name != "m.msg" {
			native[file.Name] = data
		}
	}
	tables, err := resources.Load98Tables(reader)
	if err != nil {
		return nil, err
	}
	r := &Runtime{buildID: buildID, resourceID: resourceID, provenance: provenance}
	return r.initializeRulesRuntime(ctx, program, tables, native, stamped)
}
