CREATE INDEX IF NOT EXISTS idx_directions_link_source_destination
ON directions(link_id, source_agent_id, destination_agent_id);

CREATE INDEX IF NOT EXISTS idx_agents_online_created
ON agents(online, created_at DESC, id DESC);
