package handler

import (
	"testing"

	"github.com/enjoydarts/sifto/api/internal/repository"
)

func TestValidateCreateItemBulkJobRequest(t *testing.T) {
	var body createItemBulkJobRequest
	body.Action = " retry_from_facts "
	body.Filters.Status = " pending "
	body.Filters.SourceID = " source-1 "
	body.Filters.Topic = " ai "
	body.Filters.Genre = " engineering "

	action, filters, message := validateCreateItemBulkJobRequest(body)
	if message != "" {
		t.Fatalf("validateCreateItemBulkJobRequest() message = %q, want empty", message)
	}
	if action != repository.ItemBulkJobActionRetryFromFacts {
		t.Fatalf("action = %q, want retry_from_facts", action)
	}
	if filters.Status != "pending" || filters.SourceID != "source-1" || filters.Topic != "ai" || filters.Genre != "engineering" {
		t.Fatalf("filters = %#v, want trimmed filters", filters)
	}
}

func TestValidateCreateItemBulkJobRequestRejectsSearch(t *testing.T) {
	var body createItemBulkJobRequest
	body.Action = "retry"
	body.Filters.Status = "pending"
	body.Filters.Query = "deepseek"

	_, _, message := validateCreateItemBulkJobRequest(body)
	if message != "search bulk jobs are not supported" {
		t.Fatalf("message = %q, want search rejection", message)
	}
}

func TestValidateCreateItemBulkJobRequestRejectsNonPending(t *testing.T) {
	var body createItemBulkJobRequest
	body.Action = "retry"
	body.Filters.Status = "summarized"

	_, _, message := validateCreateItemBulkJobRequest(body)
	if message != "status must be pending" {
		t.Fatalf("message = %q, want pending rejection", message)
	}
}
