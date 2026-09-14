package storage

import (
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"netprob/internal/models"
)

func (s *SQLiteStore) CreateAgent(a *models.Agent) error {
	addrs, _ := json.Marshal(a.Addresses)
	caps, _ := json.Marshal(a.Capabilities)
	_, err := s.db.Exec(`
		INSERT INTO agents (id, hostname, version, addresses, capabilities, token_hash, instance_id, online, last_seen, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, a.ID, a.Hostname, a.Version, string(addrs), string(caps), a.TokenHash, a.InstanceID, a.Online, a.LastSeen, time.Now(), time.Now())
	return err
}

func (s *SQLiteStore) GetAgentByID(id string) (*models.Agent, error) {
	row := s.db.QueryRow(`
		SELECT id, hostname, version, addresses, primary_address, capabilities, public_ip, country_code, region, city, timezone, as_name, isp,
		       token_hash, instance_id, online, last_seen, created_at, updated_at
		FROM agents WHERE id = ?
	`, id)
	return scanAgent(row)
}

// ResolveAgentByTokenAndInstance authenticates an enrollment token and returns
// a stable agent record for one instance. The first instance claims the pending
// registration row; additional instances get their own records with the same
// token hash.
func (s *SQLiteStore) ResolveAgentByTokenAndInstance(tokenHash, instanceID string) (*models.Agent, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	selectColumns := `
		SELECT id, hostname, version, addresses, primary_address, capabilities, public_ip, country_code, region, city, timezone, as_name, isp,
		       token_hash, instance_id, online, last_seen, created_at, updated_at
		FROM agents`

	agent, err := scanAgent(tx.QueryRow(selectColumns+` WHERE token_hash = ? AND instance_id = ?`, tokenHash, instanceID))
	if err == nil {
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return agent, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	agent, err = scanAgent(tx.QueryRow(selectColumns+` WHERE token_hash = ? AND instance_id = '' ORDER BY created_at LIMIT 1`, tokenHash))
	if err == nil {
		result, updateErr := tx.Exec(`UPDATE agents SET instance_id = ?, updated_at = ? WHERE id = ? AND instance_id = ''`, instanceID, time.Now(), agent.ID)
		if updateErr != nil {
			return nil, updateErr
		}
		updated, updateErr := result.RowsAffected()
		if updateErr != nil {
			return nil, updateErr
		}
		if updated != 1 {
			return nil, errors.New("agent enrollment was claimed concurrently")
		}
		agent.InstanceID = instanceID
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return agent, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	var tokenExists bool
	if err := tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM agents WHERE token_hash = ?)`, tokenHash).Scan(&tokenExists); err != nil {
		return nil, err
	}
	if !tokenExists {
		return nil, sql.ErrNoRows
	}

	agent = models.NewAgent("pending", "pending", []string{}, []string{})
	agent.TokenHash = tokenHash
	agent.InstanceID = instanceID
	addrs, _ := json.Marshal(agent.Addresses)
	caps, _ := json.Marshal(agent.Capabilities)
	now := time.Now()
	if _, err := tx.Exec(`
		INSERT INTO agents (id, hostname, version, addresses, capabilities, token_hash, instance_id, online, last_seen, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, agent.ID, agent.Hostname, agent.Version, string(addrs), string(caps), agent.TokenHash, agent.InstanceID, agent.Online, agent.LastSeen, now, now); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return agent, nil
}

func (s *SQLiteStore) ListAgents() ([]*models.Agent, error) {
	rows, err := s.db.Query(`
		SELECT id, hostname, version, addresses, primary_address, capabilities, public_ip, country_code, region, city, timezone, as_name, isp,
		       token_hash, instance_id, online, last_seen, created_at, updated_at
		FROM agents ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	agents := make([]*models.Agent, 0)
	for rows.Next() {
		a, err := scanAgent(rows)
		if err != nil {
			return nil, err
		}
		agents = append(agents, a)
	}
	return agents, rows.Err()
}

func (s *SQLiteStore) ListAgentsPage(limit, offset int) ([]*models.Agent, error) {
	rows, err := s.db.Query(`
		SELECT id, hostname, version, addresses, primary_address, capabilities, public_ip, country_code, region, city, timezone, as_name, isp,
		       token_hash, instance_id, online, last_seen, created_at, updated_at
		FROM agents ORDER BY created_at DESC, id DESC LIMIT ? OFFSET ?
	`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	agents := make([]*models.Agent, 0)
	for rows.Next() {
		agent, err := scanAgent(rows)
		if err != nil {
			return nil, err
		}
		agents = append(agents, agent)
	}
	return agents, rows.Err()
}

func (s *SQLiteStore) CountAgents() (int, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM agents`).Scan(&count)
	return count, err
}

func (s *SQLiteStore) UpdateAgentStatus(id string, online bool, lastSeen time.Time) error {
	_, err := s.db.Exec(`
		UPDATE agents SET online = ?, last_seen = ?, updated_at = ? WHERE id = ?
	`, online, lastSeen, time.Now(), id)
	return err
}

func (s *SQLiteStore) UpdateAgentMetadata(id, hostname, version, primaryAddress string, addresses, capabilities []string, location models.AgentLocation) error {
	addrs, err := json.Marshal(addresses)
	if err != nil {
		return err
	}
	caps, err := json.Marshal(capabilities)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`
		UPDATE agents
		SET hostname = ?, version = ?, addresses = ?, primary_address = ?, capabilities = ?,
		    public_ip = ?, country_code = ?, region = ?, city = ?, timezone = ?, as_name = ?, isp = ?, updated_at = ?
		WHERE id = ?
	`, hostname, version, string(addrs), primaryAddress, string(caps), location.PublicIP, location.CountryCode,
		location.Region, location.City, location.Timezone, location.ASName, location.ISP, time.Now(), id)
	return err
}

func (s *SQLiteStore) DeleteAgent(id string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	rows, err := tx.Query(`
		SELECT DISTINCT link_id FROM directions
		WHERE source_agent_id = ? OR destination_agent_id = ?
	`, id, id)
	if err != nil {
		return err
	}
	var linkIDs []string
	for rows.Next() {
		var linkID string
		if err := rows.Scan(&linkID); err != nil {
			rows.Close()
			return err
		}
		linkIDs = append(linkIDs, linkID)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if err := rows.Err(); err != nil {
		return err
	}

	statements := []string{
		`DELETE FROM mtr_hops WHERE mtr_run_id IN (
			SELECT id FROM mtr_runs WHERE source_agent_id = ? OR destination_agent_id = ? OR direction_id IN (
				SELECT id FROM directions WHERE source_agent_id = ? OR destination_agent_id = ?
			)
		)`,
		`DELETE FROM mtr_runs WHERE source_agent_id = ? OR destination_agent_id = ? OR direction_id IN (
			SELECT id FROM directions WHERE source_agent_id = ? OR destination_agent_id = ?
		)`,
		`DELETE FROM ping_results WHERE source_agent_id = ? OR destination_agent_id = ? OR direction_id IN (
			SELECT id FROM directions WHERE source_agent_id = ? OR destination_agent_id = ?
		)`,
		`DELETE FROM directions WHERE source_agent_id = ? OR destination_agent_id = ?`,
	}
	for _, statement := range statements {
		if _, err := tx.Exec(statement, id, id, id, id); err != nil {
			return err
		}
	}
	for _, linkID := range linkIDs {
		if _, err := tx.Exec(`DELETE FROM links WHERE id = ? AND NOT EXISTS (
			SELECT 1 FROM directions WHERE link_id = ?
		)`, linkID, linkID); err != nil {
			return err
		}
	}
	result, err := tx.Exec(`DELETE FROM agents WHERE id = ?`, id)
	if err != nil {
		return err
	}
	deleted, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if deleted == 0 {
		return sql.ErrNoRows
	}
	return tx.Commit()
}

func scanAgent(row interface {
	Scan(dest ...any) error
}) (*models.Agent, error) {
	var a models.Agent
	var addrsJSON, capsJSON string
	var lastSeen sql.NullTime
	var createdAt, updatedAt time.Time

	err := row.Scan(
		&a.ID, &a.Hostname, &a.Version, &addrsJSON, &a.PrimaryAddress, &capsJSON,
		&a.Location.PublicIP, &a.Location.CountryCode, &a.Location.Region, &a.Location.City, &a.Location.Timezone,
		&a.Location.ASName, &a.Location.ISP,
		&a.TokenHash, &a.InstanceID, &a.Online, &lastSeen, &createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}
	json.Unmarshal([]byte(addrsJSON), &a.Addresses)
	json.Unmarshal([]byte(capsJSON), &a.Capabilities)
	if lastSeen.Valid {
		a.LastSeen = &lastSeen.Time
	}
	a.CreatedAt = createdAt
	a.UpdatedAt = updatedAt
	return &a, nil
}
