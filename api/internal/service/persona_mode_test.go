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
	if got := NormalizePersonaValue(" urban "); got != "urban" {
		t.Fatalf("NormalizePersonaValue(urban) = %q, want urban", got)
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
	if len(got) != 8 {
		t.Fatalf("len(NavigatorPersonaKeys()) = %d, want 8", len(got))
	}
	if got[0] != "editor" || got[6] != "junior" || got[7] != "urban" {
		t.Fatalf("NavigatorPersonaKeys() = %#v, want editor..urban", got)
	}
}

func TestAvailableRandomPersonas(t *testing.T) {
	got := availableRandomPersonas([]string{"editor", "editor", "hype", "analyst", "urban"})
	for _, blocked := range []string{"editor", "hype", "analyst"} {
		for _, persona := range got {
			if persona == blocked {
				t.Fatalf("availableRandomPersonas included blocked persona %q in %#v", blocked, got)
			}
		}
	}
	if len(got) != 5 {
		t.Fatalf("len(availableRandomPersonas()) = %d, want 5", len(got))
	}
}

func TestAvailableRandomPersonasUsesOnlyFirstThreeUniqueRecentPersonas(t *testing.T) {
	got := availableRandomPersonas([]string{"editor", "hype", "analyst", "concierge", "snark", "native", "junior", "urban"})
	for _, blocked := range []string{"editor", "hype", "analyst"} {
		for _, persona := range got {
			if persona == blocked {
				t.Fatalf("availableRandomPersonas included blocked persona %q in %#v", blocked, got)
			}
		}
	}
	if len(got) != 5 {
		t.Fatalf("len(availableRandomPersonas()) = %d, want 5", len(got))
	}
}

func TestResolvePersonaAvoidRecent(t *testing.T) {
	picker := func(candidates []string) (string, bool) {
		for _, persona := range candidates {
			if persona == "urban" {
				return persona, true
			}
		}
		return "", false
	}
	got := resolvePersonaWithPicker(PersonaModeRandom, "editor", []string{"editor", "hype", "analyst"}, picker)
	if got != "urban" {
		t.Fatalf("ResolvePersonaAvoidRecent(random) = %q, want %q", got, "urban")
	}
}
