package repository

import (
	"encoding/json"

	"github.com/minoru-kitayama/sifto/api/internal/model"
)

type scoreBreakdownScanner struct {
	dst **model.ItemSummaryScoreBreakdown
}

type jsonStringArrayScanner struct {
	dst *[]string
}

func (s scoreBreakdownScanner) Scan(src any) error {
	if s.dst == nil {
		return nil
	}
	if src == nil {
		*s.dst = nil
		return nil
	}
	var b []byte
	switch v := src.(type) {
	case []byte:
		b = v
	case string:
		b = []byte(v)
	default:
		*s.dst = nil
		return nil
	}
	if len(b) == 0 {
		*s.dst = nil
		return nil
	}
	var raw map[string]any
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	toPtr := func(k string) *float64 {
		v, ok := raw[k]
		if !ok || v == nil {
			return nil
		}
		switch x := v.(type) {
		case float64:
			return &x
		case int:
			f := float64(x)
			return &f
		default:
			return nil
		}
	}
	*s.dst = &model.ItemSummaryScoreBreakdown{
		Importance:    toPtr("importance"),
		Novelty:       toPtr("novelty"),
		Actionability: toPtr("actionability"),
		Reliability:   toPtr("reliability"),
		Relevance:     toPtr("relevance"),
	}
	return nil
}

func (s jsonStringArrayScanner) Scan(src any) error {
	if s.dst == nil {
		return nil
	}
	if src == nil {
		*s.dst = nil
		return nil
	}
	var b []byte
	switch v := src.(type) {
	case []byte:
		b = v
	case string:
		b = []byte(v)
	default:
		*s.dst = nil
		return nil
	}
	if len(b) == 0 {
		*s.dst = nil
		return nil
	}
	var out []string
	if err := json.Unmarshal(b, &out); err != nil {
		return err
	}
	*s.dst = out
	return nil
}
