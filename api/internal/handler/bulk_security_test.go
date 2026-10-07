package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBulkRetriesRejectExcessiveItemCount(t *testing.T) {
	h := &ItemHandler{}
	for _, handler := range []http.HandlerFunc{h.RetryBulk, h.RetryFromFactsBulk} {
		body := `{"item_ids":[` + strings.Repeat(`"id",`, 100) + `"id"]}`
		w := httptest.NewRecorder()
		handler(w, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body)))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status=%d, want 400", w.Code)
		}
	}
}
