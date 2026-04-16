package repository

import (
	"context"
	"strings"
)

const uncategorizedGenre = "uncategorized"
const otherGenre = "other"

var itemGenreTaxonomyKeys = []string{
	"ai",
	"devtools",
	"security",
	"cloud",
	"data",
	"infra",
	"web",
	"mobile",
	"robotics",
	"semiconductor",
	"research",
	"product",
	"business",
	"funding",
	"regulation",
	"design",
	uncategorizedGenre,
	otherGenre,
}

var itemGenreTaxonomySet = func() map[string]struct{} {
	out := make(map[string]struct{}, len(itemGenreTaxonomyKeys))
	for _, key := range itemGenreTaxonomyKeys {
		out[key] = struct{}{}
	}
	return out
}()

var knownGenreRawValuesSQL = sqlQuotedValues(append(append([]string{}, itemGenreTaxonomyKeys...), "untagged"))
var itemGenreTaxonomyValuesSQL = sqlQuotedValues(itemGenreTaxonomyKeys)

func isUncategorizedGenreValue(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case uncategorizedGenre, "untagged":
		return true
	default:
		return false
	}
}

func isKnownGenreKey(value string) bool {
	_, ok := itemGenreTaxonomySet[value]
	return ok
}

func trimOptionalString(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func normalizeGenreInput(genre, otherLabel *string) (*string, *string) {
	trimmedLabel := trimOptionalString(otherLabel)
	if genre == nil {
		return nil, nil
	}
	raw := strings.TrimSpace(*genre)
	if raw == "" {
		return nil, nil
	}

	normalized := strings.ToLower(raw)
	switch {
	case isUncategorizedGenreValue(normalized):
		normalized = uncategorizedGenre
	case normalized == "agent":
		normalized = "ai"
	case isKnownGenreKey(normalized):
	default:
		normalized = otherGenre
		if trimmedLabel == nil {
			trimmedLabel = &raw
		}
	}

	if normalized != otherGenre {
		return &normalized, nil
	}
	return &normalized, trimmedLabel
}

func normalizeGenreValue(value *string) *string {
	normalized, _ := normalizeGenreInput(value, nil)
	return normalized
}

func sqlQuotedValues(values []string) string {
	quoted := make([]string, 0, len(values))
	for _, value := range values {
		quoted = append(quoted, "'"+strings.ReplaceAll(value, "'", "''")+"'")
	}
	return strings.Join(quoted, ", ")
}

func normalizedSourceGenreExpr(rawExpr string) string {
	trimmed := "NULLIF(BTRIM(" + rawExpr + "), '')"
	lowered := "LOWER(BTRIM(" + rawExpr + "))"
	return "CASE " +
		"WHEN " + trimmed + " IS NULL THEN NULL " +
		"WHEN " + lowered + " IN ('" + uncategorizedGenre + "', 'untagged') THEN '" + uncategorizedGenre + "' " +
		"WHEN " + lowered + " = 'agent' THEN 'ai' " +
		"WHEN " + lowered + " IN (" + itemGenreTaxonomyValuesSQL + ") THEN " + lowered + " " +
		"ELSE '" + otherGenre + "' END"
}

func sourceOtherGenreLabelExpr(rawGenreExpr, explicitLabelExpr string) string {
	trimmedGenre := "NULLIF(BTRIM(" + rawGenreExpr + "), '')"
	trimmedLabel := "NULLIF(BTRIM(" + explicitLabelExpr + "), '')"
	loweredGenre := "LOWER(BTRIM(" + rawGenreExpr + "))"
	return "CASE " +
		"WHEN " + normalizedSourceGenreExpr(rawGenreExpr) + " <> '" + otherGenre + "' THEN NULL " +
		"WHEN " + trimmedLabel + " IS NOT NULL THEN " + trimmedLabel + " " +
		"WHEN " + trimmedGenre + " IS NOT NULL AND " + loweredGenre + " NOT IN (" + knownGenreRawValuesSQL + ") THEN BTRIM(" + rawGenreExpr + ") " +
		"ELSE NULL END"
}

func effectiveGenreExpr(itemAlias, summaryAlias string) string {
	return "COALESCE(" +
		normalizedSourceGenreExpr(itemAlias+".user_genre") + ", " +
		normalizedSourceGenreExpr(summaryAlias+".genre") + ", " +
		"'" + uncategorizedGenre + "')"
}

func effectiveOtherGenreLabelExpr(itemAlias, summaryAlias string) string {
	itemGenre := normalizedSourceGenreExpr(itemAlias + ".user_genre")
	summaryGenre := normalizedSourceGenreExpr(summaryAlias + ".genre")
	return "CASE " +
		"WHEN " + itemGenre + " = '" + otherGenre + "' THEN " + sourceOtherGenreLabelExpr(itemAlias+".user_genre", itemAlias+".user_other_genre_label") + " " +
		"WHEN " + itemGenre + " IS NOT NULL THEN NULL " +
		"WHEN " + summaryGenre + " = '" + otherGenre + "' THEN " + sourceOtherGenreLabelExpr(summaryAlias+".genre", summaryAlias+".other_genre_label") + " " +
		"ELSE NULL END"
}

func appendItemGenreFilter(query string, args []any, genre *string, itemAlias, summaryAlias string) (string, []any) {
	normalized := normalizeGenreValue(genre)
	if normalized == nil {
		return query, args
	}
	args = append(args, *normalized)
	return query + ` AND ` + effectiveGenreExpr(itemAlias, summaryAlias) + ` = $` + itoa(len(args)), args
}

func (r *ItemRepo) UpdateUserGenre(ctx context.Context, userID, itemID string, genre, otherLabel *string) error {
	if err := r.ensureOwned(ctx, userID, itemID); err != nil {
		return err
	}
	normalizedGenre, normalizedLabel := normalizeGenreInput(genre, otherLabel)
	_, err := r.db.Exec(ctx, `
		UPDATE items
		SET user_genre = $2,
		    user_other_genre_label = $3,
		    updated_at = NOW()
		WHERE id = $1`,
		itemID, normalizedGenre, normalizedLabel)
	return err
}
