// Package bubbledata contains source-bound classic Bubbleparty metadata.
package bubbledata

import _ "embed"

// Classic2002 retains the complete decoded map and tile catalog. Callers must
// treat it as immutable; this package performs no network or filesystem reads.
//
//go:embed classic-2002.json
var Classic2002 []byte
