package service

import (
	"context"
	"testing"
	"time"
)

type memoryJSONCache struct {
	data map[string]any
}

func (m *memoryJSONCache) GetJSON(_ context.Context, key string, dst any) (bool, error) {
	value, ok := m.data[key]
	if !ok {
		return false, nil
	}
	switch target := dst.(type) {
	case *modelSplitUsageCounts:
		*target = value.(modelSplitUsageCounts)
	default:
		return false, nil
	}
	return true, nil
}

func (m *memoryJSONCache) SetJSON(_ context.Context, key string, value any, _ time.Duration) error {
	if m.data == nil {
		m.data = map[string]any{}
	}
	m.data[key] = value
	return nil
}

func (m *memoryJSONCache) GetVersion(context.Context, string) (int64, error)  { return 0, nil }
func (m *memoryJSONCache) BumpVersion(context.Context, string) (int64, error) { return 0, nil }
func (m *memoryJSONCache) DeleteByPrefix(context.Context, string, int64) (int64, error) {
	return 0, nil
}
func (m *memoryJSONCache) Ping(context.Context) error { return nil }
func (m *memoryJSONCache) IncrMetric(context.Context, string, string, int64, time.Time, time.Duration) error {
	return nil
}
func (m *memoryJSONCache) SumMetrics(context.Context, string, time.Time, time.Time) (map[string]int64, error) {
	return map[string]int64{}, nil
}

func TestResolveSplitPrimaryModelByUsage(t *testing.T) {
	primary := strptr("primary")
	secondary := strptr("secondary")

	tests := []struct {
		name   string
		rate   int
		counts modelSplitUsageCounts
		want   *string
	}{
		{name: "no history starts with secondary", rate: 33, counts: modelSplitUsageCounts{}, want: secondary},
		{name: "under target prefers secondary", rate: 33, counts: modelSplitUsageCounts{PrimaryCount: 2, SecondaryCount: 0}, want: secondary},
		{name: "at target returns primary", rate: 33, counts: modelSplitUsageCounts{PrimaryCount: 2, SecondaryCount: 1}, want: primary},
		{name: "over target returns primary", rate: 33, counts: modelSplitUsageCounts{PrimaryCount: 1, SecondaryCount: 1}, want: primary},
		{name: "zero rate returns primary", rate: 0, counts: modelSplitUsageCounts{}, want: primary},
		{name: "full rate returns secondary", rate: 100, counts: modelSplitUsageCounts{PrimaryCount: 10}, want: secondary},
	}

	for _, tt := range tests {
		if got := resolveSplitPrimaryModelByUsage(primary, secondary, tt.rate, tt.counts); got == nil || *got != *tt.want {
			t.Fatalf("%s: got %v, want %v", tt.name, modelSplitStringValue(got), modelSplitStringValue(tt.want))
		}
	}
}

func TestChooseSplitPrimaryModelWithUsageAndRecord(t *testing.T) {
	cache := &memoryJSONCache{data: map[string]any{}}
	ctx := context.Background()
	primary := strptr("primary")
	secondary := strptr("secondary")

	first := ChooseSplitPrimaryModelWithUsage(ctx, cache, "u1", "facts", primary, secondary, 33)
	if first == nil || *first != "secondary" {
		t.Fatalf("first choose = %v, want secondary", modelSplitStringValue(first))
	}
	RecordSplitPrimaryModelUsage(ctx, cache, "u1", "facts", primary, secondary, first)

	second := ChooseSplitPrimaryModelWithUsage(ctx, cache, "u1", "facts", primary, secondary, 33)
	if second == nil || *second != "primary" {
		t.Fatalf("second choose = %v, want primary", modelSplitStringValue(second))
	}
	RecordSplitPrimaryModelUsage(ctx, cache, "u1", "facts", primary, secondary, second)

	third := ChooseSplitPrimaryModelWithUsage(ctx, cache, "u1", "facts", primary, secondary, 33)
	if third == nil || *third != "primary" {
		t.Fatalf("third choose = %v, want primary", modelSplitStringValue(third))
	}
}

func modelSplitStringValue(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
