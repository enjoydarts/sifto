package repository

import (
	"context"

	"github.com/enjoydarts/sifto/api/internal/model"
)

func (r *DigestRepo) loadDigestLLMRefs(ctx context.Context, digestID string, detail *model.DigestDetail) {
	if llm, err := loadLatestDigestLLMUsage(ctx, r.db, digestID, "digest"); err == nil {
		detail.DigestLLM = llm
	}
	if llm, err := loadLatestDigestLLMUsage(ctx, r.db, digestID, "digest_cluster_draft"); err == nil {
		detail.ClusterDraftLLM = llm
	}
}
