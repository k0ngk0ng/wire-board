//go:build !race

package platformtest

import "testing"

func TestCursorHTTPWSReplaySizesAndReconnect(t *testing.T) {
	runCursorTransportScenario(t, true)
}
