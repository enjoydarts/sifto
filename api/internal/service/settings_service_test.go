package service

import "testing"

func strptr(v string) *string { return &v }

func TestValidateCatalogModelForPurpose(t *testing.T) {
	tests := []struct {
		name    string
		model   *string
		purpose string
		wantErr bool
	}{
		{name: "nil allowed", model: nil, purpose: "summary", wantErr: false},
		{name: "valid summary model", model: strptr("gpt-5.4-mini"), purpose: "summary", wantErr: false},
		{name: "invalid purpose", model: strptr("text-embedding-3-small"), purpose: "summary", wantErr: true},
		{name: "unknown model", model: strptr("unknown-model"), purpose: "summary", wantErr: true},
	}
	for _, tt := range tests {
		err := validateCatalogModelForPurpose(tt.model, tt.purpose)
		if (err != nil) != tt.wantErr {
			t.Fatalf("%s: validateCatalogModelForPurpose(%v, %q) err=%v, wantErr=%v", tt.name, tt.model, tt.purpose, err, tt.wantErr)
		}
	}
}
