package inngest

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/inngest/inngestgo"
)

// functionOptsWithStepConcurrency bridges the SDK's legacy array and its newer
// {step: [...]} representation. These remain step limits; function-lifetime
// limits are deliberately absent. Decode only concurrency so options such as
// throttle durations and checkpoint pointers never pass through JSON.
func functionOptsWithStepConcurrency(options inngestgo.FunctionOpts, limits []inngestgo.ConfigStepConcurrency) (inngestgo.FunctionOpts, error) {
	var selected inngestgo.FunctionOpts
	// Both nil and empty input mean no configured limits in either SDK.
	if len(limits) == 0 {
		options.Concurrency = selected.Concurrency
		return options, nil
	}
	legacy, err := json.Marshal(struct {
		Concurrency []inngestgo.ConfigStepConcurrency
	}{limits})
	if err != nil {
		return inngestgo.FunctionOpts{}, fmt.Errorf("encode step concurrency: %w", err)
	}
	type stepLimits struct {
		Step []inngestgo.ConfigStepConcurrency `json:"step"`
	}
	grouped, err := json.Marshal(struct {
		Concurrency stepLimits
	}{Concurrency: stepLimits{Step: limits}})
	if err != nil {
		return inngestgo.FunctionOpts{}, fmt.Errorf("encode grouped step concurrency: %w", err)
	}
	selected, err = decodeStepConcurrency(legacy, grouped)
	if err != nil {
		return inngestgo.FunctionOpts{}, fmt.Errorf("configure step concurrency: %w", err)
	}
	options.Concurrency = selected.Concurrency
	return options, nil
}

func decodeStepConcurrency(legacy, grouped []byte) (inngestgo.FunctionOpts, error) {
	var selected inngestgo.FunctionOpts
	decode := func(data []byte) error {
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.DisallowUnknownFields()
		return decoder.Decode(&selected)
	}
	legacyErr := decode(legacy)
	if legacyErr == nil {
		return selected, nil
	}
	var mismatch *json.UnmarshalTypeError
	if !errors.As(legacyErr, &mismatch) || mismatch.Field != "Concurrency" || mismatch.Value != "array" {
		return inngestgo.FunctionOpts{}, legacyErr
	}
	// Do not carry any partially decoded configuration into the second attempt.
	selected = inngestgo.FunctionOpts{}
	if err := decode(grouped); err != nil {
		return inngestgo.FunctionOpts{}, errors.Join(legacyErr, err)
	}
	return selected, nil
}
