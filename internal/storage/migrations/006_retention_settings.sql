CREATE TABLE retention_settings (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    ping_retention_days INTEGER NOT NULL DEFAULT 0 CHECK (ping_retention_days >= 0),
    mtr_retention_days INTEGER NOT NULL DEFAULT 0 CHECK (mtr_retention_days >= 0),
    updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

INSERT INTO retention_settings (id, ping_retention_days, mtr_retention_days)
VALUES (1, 0, 0);
