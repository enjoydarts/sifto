package repository

import "time"

type FishAudioModelSnapshot struct {
	ID                int64      `json:"id"`
	SyncRunID         int64      `json:"sync_run_id"`
	ModelID           string     `json:"model_id"`
	Title             string     `json:"title"`
	Description       string     `json:"description"`
	CoverImage        string     `json:"cover_image"`
	Visibility        string     `json:"visibility"`
	TrainMode         string     `json:"train_mode"`
	AuthorName        string     `json:"author_name"`
	AuthorAvatar      string     `json:"author_avatar"`
	LanguageCodesJSON []byte     `json:"language_codes_json"`
	TagsJSON          []byte     `json:"tags_json"`
	SamplesJSON       []byte     `json:"samples_json"`
	MetadataJSON      []byte     `json:"metadata_json"`
	LikeCount         int        `json:"like_count"`
	MarkCount         int        `json:"mark_count"`
	SharedCount       int        `json:"shared_count"`
	TaskCount         int        `json:"task_count"`
	SampleCount       int        `json:"sample_count"`
	CreatedAtRemote   *time.Time `json:"created_at_remote,omitempty"`
	UpdatedAtRemote   *time.Time `json:"updated_at_remote,omitempty"`
	FetchedAt         time.Time  `json:"fetched_at"`
}
