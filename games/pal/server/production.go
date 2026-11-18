package pal

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"regexp"

	"github.com/k0ngk0ng/wire-board/games/pal/engine"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

type ProductionBundle struct {
	Module   gameapi.Module
	Identity engine.RuntimeIdentity
	runtimes []*engine.Runtime
}

type productionManifest struct {
	Format          int      `json:"format"`
	ResourceID      string   `json:"resourceID"`
	AuthoritySHA256 string   `json:"authoritySHA256"`
	Current         string   `json:"current"`
	Retained        []string `json:"retained"`
}

func readProductionFile(files fs.FS, name string, maximum int64) ([]byte, error) {
	file, err := files.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > maximum {
		return nil, fmt.Errorf("invalid PAL production file %s", name)
	}
	data, err := io.ReadAll(io.LimitReader(file, maximum+1))
	if err != nil || int64(len(data)) != info.Size() {
		return nil, fmt.Errorf("PAL production file %s is corrupt", name)
	}
	return data, nil
}

func OpenProductionBundle(ctx context.Context, files fs.FS) (*ProductionBundle, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	data, err := readProductionFile(files, "manifest.json", 65536)
	if err != nil {
		return nil, err
	}
	var spec productionManifest
	if err = strict(data, &spec, 65536); err != nil {
		return nil, err
	}
	validID := regexp.MustCompile(`^[a-f0-9]{64}$`)
	if spec.Format != 1 || !validID.MatchString(spec.ResourceID) || !validID.MatchString(spec.AuthoritySHA256) || !validID.MatchString(spec.Current) || len(spec.Retained) > 64 {
		return nil, errors.New("invalid PAL production version manifest")
	}
	ids := append([]string{spec.Current}, spec.Retained...)
	expected := map[string]bool{"manifest.json": true, "authority.zip": true}
	for _, id := range ids {
		name := "cores/" + id + ".wasm"
		if !validID.MatchString(id) || expected[name] {
			return nil, errors.New("PAL production manifest has an invalid or duplicate core")
		}
		expected[name] = true
	}
	if err = fs.WalkDir(files, ".", func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() && (name == "." || name == "cores") {
			return nil
		}
		if !expected[name] || !entry.Type().IsRegular() {
			return fmt.Errorf("unexpected PAL production payload %s", name)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	archive, err := readProductionFile(files, "authority.zip", 4*1024*1024)
	if err != nil {
		return nil, err
	}
	digest := func(raw []byte) string {
		hash := sha256.Sum256(raw)
		return hex.EncodeToString(hash[:])
	}
	if digest(archive) != spec.AuthoritySHA256 {
		return nil, errors.New("PAL production authority digest mismatch")
	}
	bundle, ready := &ProductionBundle{}, false
	defer func() {
		if !ready {
			bundle.Close(context.Background())
		}
	}()
	for _, id := range ids {
		program, err := readProductionFile(files, "cores/"+id+".wasm", 16*1024*1024)
		if err != nil {
			return nil, err
		}
		if digest(program) != id {
			return nil, errors.New("PAL production core digest mismatch")
		}
		runtime, err := engine.NewAuthorityRuntime(ctx, program, archive)
		if err != nil {
			return nil, err
		}
		bundle.runtimes = append(bundle.runtimes, runtime)
		identity := runtime.Identity()
		if identity.BuildID != id || identity.ResourceID != spec.ResourceID || identity.Profile != "original-98" || identity.Purpose != "campaign" {
			return nil, errors.New("PAL production bundle requires exact original-profile campaign cores")
		}
	}
	catalog, err := engine.NewRuntimeCatalog(bundle.runtimes[0], bundle.runtimes[1:]...)
	if err != nil {
		return nil, err
	}
	bundle.Identity = catalog.CurrentIdentity()
	bundle.Module = NewVersionedModule(catalog)
	ready = true
	return bundle, nil
}

func (bundle *ProductionBundle) Close(ctx context.Context) error {
	var result error
	for _, runtime := range bundle.runtimes {
		result = errors.Join(result, runtime.Close(ctx))
	}
	return result
}
