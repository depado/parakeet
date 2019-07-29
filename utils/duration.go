package utils

import (
	"fmt"
	"math"
	"time"
)

// FormatDuration will return a user-friendly string based on a time duration
func FormatDuration(t time.Duration) string {
	h := int64(math.Mod(t.Hours(), 24))
	m := int64(math.Mod(t.Minutes(), 60))
	s := int64(math.Mod(t.Seconds(), 60))
	return fmt.Sprintf("%02d:%02d:%02d", int(h), int(m), int(s))
}
