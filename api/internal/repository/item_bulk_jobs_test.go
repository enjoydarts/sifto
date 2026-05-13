package repository

import "testing"

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
