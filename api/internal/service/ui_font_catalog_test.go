package service

import (
	"context"
	"testing"
)

func TestLoadUIFontCatalogIncludesDefaults(t *testing.T) {
	svc := NewUIFontCatalogService()
	catalog, err := svc.LoadCatalog(context.Background())
	if err != nil {
		t.Fatalf("LoadCatalog() error = %v", err)
	}
	if got := FindUIFontByKey(catalog, DefaultUIFontSansKey); got == nil || !got.SelectableForSans {
		t.Fatalf("default sans font missing or invalid: %v", got)
	}
	if got := FindUIFontByKey(catalog, DefaultUIFontSerifKey); got == nil || !got.SelectableForSerif {
		t.Fatalf("default serif font missing or invalid: %v", got)
	}
}

func TestValidateUIFontSelectionRejectsWrongCategory(t *testing.T) {
	svc := NewUIFontCatalogService()
	catalog, err := svc.LoadCatalog(context.Background())
	if err != nil {
		t.Fatalf("LoadCatalog() error = %v", err)
	}
	if err := ValidateUIFontSelection(catalog, "biz-udmincho", "biz-udgothic"); err == nil {
		t.Fatalf("ValidateUIFontSelection() error = nil, want category validation failure")
	}
}
