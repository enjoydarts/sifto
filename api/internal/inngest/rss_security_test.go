package inngest

import (
	"strings"
	"testing"
)

func TestRSSCacheHeadersCannotPersistOversizedOrInjectedValues(t *testing.T) {
	previous := "valid-etag"
	for _, value := range []string{strings.Repeat("x", 1025), "etag\r\nInjected: value", "etag\x00value"} {
		if got := headerValueOrPrevious(value, &previous); got != nil {
			t.Fatalf("unsafe header persisted: %q", *got)
		}
		if got := headerValueOrPrevious("", &value); got != nil {
			t.Fatal("unsafe previous header retained")
		}
	}
	if got := headerValueOrPrevious("", &previous); got == nil || *got != previous {
		t.Fatal("valid conditional request header lost")
	}
}
