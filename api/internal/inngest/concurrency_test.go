package inngest

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/inngest/inngest/pkg/enums"
	"github.com/inngest/inngestgo"
)

// The SDK changed the configuration container from a legacy array to a
// step/fn object. Compare the public wire meaning without depending on either
// Go container type, and require that no function-lifetime limits were added.
func stepConcurrencyForTest(t *testing.T, config inngestgo.FunctionOpts) []inngestgo.ConfigStepConcurrency {
	t.Helper()
	data, err := json.Marshal(config.Concurrency)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) == "null" {
		return nil
	}
	var limits []inngestgo.ConfigStepConcurrency
	if len(data) > 0 && data[0] == '[' {
		if err := json.Unmarshal(data, &limits); err != nil {
			t.Fatal(err)
		}
		return limits
	}
	var grouped struct {
		Step []inngestgo.ConfigStepConcurrency `json:"step"`
		Fn   []json.RawMessage                 `json:"fn"`
	}
	if err := json.Unmarshal(data, &grouped); err != nil {
		t.Fatal(err)
	}
	if len(grouped.Fn) != 0 {
		t.Fatalf("unexpected function-lifetime limits: %s", data)
	}
	return grouped.Step
}

func assertStepConcurrency(t *testing.T, config inngestgo.FunctionOpts, expected []inngestgo.ConfigStepConcurrency) {
	t.Helper()
	actual := stepConcurrencyForTest(t, config)
	if len(actual) != len(expected) {
		t.Fatalf("concurrency count = %d, want %d", len(actual), len(expected))
	}
	for index, want := range expected {
		got := actual[index]
		if got.Limit != want.Limit || got.Scope != want.Scope || got.Hash != want.Hash ||
			(got.Key == nil) != (want.Key == nil) || (got.Key != nil && *got.Key != *want.Key) {
			t.Fatalf("concurrency[%d] = %+v, want %+v", index, got, want)
		}
	}
}

func TestFunctionOptsWithStepConcurrencyPreservesOtherOptions(t *testing.T) {
	throttle := &inngestgo.ConfigThrottle{Limit: 30, Period: time.Minute, Burst: 6}
	retries := 4
	key := "event.data.item_id"
	limits := []inngestgo.ConfigStepConcurrency{{Limit: 5}, {Limit: 1, Key: &key, Scope: enums.ConcurrencyScopeFn}}
	options := inngestgo.FunctionOpts{ID: "fixture", Name: "Fixture", Throttle: throttle, Retries: &retries}
	actual, err := functionOptsWithStepConcurrency(options, limits)
	if err != nil {
		t.Fatal(err)
	}
	assertStepConcurrency(t, actual, limits)
	if actual.ID != options.ID || actual.Name != options.Name || actual.Throttle != throttle || actual.Retries != &retries {
		t.Fatalf("non-concurrency options changed: %+v", actual)
	}
	if len(stepConcurrencyForTest(t, options)) != 0 {
		t.Fatal("input options were mutated")
	}
}

func TestFunctionOptsWithStepConcurrencyNilAndEmptyClearLimits(t *testing.T) {
	for _, limits := range [][]inngestgo.ConfigStepConcurrency{nil, {}} {
		options, err := functionOptsWithStepConcurrency(inngestgo.FunctionOpts{ID: "fixture"}, []inngestgo.ConfigStepConcurrency{{Limit: 1}})
		if err != nil {
			t.Fatal(err)
		}
		actual, err := functionOptsWithStepConcurrency(options, limits)
		if err != nil {
			t.Fatal(err)
		}
		data, err := json.Marshal(actual.Concurrency)
		if err != nil || string(data) != "null" || actual.ID != "fixture" {
			t.Fatalf("empty concurrency = %s, error = %v", data, err)
		}
	}
}

func TestDecodeStepConcurrencyRefusesMalformedAndUnrelatedTypeErrors(t *testing.T) {
	for _, payload := range []string{
		`{`, `{"Concurrency":false}`, `{"Concurrency":[{"limit":"bad"}]}`,
		`{"Name":[],"Concurrency":[{"limit":1}]}`, `{"unexpected":1}`,
	} {
		t.Run(payload, func(t *testing.T) {
			if _, err := decodeStepConcurrency([]byte(payload), nil); err == nil {
				t.Fatal("invalid configuration accepted")
			}
		})
	}
}

func TestDecodeStepConcurrencyDoesNotFallbackForOtherTypeMismatches(t *testing.T) {
	grouped := []byte(`{"Concurrency":{"step":[{"limit":1}]}}`)
	for _, payload := range []string{
		`{"Concurrency":false}`, `{"Concurrency":"invalid"}`,
		`{"Name":[],"Concurrency":[{"limit":1}]}`,
	} {
		t.Run(payload, func(t *testing.T) {
			if _, err := decodeStepConcurrency([]byte(payload), grouped); err == nil {
				t.Fatal("unrelated type error bypassed by grouped fallback")
			}
		})
	}
}
