ALTER TABLE agents ADD COLUMN instance_id TEXT NOT NULL DEFAULT '';

CREATE UNIQUE INDEX idx_agents_token_instance
ON agents(token_hash, instance_id)
WHERE instance_id <> '';
