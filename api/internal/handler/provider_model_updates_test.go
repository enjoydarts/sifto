package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/enjoydarts/sifto/api/internal/middleware"
	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/enjoydarts/sifto/api/internal/service"
)

type fakeProviderModelUpdateStore struct {
	lastProviders []string
	lastQuery     string
	lastLimit     int
	lastOffset    int
	items         []model.ProviderModelSnapshotEntry
	providers     []string
	total         int
}

type fakeProviderModelSnapshotSyncer struct {
	calls int
	err   error
}

func (f *fakeProviderModelUpdateStore) ListRecent(_ context.Context, _ time.Time, _ int) ([]model.ProviderModelChangeEvent, error) {
	return nil, nil
}

func (f *fakeProviderModelUpdateStore) ListSnapshotEntries(_ context.Context, providers []string, query string, limit, offset int) ([]model.ProviderModelSnapshotEntry, int, error) {
	f.lastProviders = append([]string{}, providers...)
	f.lastQuery = query
	f.lastLimit = limit
	f.lastOffset = offset
	return f.items, f.total, nil
}

func (f *fakeProviderModelUpdateStore) ListSnapshotProviders(_ context.Context) ([]string, error) {
	return append([]string{}, f.providers...), nil
}

func (f *fakeProviderModelSnapshotSyncer) SyncCommonProviders(_ context.Context, _ string) (*service.ProviderModelSnapshotSyncSummary, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return &service.ProviderModelSnapshotSyncSummary{Providers: 2, Changes: 3}, nil
}

type fakeAdminUserStore struct {
	user *model.User
	err  error
}

func TestPoeSyncRejectsOrdinaryUserBeforeProviderAccess(t *testing.T) {
	h := &PoeModelsHandler{users: &fakeAdminUserStore{user: &model.User{Email: "reader@example.com"}}, auth: service.NewPromptAdminAuthService([]string{"admin@example.com"})}
	r := httptest.NewRequest(http.MethodPost, "/api/poe-models/sync", nil)
	r = r.WithContext(context.WithValue(r.Context(), middleware.UserIDKey, "reader"))
	r.Header.Set("X-User-Email", "admin@example.com")
	w := httptest.NewRecorder()
	h.Sync(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status=%d, want 403", w.Code)
	}
}

func (f *fakeAdminUserStore) GetByID(_ context.Context, _ string) (*model.User, error) {
	return f.user, f.err
}

func TestProviderModelUpdateSyncAuthorization(t *testing.T) {
	for _, tc := range []struct {
		name       string
		userID     string
		user       *model.User
		lookupErr  error
		emails     []string
		syncErr    error
		wantStatus int
		wantCalls  int
	}{
		{name: "admin", userID: "admin", user: &model.User{Email: "ADMIN@example.com"}, emails: []string{"admin@example.com"}, wantStatus: 200, wantCalls: 1},
		{name: "ordinary user", userID: "reader", user: &model.User{Email: "reader@example.com"}, emails: []string{"admin@example.com"}, wantStatus: 403},
		{name: "no configured admin", userID: "admin", user: &model.User{Email: "admin@example.com"}, wantStatus: 403},
		{name: "no authentication", user: &model.User{Email: "admin@example.com"}, emails: []string{"admin@example.com"}, wantStatus: 403},
		{name: "lookup failure", userID: "admin", lookupErr: errors.New("database unavailable"), emails: []string{"admin@example.com"}, wantStatus: 403},
		{name: "missing user", userID: "admin", emails: []string{"admin@example.com"}, wantStatus: 403},
		{name: "safe provider error", userID: "admin", user: &model.User{Email: "admin@example.com"}, emails: []string{"admin@example.com"}, syncErr: errors.New("secret-provider-response"), wantStatus: 502, wantCalls: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			syncer := &fakeProviderModelSnapshotSyncer{err: tc.syncErr}
			h := &ProviderModelUpdateHandler{syncer: syncer, users: &fakeAdminUserStore{user: tc.user, err: tc.lookupErr}, auth: service.NewPromptAdminAuthService(tc.emails)}
			req := httptest.NewRequest(http.MethodPost, "/api/provider-model-snapshots/sync", nil)
			req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, tc.userID))
			rec := httptest.NewRecorder()
			h.SyncSnapshots(rec, req)
			if rec.Code != tc.wantStatus || syncer.calls != tc.wantCalls {
				t.Fatalf("status=%d calls=%d, want %d and %d", rec.Code, syncer.calls, tc.wantStatus, tc.wantCalls)
			}
			if strings.Contains(rec.Body.String(), "secret-provider-response") {
				t.Fatal("provider response leaked")
			}
		})
	}
}

func TestProviderModelUpdateHandlerListSnapshots(t *testing.T) {
	store := &fakeProviderModelUpdateStore{
		items: []model.ProviderModelSnapshotEntry{
			{
				Provider:  "openai",
				ModelID:   "gpt-5",
				FetchedAt: time.Date(2026, 3, 28, 10, 0, 0, 0, time.UTC),
				Status:    "ok",
			},
		},
		providers: []string{"anthropic", "openai", "groq"},
		total:     1,
	}
	h := &ProviderModelUpdateHandler{repo: store}
	req := httptest.NewRequest(http.MethodGet, "/api/provider-model-snapshots?provider=openai,groq&q=gpt&limit=25&offset=50", nil)
	rec := httptest.NewRecorder()

	h.ListSnapshots(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", rec.Code, rec.Body.String())
	}
	if got, want := len(store.lastProviders), 2; got != want {
		t.Fatalf("providers len = %d, want %d", got, want)
	}
	if got, want := store.lastProviders[0], "openai"; got != want {
		t.Fatalf("provider[0] = %q, want %q", got, want)
	}
	if got, want := store.lastProviders[1], "groq"; got != want {
		t.Fatalf("provider[1] = %q, want %q", got, want)
	}
	if got, want := store.lastQuery, "gpt"; got != want {
		t.Fatalf("query = %q, want %q", got, want)
	}
	if got, want := store.lastLimit, 25; got != want {
		t.Fatalf("limit = %d, want %d", got, want)
	}
	if got, want := store.lastOffset, 50; got != want {
		t.Fatalf("offset = %d, want %d", got, want)
	}

	var resp model.ProviderModelSnapshotList
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got, want := resp.Total, 1; got != want {
		t.Fatalf("total = %d, want %d", got, want)
	}
	if got, want := len(resp.Providers), 3; got != want {
		t.Fatalf("providers len = %d, want %d", got, want)
	}
	if got, want := len(resp.Items), 1; got != want {
		t.Fatalf("items len = %d, want %d", got, want)
	}
	if got, want := resp.Items[0].ModelID, "gpt-5"; got != want {
		t.Fatalf("model_id = %q, want %q", got, want)
	}
}

func TestProviderModelUpdateHandlerListSnapshotsRejectsInvalidLimit(t *testing.T) {
	h := &ProviderModelUpdateHandler{repo: &fakeProviderModelUpdateStore{}}
	req := httptest.NewRequest(http.MethodGet, "/api/provider-model-snapshots?limit=999", nil)
	rec := httptest.NewRecorder()

	h.ListSnapshots(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestProviderModelUpdateHandlerSyncSnapshots(t *testing.T) {
	syncer := &fakeProviderModelSnapshotSyncer{}
	h := &ProviderModelUpdateHandler{repo: &fakeProviderModelUpdateStore{}, syncer: syncer}
	req := httptest.NewRequest(http.MethodPost, "/api/provider-model-snapshots/sync", nil)
	rec := httptest.NewRecorder()

	h.SyncSnapshots(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 body=%s", rec.Code, rec.Body.String())
	}
	if syncer.calls != 0 {
		t.Fatalf("syncer calls = %d, want 0", syncer.calls)
	}
}

func TestProviderModelUpdateSyncRejectsSpoofedAdminEmail(t *testing.T) {
	syncer := &fakeProviderModelSnapshotSyncer{}
	h := &ProviderModelUpdateHandler{syncer: syncer}
	req := httptest.NewRequest(http.MethodPost, "/api/provider-model-snapshots/sync", nil)
	req.Header.Set("X-Internal-User-Email", "admin@example.com")
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "ordinary-user"))
	rec := httptest.NewRecorder()
	h.SyncSnapshots(rec, req)
	if rec.Code != http.StatusForbidden || syncer.calls != 0 {
		t.Fatalf("status=%d sync calls=%d, want 403 and 0", rec.Code, syncer.calls)
	}
}
