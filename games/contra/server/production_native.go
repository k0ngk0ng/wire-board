//go:build !js

package contra

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"

	engine "github.com/k0ngk0ng/wire-board/games/contra/engine"
	runtime "github.com/k0ngk0ng/wire-board/games/contra/runtime"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

const originalVisualROM = "6ba53139fa88b8de1ae527c438bda6f1541d1ee7df26d63dec5164a32d166bfe"

func hashProductionBytes(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func decodeProductionVisuals(packed []byte, expected string) (*BotVisualResources, error) {
	if !validSHA(expected) || len(packed) > 256<<10 || hashProductionBytes(packed) != expected {
		return nil, errors.New("production visual archive identity differs")
	}
	input := bytes.NewReader(packed)
	reader, err := gzip.NewReader(input)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	reader.Multistream(false)
	raw, err := io.ReadAll(io.LimitReader(reader, (1<<20)+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > 1<<20 || input.Len() != 0 {
		return nil, errors.New("oversized or trailing production visual data")
	}
	var data struct {
		Schema    int                             `json:"schema"`
		ROMSHA256 string                          `json:"romSHA256"`
		CatalogID string                          `json:"catalogID"`
		Catalog   engine.BotVisualCatalogManifest `json:"catalog"`
		RGB       []byte                          `json:"rgb"`
		Pages     map[uint8][]byte                `json:"pages"`
	}
	if err = strict(raw, &data); err != nil {
		return nil, err
	}
	if data.Schema != 1 || data.ROMSHA256 != originalVisualROM {
		return nil, errors.New("unsupported production visual source")
	}
	resources, err := NewBotVisualResources(data.Catalog, data.RGB, data.Pages)
	if err != nil {
		return nil, err
	}
	if data.CatalogID != resources.ID() {
		return nil, errors.New("production visual catalogue identity differs")
	}
	return resources, nil
}

// NewProductionModule performs all validation and visual-knowledge construction
// before registration. It uses only linked identities and native embedded data;
// no ROM, repository, test fixture, external service or runtime configuration is
// read. The returned callbacks capture only frozen memory and the pure observer.
// This constructor does not register the game or publish any client assets.
func NewProductionModule() (gameapi.Module, error) {
	if !validSHA(runtime.CompiledRulesBuildID) || !validSHA(runtime.CompiledRuntimeBuildID) {
		return gameapi.Module{}, errors.New("production Contra requires linked rule and runtime asset identities")
	}
	resources, err := decodeProductionVisuals(productionVisualArchive, productionVisualArchiveSHA256)
	if err != nil {
		return gameapi.Module{}, err
	}
	bundle, err := engine.BuiltinLevelBundle()
	if err != nil {
		return gameapi.Module{}, err
	}
	vision, err := engine.NewBuiltinBotVision(resources.Catalog(), bundle)
	if err != nil {
		return gameapi.Module{}, err
	}
	module, err := ModuleWithConfig(Config{RulesBuildID: runtime.CompiledRulesBuildID, RuntimeBuildID: runtime.CompiledRuntimeBuildID, Visuals: resources, ObserveBot: func(frame engine.BotVisualFrame) (engine.BotObservation, error) {
		return engine.ObserveBotFrame(frame, vision)
	}})
	if err != nil {
		return gameapi.Module{}, err
	}
	// Delivery currently supports one human or two cooperating humans. Keep
	// the experimental policy in development fixtures, outside public room
	// admission and autoplay. The platform enforces these omitted capabilities.
	module.BotAction, module.Reclaim, module.BotSearch = nil, nil, nil
	module.ReclaimOnCancel = false
	return module, module.Validate()
}

// ProductionVisualManifest exposes immutable identity metadata for a local asset
// builder to prove that browser pixels match the native observer. Byte payloads
// stay private to the embedded constructor; all returned maps are detached.
func ProductionVisualManifest() (engine.BotVisualCatalogManifest, string, error) {
	if _, err := decodeProductionVisuals(productionVisualArchive, productionVisualArchiveSHA256); err != nil {
		return engine.BotVisualCatalogManifest{}, "", err
	}
	reader, err := gzip.NewReader(bytes.NewReader(productionVisualArchive))
	if err != nil {
		return engine.BotVisualCatalogManifest{}, "", err
	}
	defer reader.Close()
	var data struct {
		Catalog   engine.BotVisualCatalogManifest
		CatalogID string
	}
	err = json.NewDecoder(reader).Decode(&data)
	return data.Catalog, data.CatalogID, err
}
