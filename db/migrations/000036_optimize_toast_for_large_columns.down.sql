ALTER TABLE items
  ALTER COLUMN content_text SET STORAGE EXTENDED,
  ALTER COLUMN processing_error SET STORAGE EXTENDED;

ALTER TABLE item_facts
  ALTER COLUMN facts SET STORAGE EXTENDED;

ALTER TABLE item_summaries
  ALTER COLUMN summary SET STORAGE EXTENDED,
  ALTER COLUMN score_breakdown SET STORAGE EXTENDED,
  ALTER COLUMN score_reason SET STORAGE EXTENDED,
  ALTER COLUMN translated_title SET STORAGE EXTENDED;

ALTER TABLE digests
  ALTER COLUMN email_body SET STORAGE EXTENDED,
  ALTER COLUMN send_error SET STORAGE EXTENDED;

ALTER TABLE digest_cluster_drafts
  ALTER COLUMN topics SET STORAGE EXTENDED,
  ALTER COLUMN draft_summary SET STORAGE EXTENDED;

ALTER TABLE briefing_snapshots
  ALTER COLUMN payload_json SET STORAGE EXTENDED;

DO $$
BEGIN
  ALTER TABLE items
    ALTER COLUMN content_text SET COMPRESSION default,
    ALTER COLUMN processing_error SET COMPRESSION default;

  ALTER TABLE item_facts
    ALTER COLUMN facts SET COMPRESSION default;

  ALTER TABLE item_summaries
    ALTER COLUMN summary SET COMPRESSION default,
    ALTER COLUMN score_breakdown SET COMPRESSION default,
    ALTER COLUMN score_reason SET COMPRESSION default,
    ALTER COLUMN translated_title SET COMPRESSION default;

  ALTER TABLE digests
    ALTER COLUMN email_body SET COMPRESSION default,
    ALTER COLUMN send_error SET COMPRESSION default;

  ALTER TABLE digest_cluster_drafts
    ALTER COLUMN topics SET COMPRESSION default,
    ALTER COLUMN draft_summary SET COMPRESSION default;

  ALTER TABLE briefing_snapshots
    ALTER COLUMN payload_json SET COMPRESSION default;
EXCEPTION
  WHEN undefined_object OR feature_not_supported THEN
    NULL;
END
$$;
