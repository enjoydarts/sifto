package service

import (
	crand "crypto/rand"
	"math/big"
	"strings"
)

const (
	PersonaModeFixed  = "fixed"
	PersonaModeRandom = "random"
)

var supportedPersonaKeys = []string{"editor", "hype", "analyst", "concierge", "snark", "native", "junior", "urban"}

func NavigatorPersonaKeys() []string {
	return append([]string(nil), supportedPersonaKeys...)
}

func NormalizePersonaMode(v *string) string {
	if v == nil {
		return PersonaModeFixed
	}
	switch strings.TrimSpace(*v) {
	case PersonaModeRandom:
		return PersonaModeRandom
	default:
		return PersonaModeFixed
	}
}

func NormalizePersonaValue(v string) string {
	switch strings.TrimSpace(v) {
	case "editor", "hype", "analyst", "concierge", "snark", "native", "junior", "urban":
		return strings.TrimSpace(v)
	default:
		return "editor"
	}
}

func ResolvePersona(mode string, fixed string) string {
	if NormalizePersonaMode(&mode) != PersonaModeRandom {
		return NormalizePersonaValue(fixed)
	}
	if picked, ok := randomPersona(); ok {
		return picked
	}
	return NormalizePersonaValue(fixed)
}

func ResolvePersonaAvoidRecent(mode string, fixed string, recent []string) string {
	return resolvePersonaWithPicker(mode, fixed, recent, randomPersonaFromCandidates)
}

func resolvePersonaWithPicker(mode string, fixed string, recent []string, picker func([]string) (string, bool)) string {
	if NormalizePersonaMode(&mode) != PersonaModeRandom {
		return NormalizePersonaValue(fixed)
	}
	candidates := availableRandomPersonas(recent)
	if picked, ok := picker(candidates); ok {
		return NormalizePersonaValue(picked)
	}
	return NormalizePersonaValue(fixed)
}

func availableRandomPersonas(recent []string) []string {
	if len(supportedPersonaKeys) == 0 {
		return nil
	}
	blocked := make(map[string]struct{}, len(recent))
	for _, persona := range recent {
		normalized := NormalizePersonaValue(persona)
		if normalized == "" {
			continue
		}
		blocked[normalized] = struct{}{}
		if len(blocked) >= 3 {
			break
		}
	}
	out := make([]string, 0, len(supportedPersonaKeys))
	for _, persona := range supportedPersonaKeys {
		if _, ok := blocked[persona]; ok {
			continue
		}
		out = append(out, persona)
	}
	if len(out) == 0 {
		return append([]string(nil), supportedPersonaKeys...)
	}
	return out
}

func randomPersona() (string, bool) {
	return randomPersonaFromCandidates(supportedPersonaKeys)
}

func randomPersonaFromCandidates(candidates []string) (string, bool) {
	if len(candidates) == 0 {
		return "", false
	}
	n, err := crand.Int(crand.Reader, big.NewInt(int64(len(candidates))))
	if err != nil {
		return "", false
	}
	return candidates[int(n.Int64())], true
}
