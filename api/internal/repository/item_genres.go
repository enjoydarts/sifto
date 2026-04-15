package repository

import (
	"context"
	"strings"
)

const uncategorizedGenre = "uncategorized"

func isUncategorizedGenreValue(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case uncategorizedGenre, "untagged":
		return true
	default:
		return false
	}
}

func normalizeGenreValue(value *string) *string {
	if value == nil {
		return nil
	}
	normalized := strings.ToLower(strings.TrimSpace(*value))
	if normalized == "" {
		return nil
	}
	if isUncategorizedGenreValue(normalized) {
		normalized = uncategorizedGenre
	}
	return &normalized
}

func effectiveGenreExpr(itemAlias, summaryAlias string) string {
	return "COALESCE(NULLIF(BTRIM(" + itemAlias + ".user_genre), ''), NULLIF(BTRIM(" + summaryAlias + ".genre), ''), '" + uncategorizedGenre + "')"
}

func appendItemGenreFilter(query string, args []any, genre *string, itemAlias, summaryAlias string) (string, []any) {
	normalized := normalizeGenreValue(genre)
	if normalized == nil {
		return query, args
	}
	args = append(args, *normalized)
	return query + ` AND ` + effectiveGenreExpr(itemAlias, summaryAlias) + ` = $` + itoa(len(args)), args
}

func (r *ItemRepo) UpdateUserGenre(ctx context.Context, userID, itemID string, genre *string) error {
	if err := r.ensureOwned(ctx, userID, itemID); err != nil {
		return err
	}
	normalized := normalizeGenreValue(genre)
	_, err := r.db.Exec(ctx, `
		UPDATE items
		SET user_genre = $2,
		    updated_at = NOW()
		WHERE id = $1`,
		itemID, normalized)
	return err
}
