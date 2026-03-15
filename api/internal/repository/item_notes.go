package repository

import (
	"context"

	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/google/uuid"
)

func (r *ItemRepo) GetByItem(ctx context.Context, userID, itemID string) (*model.ItemNote, []model.ItemHighlight, error) {
	if err := r.ensureOwned(ctx, userID, itemID); err != nil {
		return nil, nil, err
	}
	var note *model.ItemNote
	var noteRow model.ItemNote
	err := r.db.QueryRow(ctx, `
		SELECT id, user_id, item_id, content, tags, created_at, updated_at
		FROM item_notes
		WHERE user_id = $1 AND item_id = $2`,
		userID, itemID,
	).Scan(&noteRow.ID, &noteRow.UserID, &noteRow.ItemID, &noteRow.Content, &noteRow.Tags, &noteRow.CreatedAt, &noteRow.UpdatedAt)
	if err == nil {
		note = &noteRow
	} else if mapDBError(err) != ErrNotFound {
		return nil, nil, err
	}

	rows, err := r.db.Query(ctx, `
		SELECT id, user_id, item_id, quote_text, anchor_text, section, created_at
		FROM item_highlights
		WHERE user_id = $1 AND item_id = $2
		ORDER BY created_at DESC`,
		userID, itemID,
	)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	highlights := []model.ItemHighlight{}
	for rows.Next() {
		var highlight model.ItemHighlight
		if err := rows.Scan(&highlight.ID, &highlight.UserID, &highlight.ItemID, &highlight.QuoteText, &highlight.AnchorText, &highlight.Section, &highlight.CreatedAt); err != nil {
			return nil, nil, err
		}
		highlights = append(highlights, highlight)
	}
	return note, highlights, rows.Err()
}

func (r *ItemRepo) UpsertNote(ctx context.Context, note model.ItemNote) (model.ItemNote, error) {
	if err := r.ensureOwned(ctx, note.UserID, note.ItemID); err != nil {
		return model.ItemNote{}, err
	}
	if note.ID == "" {
		note.ID = uuid.NewString()
	}
	err := r.db.QueryRow(ctx, `
		INSERT INTO item_notes (id, user_id, item_id, content, tags)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (user_id, item_id) DO UPDATE
		SET content = EXCLUDED.content,
		    tags = EXCLUDED.tags,
		    updated_at = now()
		RETURNING id, created_at, updated_at`,
		note.ID, note.UserID, note.ItemID, note.Content, note.Tags,
	).Scan(&note.ID, &note.CreatedAt, &note.UpdatedAt)
	if err != nil {
		return model.ItemNote{}, err
	}
	return note, nil
}

func (r *ItemRepo) CreateHighlight(ctx context.Context, highlight model.ItemHighlight) (model.ItemHighlight, error) {
	if err := r.ensureOwned(ctx, highlight.UserID, highlight.ItemID); err != nil {
		return model.ItemHighlight{}, err
	}
	if highlight.ID == "" {
		highlight.ID = uuid.NewString()
	}
	err := r.db.QueryRow(ctx, `
		INSERT INTO item_highlights (id, user_id, item_id, quote_text, anchor_text, section)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING created_at`,
		highlight.ID, highlight.UserID, highlight.ItemID, highlight.QuoteText, highlight.AnchorText, highlight.Section,
	).Scan(&highlight.CreatedAt)
	if err != nil {
		return model.ItemHighlight{}, err
	}
	return highlight, nil
}

func (r *ItemRepo) DeleteHighlight(ctx context.Context, userID, itemID, highlightID string) error {
	if err := r.ensureOwned(ctx, userID, itemID); err != nil {
		return err
	}
	_, err := r.db.Exec(ctx, `
		DELETE FROM item_highlights
		WHERE user_id = $1 AND item_id = $2 AND id = $3`,
		userID, itemID, highlightID,
	)
	return err
}
