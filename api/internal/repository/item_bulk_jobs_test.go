package repository

import (
	"testing"
	"time"
)

func TestNormalizeItemBulkJobAction(t *testing.T) {
	tests := []struct {
		name    string
		action  ItemBulkJobAction
		wantErr bool
	}{
		{name: "retry", action: ItemBulkJobActionRetry},
		{name: "retry from facts", action: ItemBulkJobActionRetryFromFacts},
		{name: "invalid", action: ItemBulkJobAction("delete"), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeItemBulkJobAction(tt.action)
			if tt.wantErr {
				if err != ErrInvalidState {
					t.Fatalf("normalizeItemBulkJobAction() error = %v, want ErrInvalidState", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("normalizeItemBulkJobAction() error = %v, want nil", err)
			}
			if got != tt.action {
				t.Fatalf("normalizeItemBulkJobAction() = %q, want %q", got, tt.action)
			}
		})
	}
}

func TestItemBulkJobCandidateEligible(t *testing.T) {
	now := time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)
	cutoff := now.Add(-30 * time.Minute)
	filters := ItemBulkJobFilters{Status: "pending", RetryEligibleBefore: &cutoff}

	tests := []struct {
		name      string
		candidate ItemBulkJobCandidate
		want      bool
	}{
		{name: "failed is eligible immediately", candidate: ItemBulkJobCandidate{Status: "failed", UpdatedAt: now}, want: true},
		{name: "stale new is eligible", candidate: ItemBulkJobCandidate{Status: "new", UpdatedAt: cutoff.Add(-time.Second)}, want: true},
		{name: "fresh fetched is excluded", candidate: ItemBulkJobCandidate{Status: "fetched", UpdatedAt: cutoff.Add(time.Second)}, want: false},
		{name: "completed item is excluded", candidate: ItemBulkJobCandidate{Status: "summarized", UpdatedAt: cutoff.Add(-time.Hour)}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ItemBulkJobCandidateEligible(tt.candidate, filters, now); got != tt.want {
				t.Fatalf("ItemBulkJobCandidateEligible() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateItemBulkJobFilters(t *testing.T) {
	tests := []struct {
		name    string
		filters ItemBulkJobFilters
		wantErr bool
	}{
		{name: "pending", filters: ItemBulkJobFilters{Status: "pending"}},
		{name: "non pending rejected", filters: ItemBulkJobFilters{Status: "summarized"}, wantErr: true},
		{name: "search rejected", filters: ItemBulkJobFilters{Status: "pending", Query: "llm"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateItemBulkJobFilters(tt.filters)
			if tt.wantErr && err != ErrInvalidState {
				t.Fatalf("validateItemBulkJobFilters() error = %v, want ErrInvalidState", err)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("validateItemBulkJobFilters() error = %v, want nil", err)
			}
		})
	}
}
