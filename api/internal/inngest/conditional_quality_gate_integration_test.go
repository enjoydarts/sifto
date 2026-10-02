package inngest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/enjoydarts/sifto/api/internal/repository"
	"github.com/enjoydarts/sifto/api/internal/service"
)

func TestFactsAndFaithfulnessD1GateEvaluateAndPersistCurrentCandidate(t *testing.T) {
	ctx := context.Background()
	pool, err := repository.NewPool(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	userID, sourceID, itemID := "00000000-0000-4000-8000-000000000186", "00000000-0000-4000-8000-000000000187", "00000000-0000-4000-8000-000000000188"
	_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id=$1`, userID)
	for _, query := range []string{
		`INSERT INTO users (id,email) VALUES ('` + userID + `','conditional-gate@example.com')`,
		`INSERT INTO sources (id,user_id,url,type) VALUES ('` + sourceID + `','` + userID + `','https://example.com/gate-feed','rss')`,
		`INSERT INTO items (id,source_id,url) VALUES ('` + itemID + `','` + sourceID + `','https://example.com/gate')`,
	} {
		if _, err := pool.Exec(ctx, query); err != nil {
			t.Fatal(err)
		}
	}
	defer pool.Exec(ctx, `DELETE FROM users WHERE id=$1`, userID)
	t.Setenv("USER_SECRET_ENCRYPTION_KEY", "conditional-gate-test")
	cipher := service.NewSecretCipher()
	encrypted, err := cipher.EncryptString("test-key")
	if err != nil {
		t.Fatal(err)
	}
	settings := repository.NewUserSettingsRepo(pool)
	if _, err := settings.SetJevAPIKey(ctx, userID, encrypted, "-key"); err != nil {
		t.Fatal(err)
	}
	if _, err := settings.SetD1APIKey(ctx, userID, encrypted, "-key"); err != nil {
		t.Fatal(err)
	}
	jevCatalog, err := service.LoadJevCatalog()
	if err != nil {
		t.Fatal(err)
	}
	d1Catalog, err := service.LoadD1Catalog()
	if err != nil {
		t.Fatal(err)
	}
	jevCalls, d1Calls := 0, 0
	var d1State map[string]any
	serve := func(soft bool) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var request struct {
				State     map[string]any `json:"state"`
				Questions map[string]any `json:"questions"`
			}
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Error(err)
			}
			model := "d1:free"
			if soft {
				jevCalls++
				model = "jev-latest"
			} else {
				d1Calls++
				d1State = request.State
			}
			answers := map[string]any{}
			for name, question := range request.Questions {
				if question.(map[string]any)["type"] == "noul" {
					answers[name] = map[string]any{"type": "noul", "noul": 0}
				} else {
					score := 4
					if soft && name == "coverage" {
						score = 2
					}
					answers[name] = map[string]any{"type": "score", "score": score, "confidence": 1}
				}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"model": model, "answers": answers, "usage": map[string]int{"input_tokens": 20}})
		}))
	}
	jevServer, d1Server := serve(true), serve(false)
	defer jevServer.Close()
	defer d1Server.Close()
	repo := repository.NewItemQualityEvaluationRepo(pool)
	deps := processItemDeps{keyProvider: service.NewUserKeyProvider(settings, cipher), qualityRepo: repo,
		jev: service.NewJevClient(jevServer.URL, jevServer.Client(), jevCatalog), jevCatalog: jevCatalog,
		d1: service.NewJevClient(d1Server.URL, d1Server.Client(), d1Catalog), d1Catalog: d1Catalog}
	data := processItemEventData{SourceID: sourceID, TriggerID: "current-gate-run"}
	for _, kind := range []string{"facts", "faithfulness"} {
		// An earlier accepted D1 evaluation must not approve the current candidate.
		if err := repo.Upsert(ctx, repository.ItemQualityEvaluationInput{ItemID: itemID, Kind: kind, Provider: "d1", GatePolicyVersion: "d1-shadow-v1", Decision: "accepted"}); err != nil {
			t.Fatal(err)
		}
		provider := ""
		if kind == "facts" {
			provider = executeFactsQualityGate(ctx, deps, data, itemID, &userID, 1, nil, "current source", []string{"current fact"})
		} else {
			provider = executeFaithfulnessQualityGate(ctx, deps, data, itemID, &userID, 1, nil, []string{"current fact"}, "current summary")
		}
		if provider != "d1" {
			t.Fatalf("%s provider=%q", kind, provider)
		}
		factsField := "facts"
		if kind == "facts" {
			factsField = "extracted_facts"
		}
		facts, ok := d1State[factsField].([]any)
		if !ok || len(facts) != 1 || facts[0] != "current fact" {
			t.Fatalf("%s candidate=%#v", kind, d1State)
		}
		if kind == "facts" && d1State["source_content"] != "current source" {
			t.Fatalf("source=%#v", d1State)
		}
		if kind == "faithfulness" && d1State["summary"] != "current summary" {
			t.Fatalf("summary=%#v", d1State)
		}
		saved, err := repo.LoadLatestByKindAndProvider(ctx, itemID, kind, "d1")
		if err != nil || saved == nil || saved.Decision != "accepted" || saved.AttemptIndex != 1 || saved.GatePolicyVersion != d1GatePolicyVersion {
			t.Fatalf("saved=%#v err=%v", saved, err)
		}
	}
	if jevCalls != 2 || d1Calls != 2 {
		t.Fatalf("jev=%d d1=%d", jevCalls, d1Calls)
	}
	// Run the real processing stages. Generation must still happen, while the
	// quality-check worker endpoints must never be called after a D1 rescue.
	if _, err := settings.SetAnthropicAPIKey(ctx, userID, encrypted, "-key"); err != nil {
		t.Fatal(err)
	}
	generationCalls, qualityCalls := 0, 0
	worker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/extract-facts":
			generationCalls++
			_ = json.NewEncoder(w).Encode(service.ExtractFactsResponse{Facts: []string{"current fact"}})
		case "/summarize":
			generationCalls++
			_ = json.NewEncoder(w).Encode(service.SummarizeResponse{Summary: "current summary", Topics: []string{}, Score: 0.5})
		default:
			qualityCalls++
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer worker.Close()
	t.Setenv("PYTHON_WORKER_URL", worker.URL)
	deps.worker, deps.itemRepo = service.NewWorkerClient(), repository.NewItemInngestRepo(pool)
	deps.itemViewRepo = repository.NewItemRepo(pool)
	facts, err := extractAndPersistFacts(ctx, deps, data, itemID, &userID, nil, nil, "current source")
	if err != nil || facts == nil || facts.Check.Verdict != "pass" || facts.Check.ShortComment != qualityGateComment("d1") {
		t.Fatalf("facts=%#v err=%v", facts, err)
	}
	summary, err := summarizeAndPersistItem(ctx, deps, data, itemID, &userID, nil, nil, "current source", facts.Facts.Facts)
	if err != nil || summary == nil || summary.Check.Verdict != "pass" || summary.Check.ShortComment != qualityGateComment("d1") {
		t.Fatalf("summary=%#v err=%v", summary, err)
	}
	if generationCalls != 2 || qualityCalls != 0 || d1Calls != 4 {
		t.Fatalf("generation=%d quality=%d D1=%d", generationCalls, qualityCalls, d1Calls)
	}
}
