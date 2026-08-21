package selfcheck

import "time"

// parseTime parses an RFC3339 timestamp and returns its epoch seconds; the
// selfcheck anchors the fake clock at a fixed point so deterministic scenarios
// don't depend on the wall clock. A malformed fixed base time is a programmer
// mistake, so it panics.
func parseTime(s string) int64 {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic("selfcheck: bad fixed time " + s + ": " + err.Error())
	}
	return t.Unix()
}
