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

func randomPersona() (string, bool) {
	if len(supportedPersonaKeys) == 0 {
		return "", false
	}
	n, err := crand.Int(crand.Reader, big.NewInt(int64(len(supportedPersonaKeys))))
	if err != nil {
		return "", false
	}
	return supportedPersonaKeys[int(n.Int64())], true
}
