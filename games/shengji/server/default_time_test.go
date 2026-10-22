package shengji

// Before changing defaults: frontend/server disagree; a custom/saved 30-second
// option is rewritten; a new round is still scheduled for only 30 seconds.
import (
	"encoding/json"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"testing"
	"time"
)

func TestDefaultMinutePreservesExplicitTime(t *testing.T) {
	for _, tc := range []struct {
		raw     string
		seconds int
	}{{"{}", 60}, {`{"turnSeconds":30}`, 30}} {
		raw, e := Module().Configure(4, json.RawMessage(tc.raw))
		if e != nil {
			t.Fatal(e)
		}
		var o Options
		if e = json.Unmarshal(raw, &o); e != nil {
			t.Fatal(e)
		}
		if o.TurnSeconds != tc.seconds {
			t.Fatalf("got %d want %d", o.TurnSeconds, tc.seconds)
		}
		ctx := gameapi.Context{Players: 4, Now: time.Unix(1000, 0)}
		m, e := Module().Start(ctx, raw)
		if e != nil {
			t.Fatal(e)
		}
		if m.Deadline-ctx.Now.UnixMilli() != int64(tc.seconds)*1000 {
			t.Fatal("wrong initial deadline", m.Deadline)
		}
	}
}
