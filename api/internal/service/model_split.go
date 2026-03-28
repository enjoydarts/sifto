package service

import (
	crand "crypto/rand"
	"math/big"
	"strings"
)

func normalizeModelSplitRatePercent(v *int) int {
	if v == nil {
		return 0
	}
	if *v < 0 {
		return 0
	}
	if *v > 100 {
		return 100
	}
	return *v
}

func ChooseSplitPrimaryModel(primary, secondary *string, secondaryRatePercent int) *string {
	return resolveSplitPrimaryModel(primary, secondary, secondaryRatePercent, randomIntn)
}

func resolveSplitPrimaryModel(primary, secondary *string, secondaryRatePercent int, draw func(int) int) *string {
	if secondary == nil || strings.TrimSpace(*secondary) == "" {
		return primary
	}
	rate := normalizeModelSplitRatePercent(&secondaryRatePercent)
	if rate <= 0 {
		return primary
	}
	if rate >= 100 {
		return secondary
	}
	if draw(100) < rate {
		return secondary
	}
	return primary
}

func randomIntn(max int) int {
	if max <= 1 {
		return 0
	}
	n, err := crand.Int(crand.Reader, big.NewInt(int64(max)))
	if err != nil {
		return 0
	}
	return int(n.Int64())
}
