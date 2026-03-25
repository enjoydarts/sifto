package service

import "testing"

func TestNormalizePersonaMode(t *testing.T) {
	if got := NormalizePersonaMode(nil); got != PersonaModeFixed {
		t.Fatalf("NormalizePersonaMode(nil) = %q, want %q", got, PersonaModeFixed)
	}
	if got := NormalizePersonaMode(strptr(" random ")); got != PersonaModeRandom {
		t.Fatalf("NormalizePersonaMode(random) = %q, want %q", got, PersonaModeRandom)
	}
	if got := NormalizePersonaMode(strptr("unknown")); got != PersonaModeFixed {
		t.Fatalf("NormalizePersonaMode(unknown) = %q, want %q", got, PersonaModeFixed)
	}
}

func TestNormalizePersonaValue(t *testing.T) {
	if got := NormalizePersonaValue(" snark "); got != "snark" {
		t.Fatalf("NormalizePersonaValue(snark) = %q, want snark", got)
	}
	if got := NormalizePersonaValue("unknown"); got != "editor" {
		t.Fatalf("NormalizePersonaValue(unknown) = %q, want editor", got)
	}
}

func TestResolvePersonaFixed(t *testing.T) {
	if got := ResolvePersona(PersonaModeFixed, "analyst"); got != "analyst" {
		t.Fatalf("ResolvePersona(fixed, analyst) = %q, want analyst", got)
	}
}

func TestNavigatorPersonaKeysIncludesExpectedValues(t *testing.T) {
	got := NavigatorPersonaKeys()
	if len(got) != 6 {
		t.Fatalf("len(NavigatorPersonaKeys()) = %d, want 6", len(got))
	}
	if got[0] != "editor" || got[5] != "native" {
		t.Fatalf("NavigatorPersonaKeys() = %#v, want editor..native", got)
	}
}
