package pcmplayer

import (
	"bytes"
	"compress/gzip"
	_ "embed"
	"encoding/json"
	"io"
	"reflect"
	"testing"
)

//go:embed testdata/original-player-edges.json.gz
var originalPlayerEdges []byte

func TestOriginalRateAndMinimumDualEdges(t *testing.T) {
	reader, err := gzip.NewReader(bytes.NewReader(originalPlayerEdges))
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	raw, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	var body struct{ Cases []oracleCase }
	if err = json.Unmarshal(raw, &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Cases) != 5 {
		t.Fatal("incomplete declared edge set")
	}
	for _, c := range body.Cases {
		t.Run(c.Name, func(t *testing.T) {
			cfg := config(t, c)
			p, err := New(cfg)
			if err != nil {
				t.Fatal(err)
			}
			var result []oracleEvent
			for steps := 0; !p.Stopped(); steps++ {
				if steps > 40000 {
					t.Fatal("native edge failed to stop")
				}
				s := p.Snapshot()
				p, err = Restore(cfg, s)
				if err != nil {
					t.Fatal(err)
				}
				events, err := p.Next()
				if err != nil {
					t.Fatal(err)
				}
				result = append(result, normalized(events)...)
			}
			if !reflect.DeepEqual(result, c.Events) {
				for i := 0; i < len(result) && i < len(c.Events); i++ {
					if result[i] != c.Events[i] {
						t.Fatalf("original edge differs at %d: actual=%+v native=%+v", i, result[i], c.Events[i])
					}
				}
				t.Fatalf("original edge count actual=%d native=%d", len(result), len(c.Events))
			}
			s := p.Snapshot()
			if s.Time != c.Final.Time {
				t.Fatal("original edge completion differs", s.Time, c.Final.Time)
			}
			if s.Single != nil {
				if s.Single.DummyCursor != c.Final.Dummy || s.Single.Accumulator != c.Final.Accumulator[0] || s.Single.Consumed != c.Final.Consumed[0] || s.Single.Fetched != c.Final.Fetched[0] {
					t.Fatal("edge final state differs")
				}
			}
			t.Logf("native edge %d events at %d cycles", len(result), s.Time)
		})
	}
}
