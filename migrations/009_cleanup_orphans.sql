-- Remove relationship rows created while SQLite foreign-key enforcement was
-- disabled. New connections enable foreign_keys and repository deletes are
-- ordered transactionally so these rows cannot be recreated.
DELETE FROM mtr_hops
WHERE mtr_run_id NOT IN (SELECT id FROM mtr_runs);

DELETE FROM mtr_hops
WHERE mtr_run_id IN (
    SELECT r.id FROM mtr_runs r
    LEFT JOIN directions d ON d.id = r.direction_id
    LEFT JOIN agents s ON s.id = r.source_agent_id
    LEFT JOIN agents t ON t.id = r.destination_agent_id
    WHERE d.id IS NULL OR s.id IS NULL OR t.id IS NULL
       OR r.direction_id IN (
           SELECT invalid.id FROM directions invalid
           LEFT JOIN links l ON l.id = invalid.link_id
           LEFT JOIN agents ds ON ds.id = invalid.source_agent_id
           LEFT JOIN agents dt ON dt.id = invalid.destination_agent_id
           WHERE l.id IS NULL OR ds.id IS NULL OR dt.id IS NULL
       )
);

DELETE FROM mtr_runs
WHERE direction_id NOT IN (SELECT id FROM directions)
   OR source_agent_id NOT IN (SELECT id FROM agents)
   OR destination_agent_id NOT IN (SELECT id FROM agents)
   OR direction_id IN (
       SELECT invalid.id FROM directions invalid
       LEFT JOIN links l ON l.id = invalid.link_id
       LEFT JOIN agents s ON s.id = invalid.source_agent_id
       LEFT JOIN agents t ON t.id = invalid.destination_agent_id
       WHERE l.id IS NULL OR s.id IS NULL OR t.id IS NULL
   );

DELETE FROM ping_results
WHERE direction_id NOT IN (SELECT id FROM directions)
   OR source_agent_id NOT IN (SELECT id FROM agents)
   OR destination_agent_id NOT IN (SELECT id FROM agents)
   OR direction_id IN (
       SELECT invalid.id FROM directions invalid
       LEFT JOIN links l ON l.id = invalid.link_id
       LEFT JOIN agents s ON s.id = invalid.source_agent_id
       LEFT JOIN agents t ON t.id = invalid.destination_agent_id
       WHERE l.id IS NULL OR s.id IS NULL OR t.id IS NULL
   );

DELETE FROM directions
WHERE link_id NOT IN (SELECT id FROM links)
   OR source_agent_id NOT IN (SELECT id FROM agents)
   OR destination_agent_id NOT IN (SELECT id FROM agents);

CREATE INDEX IF NOT EXISTS idx_ping_results_direction_timestamp
ON ping_results(direction_id, timestamp DESC, id DESC);
