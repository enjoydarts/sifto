package service

import (
	"context"
	crand "crypto/rand"
	"fmt"
	"math/big"
	"strings"
	"time"
)

type modelSplitUsageCounts struct {
	PrimaryCount   int `json:"primary_count"`
	SecondaryCount int `json:"secondary_count"`
}

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

func ChooseSplitPrimaryModelWithUsage(ctx context.Context, cache JSONCache, userID, purpose string, primary, secondary *string, secondaryRatePercent int) *string {
	if !canUseModelSplitUsageCache(cache, userID, purpose) {
		return ChooseSplitPrimaryModel(primary, secondary, secondaryRatePercent)
	}
	if chosen, ok := chooseSplitPrimaryModelWithRedisSequence(ctx, cache, userID, purpose, primary, secondary, secondaryRatePercent); ok {
		return chosen
	}
	counts, ok := loadModelSplitUsageCounts(ctx, cache, userID, purpose, primary, secondary, secondaryRatePercent)
	if !ok {
		return ChooseSplitPrimaryModel(primary, secondary, secondaryRatePercent)
	}
	return resolveSplitPrimaryModelByUsage(primary, secondary, secondaryRatePercent, counts)
}

func RecordSplitPrimaryModelUsage(ctx context.Context, cache JSONCache, userID, purpose string, primary, secondary *string, secondaryRatePercent int, usedModel *string) {
	if !canUseModelSplitUsageCache(cache, userID, purpose) || usedModel == nil {
		return
	}
	if client, _ := RedisClientFromCache(cache); client != nil {
		return
	}
	used := strings.TrimSpace(*usedModel)
	primaryModel := ""
	if primary != nil {
		primaryModel = strings.TrimSpace(*primary)
	}
	secondaryModel := ""
	if secondary != nil {
		secondaryModel = strings.TrimSpace(*secondary)
	}
	if used == "" || (used != primaryModel && used != secondaryModel) {
		return
	}
	counts, ok := loadModelSplitUsageCounts(ctx, cache, userID, purpose, primary, secondary, secondaryRatePercent)
	if !ok {
		return
	}
	if used == secondaryModel {
		counts.SecondaryCount++
	} else {
		counts.PrimaryCount++
	}
	_ = cache.SetJSON(ctx, modelSplitUsageCacheKey(userID, purpose, primary, secondary, secondaryRatePercent), counts, 180*24*time.Hour)
}

func ResetSplitPrimaryModelUsage(ctx context.Context, cache JSONCache, userID, purpose string) error {
	if !canUseModelSplitUsageCache(cache, userID, purpose) {
		return nil
	}
	_, err := cache.DeleteByPrefix(ctx, modelSplitUsageCachePrefix(userID, purpose), 1000)
	return err
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

func resolveSplitPrimaryModelByUsage(primary, secondary *string, secondaryRatePercent int, counts modelSplitUsageCounts) *string {
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
	total := counts.PrimaryCount + counts.SecondaryCount
	if total <= 0 {
		return secondary
	}
	if counts.SecondaryCount*100 < rate*total {
		return secondary
	}
	return primary
}

func chooseSplitPrimaryModelWithRedisSequence(ctx context.Context, cache JSONCache, userID, purpose string, primary, secondary *string, secondaryRatePercent int) (*string, bool) {
	if secondary == nil || strings.TrimSpace(*secondary) == "" {
		return primary, true
	}
	rate := normalizeModelSplitRatePercent(&secondaryRatePercent)
	if rate <= 0 {
		return primary, true
	}
	if rate >= 100 {
		return secondary, true
	}
	client, prefix := RedisClientFromCache(cache)
	if client == nil {
		return nil, false
	}
	key := modelSplitUsageCacheKey(userID, purpose, primary, secondary, secondaryRatePercent) + ":seq"
	if prefix != "" {
		key = prefix + ":" + key
	}
	n, err := client.Incr(ctx, key).Result()
	if err != nil {
		return nil, false
	}
	_ = client.Expire(ctx, key, 180*24*time.Hour).Err()
	if shouldUseSecondaryForSequence(n, rate) {
		return secondary, true
	}
	return primary, true
}

func shouldUseSecondaryForSequence(n int64, rate int) bool {
	if n <= 0 || rate <= 0 {
		return false
	}
	if rate >= 100 {
		return true
	}
	return ((n*int64(rate))+99)/100 > (((n-1)*int64(rate))+99)/100
}

func modelSplitUsageCacheKey(userID, purpose string, primary, secondary *string, secondaryRatePercent int) string {
	return fmt.Sprintf(
		"%s%s:%s:%d",
		modelSplitUsageCachePrefix(userID, purpose),
		modelSplitCachePart(primary),
		modelSplitCachePart(secondary),
		normalizeModelSplitRatePercent(&secondaryRatePercent),
	)
}

func modelSplitUsageCachePrefix(userID, purpose string) string {
	return fmt.Sprintf("model_split_usage:%s:%s:", strings.TrimSpace(userID), strings.TrimSpace(purpose))
}

func modelSplitCachePart(model *string) string {
	if model == nil {
		return "_"
	}
	v := strings.TrimSpace(*model)
	if v == "" {
		return "_"
	}
	return strings.NewReplacer(":", "_", "/", "_", " ", "_").Replace(v)
}

func canUseModelSplitUsageCache(cache JSONCache, userID, purpose string) bool {
	if cache == nil || strings.TrimSpace(userID) == "" || strings.TrimSpace(purpose) == "" {
		return false
	}
	_, isNoop := cache.(NoopJSONCache)
	return !isNoop
}

func loadModelSplitUsageCounts(ctx context.Context, cache JSONCache, userID, purpose string, primary, secondary *string, secondaryRatePercent int) (modelSplitUsageCounts, bool) {
	var counts modelSplitUsageCounts
	ok, err := cache.GetJSON(ctx, modelSplitUsageCacheKey(userID, purpose, primary, secondary, secondaryRatePercent), &counts)
	if err != nil {
		return modelSplitUsageCounts{}, false
	}
	if !ok {
		return modelSplitUsageCounts{}, true
	}
	if counts.PrimaryCount < 0 {
		counts.PrimaryCount = 0
	}
	if counts.SecondaryCount < 0 {
		counts.SecondaryCount = 0
	}
	return counts, true
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
