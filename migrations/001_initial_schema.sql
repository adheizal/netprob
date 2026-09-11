-- Initial schema for NetProb

CREATE TABLE agents (
    id TEXT PRIMARY KEY,
    hostname TEXT NOT NULL,
    version TEXT NOT NULL,
    addresses TEXT NOT NULL, -- JSON array of addresses
    capabilities TEXT NOT NULL, -- JSON array of capabilities
    token_hash TEXT NOT NULL,
    online BOOLEAN NOT NULL DEFAULT 0,
    last_seen DATETIME,
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE links (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT,
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE directions (
    id TEXT PRIMARY KEY,
    link_id TEXT NOT NULL,
    source_agent_id TEXT NOT NULL,
    destination_agent_id TEXT NOT NULL,
    target_address TEXT NOT NULL,
    ping_interval_seconds INTEGER NOT NULL DEFAULT 5,
    mtr_interval_seconds INTEGER NOT NULL DEFAULT 120,
    ping_enabled BOOLEAN NOT NULL DEFAULT 1,
    mtr_enabled BOOLEAN NOT NULL DEFAULT 1,
    mtr_threshold_loss_percent REAL,
    mtr_threshold_latency_ms REAL,
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at DATETIME NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (link_id) REFERENCES links(id) ON DELETE CASCADE,
    FOREIGN KEY (source_agent_id) REFERENCES agents(id),
    FOREIGN KEY (destination_agent_id) REFERENCES agents(id),
    UNIQUE (link_id, source_agent_id, destination_agent_id)
);

CREATE TABLE ping_results (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    direction_id TEXT NOT NULL,
    source_agent_id TEXT NOT NULL,
    destination_agent_id TEXT NOT NULL,
    timestamp DATETIME NOT NULL DEFAULT (datetime('now')),
    min_rtt_ms REAL,
    avg_rtt_ms REAL,
    max_rtt_ms REAL,
    jitter_ms REAL,
    packet_loss_percent REAL,
    packets_sent INTEGER,
    packets_received INTEGER,
    FOREIGN KEY (direction_id) REFERENCES directions(id),
    FOREIGN KEY (source_agent_id) REFERENCES agents(id),
    FOREIGN KEY (destination_agent_id) REFERENCES agents(id)
);

CREATE INDEX idx_ping_results_direction ON ping_results(direction_id);
CREATE INDEX idx_ping_results_timestamp ON ping_results(timestamp);

CREATE TABLE mtr_runs (
    id TEXT PRIMARY KEY,
    direction_id TEXT NOT NULL,
    source_agent_id TEXT NOT NULL,
    destination_agent_id TEXT NOT NULL,
    timestamp DATETIME NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (direction_id) REFERENCES directions(id),
    FOREIGN KEY (source_agent_id) REFERENCES agents(id),
    FOREIGN KEY (destination_agent_id) REFERENCES agents(id)
);

CREATE INDEX idx_mtr_runs_direction ON mtr_runs(direction_id);
CREATE INDEX idx_mtr_runs_timestamp ON mtr_runs(timestamp);

CREATE TABLE mtr_hops (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    mtr_run_id TEXT NOT NULL,
    hop_number INTEGER NOT NULL,
    host TEXT,
    ip TEXT,
    loss_percent REAL,
    sent INTEGER,
    last_ms REAL,
    avg_ms REAL,
    best_ms REAL,
    worst_ms REAL,
    FOREIGN KEY (mtr_run_id) REFERENCES mtr_runs(id) ON DELETE CASCADE
);

CREATE INDEX idx_mtr_hops_run ON mtr_hops(mtr_run_id);
