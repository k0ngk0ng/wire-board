package main

import (
	"strings"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/engine"
)

// These acceptance and negative controls are written after original CPU
// fixtures but before the verifier implementation. They are not self-derived
// animation expectations: all 9,464 outputs came from the original program.
func TestVerifyAllCatalogFramesAgainstOriginalCPU(t *testing.T) {
	catalog, oracle, err := loadInputs("../../data/animation-catalog.json.gz", "../../data/animation-catalog-oracle.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	report, err := verifyCatalog(catalog, oracle)
	if err != nil {
		t.Fatal(err)
	}
	if !report.Pass || report.Characters != 11 || report.Frames != 4732 || report.OriginalCPUCalls != 9464 || report.VerifiedFrames != report.Frames {
		t.Fatalf("incomplete catalog verification: %+v", report)
	}
}

func TestCatalogVerifierRejectsIndependentCorruption(t *testing.T) {
	catalog, oracle, err := loadInputs("../../data/animation-catalog.json.gz", "../../data/animation-catalog-oracle.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	library, err := buildLibrary(catalog)
	if err != nil {
		t.Fatal(err)
	}
	first := oracle.Calls[0]
	if err := verifyCall(library, first); err != nil {
		t.Fatal(err)
	}
	frame := library[first.Key]
	frame.Duration++
	library[first.Key] = frame
	if err := verifyCall(library, first); err == nil {
		t.Fatal("changed duration was accepted against original CPU output")
	}
	delete(library, first.Key)
	if err := verifyCall(library, first); err == nil {
		t.Fatal("missing immutable frame was accepted")
	}
	if _, err := verifyCatalog(catalog, originalCalls{CatalogSHA: oracle.CatalogSHA}); err == nil {
		t.Fatal("empty CPU coverage was accepted")
	}
	catalog.Frames = append(catalog.Frames, catalog.Frames[0])
	if _, err := buildLibrary(catalog); err == nil {
		t.Fatal("duplicate frame identity was accepted")
	}
	_ = engine.AnimationFrameKey{} // Ensure the verifier uses the actual game API.
}

func TestCatalogVerifierRequiresFullStateAndUniqueCoverage(t *testing.T) {
	catalog, oracle, err := loadInputs("../../data/animation-catalog.json.gz", "../../data/animation-catalog-oracle.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	library, err := buildLibrary(catalog)
	if err != nil {
		t.Fatal(err)
	}
	first := oracle.Calls[0]
	for _, mutation := range []func(*originalCall){
		func(c *originalCall) { c.After.ActorBytes = "01" + c.After.ActorBytes[2:] },
		func(c *originalCall) { c.After.Buffers = "01" + c.After.Buffers[2:] },
		func(c *originalCall) { c.After.Offsets[0]++ },
		func(c *originalCall) { c.After.SoundQueue[0]++ },
		func(c *originalCall) { c.After.PendingSound++ },
		func(c *originalCall) { c.Before.Actor.WorldX++ },
		func(c *originalCall) { c.Exit = "0x1234" },
		func(c *originalCall) { c.Key.Index++ },
		func(c *originalCall) { c.After.ActorBytes = strings.Repeat("x", 512) },
	} {
		changed := first
		mutation(&changed)
		if err := verifyCall(library, changed); err == nil {
			t.Fatal("corrupted complete original snapshot was accepted")
		}
	}
	changed := oracle
	changed.Calls = append(append([]originalCall(nil), oracle.Calls...), first)
	if _, err := verifyCatalog(catalog, changed); err == nil {
		t.Fatal("duplicate CPU coverage was accepted")
	}
	changed.Calls = oracle.Calls[1:]
	if _, err := verifyCatalog(catalog, changed); err == nil {
		t.Fatal("missing facing/timer/capacity scenario was accepted")
	}
	changed = oracle
	changed.CatalogSHA = "changed"
	if _, err := verifyCatalog(catalog, changed); err == nil {
		t.Fatal("unbound original CPU fixture was accepted")
	}
}
