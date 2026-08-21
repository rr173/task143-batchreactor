// Package idlib generates opaque, prefix-tagged identifiers. IDs are a base62
// encoding of a high-resolution timestamp plus counter randomness; they are
// monotonically increasing within a process and effectively unique across
// short time spans, which is sufficient for single-node SQLite storage.
package idlib

import (
	"sync/atomic"
	"time"
)

var counter uint64

// New returns a new identifier tagged with the given short prefix (e.g. "rct",
// "rcp", "cmp", "cpg", "btch"). The body is base62 over (epoch_ms << 20 |
// counter) so lexical sort matches creation order within a millisecond.
func New(prefix string) string {
	ms := uint64(time.Now().UnixMilli())
	c := atomic.AddUint64(&counter, 1)
	id := (ms << 20) | (c & 0xFFFFF)
	return prefix + "_" + base62(id)
}

const alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

func base62(n uint64) string {
	if n == 0 {
		return "0"
	}
	var buf [16]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = alphabet[n%62]
		n /= 62
	}
	return string(buf[i:])
}
