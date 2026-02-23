-- 部分インデックスを削除し、通常のユニークインデックスに置き換える。
-- ON CONFLICT (idempotency_key) には部分インデックスは使用できないため。
-- NULL 値は PostgreSQL のユニークインデックスでも複数許容される。
DROP INDEX IF EXISTS uq_llm_usage_logs_idempotency_key;

CREATE UNIQUE INDEX IF NOT EXISTS uq_llm_usage_logs_idempotency_key
  ON llm_usage_logs (idempotency_key);
