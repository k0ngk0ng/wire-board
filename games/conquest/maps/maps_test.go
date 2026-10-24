package maps

import "testing"

// Failure cases: unknown IDs accepted; catalog exposes missing maps; loads share
// mutable province slices; IDs or start positions disappear during decoding.
func TestCatalogAndIndependentLoads(t *testing.T) {
	if _, err := Load("missing"); err == nil {
		t.Fatal("unknown map accepted")
	}
	if len(Catalog()) != 4 {
		t.Fatal("world, europe, USA and China must all be available")
	}
	for _, entry := range Catalog() {
		a, err := Load(entry.ID)
		if err != nil {
			t.Fatal(err)
		}
		if len(a.Provinces) != entry.ProvinceCount || a.SHA256 != entry.SHA256 || len(a.Starts["8"]) != 8 {
			t.Fatalf("invalid catalog entry: %+v", entry)
		}
		original := a.Provinces[0].Name
		a.Provinces[0].Name = "changed"
		b, _ := Load(entry.ID)
		if b.Provinces[0].Name != original {
			t.Fatal("map loads share mutable data")
		}
	}
}
