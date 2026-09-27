package storage

import (
	"database/sql"
	"fmt"
	"strings"

	"netprob/internal/models"
)

const linkStatsQuery = `
	SELECT l.id, l.name, l.description, l.created_at, l.updated_at,
		COUNT(d.id) AS direction_count,
		SUM(CASE WHEN COALESCE(source.online, 0) = 1 THEN 1 ELSE 0 END) AS online_count,
		SUM(CASE WHEN COALESCE(source.online, 0) = 1 AND d.ping_enabled = 1
			AND p.packet_loss_percent >= 100 THEN 1 ELSE 0 END) AS down_count,
		SUM(CASE WHEN COALESCE(source.online, 0) = 1 AND d.ping_enabled = 1
			AND (p.id IS NULL OR (p.packet_loss_percent > 0 AND p.packet_loss_percent < 100))
			THEN 1 ELSE 0 END) AS degraded_count,
		MAX(p.packet_loss_percent) AS max_loss,
		MAX(p.avg_rtt_ms) AS max_latency,
		MAX(p.timestamp) AS last_metric_at
	FROM links l
	LEFT JOIN directions d ON d.link_id = l.id
	LEFT JOIN agents source ON source.id = d.source_agent_id
	LEFT JOIN ping_results p ON p.id = (
		SELECT newest.id FROM ping_results newest
		WHERE newest.direction_id = d.id
		ORDER BY newest.timestamp DESC, newest.id DESC LIMIT 1
	)
	WHERE (? = '' OR lower(l.name) LIKE ? OR lower(COALESCE(l.description, '')) LIKE ? OR EXISTS (
		SELECT 1 FROM directions searched_direction
		LEFT JOIN agents searched_source ON searched_source.id = searched_direction.source_agent_id
		LEFT JOIN agents searched_destination ON searched_destination.id = searched_direction.destination_agent_id
		WHERE searched_direction.link_id = l.id AND (
			lower(COALESCE(searched_source.hostname, '')) LIKE ? OR
			lower(COALESCE(searched_destination.hostname, '')) LIKE ? OR
			lower(searched_direction.target_address) LIKE ?
		)
	))
	AND (? = '' OR EXISTS (
		SELECT 1 FROM directions agent_direction
		WHERE agent_direction.link_id = l.id
		AND (agent_direction.source_agent_id = ? OR agent_direction.destination_agent_id = ?)
	))
	AND (? = '' OR EXISTS (
		SELECT 1 FROM directions source_direction
		WHERE source_direction.link_id = l.id AND source_direction.source_agent_id = ?
	))
	AND (? = '' OR EXISTS (
		SELECT 1 FROM directions destination_direction
		WHERE destination_direction.link_id = l.id AND destination_direction.destination_agent_id = ?
	))
	GROUP BY l.id
`

const classifiedLinkQuery = `
	WITH link_stats AS (` + linkStatsQuery + `),
	classified AS (
		SELECT *, CASE
			WHEN direction_count = 0 OR online_count = 0 THEN 'inactive'
			WHEN down_count > 0 THEN 'down'
			WHEN online_count < direction_count OR degraded_count > 0 THEN 'degraded'
			ELSE 'healthy'
		END AS status
		FROM link_stats
	)
`

type LinkListOptions struct {
	Limit              int
	Offset             int
	Search             string
	Status             string
	AgentID            string
	SourceAgentID      string
	DestinationAgentID string
	Sort               string
	Order              string
}

type LinkListItem struct {
	Link       *models.Link
	Status     string
	MaxLoss    *float64
	MaxLatency *float64
}

type LinkStatusCounts struct {
	Total    int `json:"total"`
	Healthy  int `json:"healthy"`
	Degraded int `json:"degraded"`
	Down     int `json:"down"`
	Inactive int `json:"inactive"`
}

type AgentStatusCounts struct {
	Total   int `json:"total"`
	Online  int `json:"online"`
	Offline int `json:"offline"`
}

func linkFilterArgs(options LinkListOptions) []any {
	search := strings.ToLower(strings.TrimSpace(options.Search))
	pattern := "%" + search + "%"
	return []any{
		search, pattern, pattern, pattern, pattern, pattern,
		options.AgentID, options.AgentID, options.AgentID,
		options.SourceAgentID, options.SourceAgentID,
		options.DestinationAgentID, options.DestinationAgentID,
	}
}

func linkStatusClause(status string) (string, []any) {
	switch status {
	case "healthy", "degraded", "down", "inactive":
		return " WHERE status = ?", []any{status}
	case "problem":
		return " WHERE status IN ('down', 'degraded', 'inactive')", nil
	default:
		return "", nil
	}
}

func linkOrderClause(sortBy, order string) string {
	direction := "DESC"
	if strings.EqualFold(order, "asc") {
		direction = "ASC"
	}
	switch sortBy {
	case "name":
		return " ORDER BY lower(name) " + direction + ", id ASC"
	case "latency":
		return " ORDER BY max_latency IS NULL, max_latency " + direction + ", lower(name) ASC"
	case "loss":
		return " ORDER BY max_loss IS NULL, max_loss " + direction + ", lower(name) ASC"
	case "updated":
		return " ORDER BY COALESCE(last_metric_at, updated_at) " + direction + ", id DESC"
	default:
		return ` ORDER BY CASE status
			WHEN 'down' THEN 0 WHEN 'degraded' THEN 1 WHEN 'inactive' THEN 2 ELSE 3 END ASC,
			COALESCE(last_metric_at, updated_at) DESC, id DESC`
	}
}

func (s *SQLiteStore) ListLinksPage(options LinkListOptions) ([]LinkListItem, int, error) {
	statusClause, statusArgs := linkStatusClause(options.Status)
	args := append(linkFilterArgs(options), statusArgs...)

	var total int
	if err := s.db.QueryRow(classifiedLinkQuery+" SELECT COUNT(*) FROM classified"+statusClause, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := classifiedLinkQuery + `
		SELECT id, name, description, created_at, updated_at, status,
			max_loss, max_latency
		FROM classified` + statusClause + linkOrderClause(options.Sort, options.Order) + " LIMIT ? OFFSET ?"
	args = append(args, options.Limit, options.Offset)
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]LinkListItem, 0)
	for rows.Next() {
		var link models.Link
		var description sql.NullString
		var maxLoss, maxLatency sql.NullFloat64
		var status string
		if err := rows.Scan(&link.ID, &link.Name, &description, &link.CreatedAt, &link.UpdatedAt, &status, &maxLoss, &maxLatency); err != nil {
			return nil, 0, err
		}
		if description.Valid {
			link.Description = description.String
		}
		item := LinkListItem{Link: &link, Status: status}
		if maxLoss.Valid {
			item.MaxLoss = &maxLoss.Float64
		}
		if maxLatency.Valid {
			item.MaxLatency = &maxLatency.Float64
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func (s *SQLiteStore) CountLinkStatuses() (LinkStatusCounts, error) {
	query := classifiedLinkQuery + `
		SELECT COUNT(*),
			COALESCE(SUM(CASE WHEN status = 'healthy' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'degraded' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'down' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'inactive' THEN 1 ELSE 0 END), 0)
		FROM classified`
	var counts LinkStatusCounts
	err := s.db.QueryRow(query, linkFilterArgs(LinkListOptions{})...).Scan(
		&counts.Total, &counts.Healthy, &counts.Degraded, &counts.Down, &counts.Inactive,
	)
	return counts, err
}

func (s *SQLiteStore) CountAgentStatuses() (AgentStatusCounts, error) {
	var counts AgentStatusCounts
	err := s.db.QueryRow(`
		SELECT COUNT(*),
			COALESCE(SUM(CASE WHEN online = 1 THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN online = 0 THEN 1 ELSE 0 END), 0)
		FROM agents
	`).Scan(&counts.Total, &counts.Online, &counts.Offline)
	return counts, err
}

func placeholders(count int) string {
	return strings.TrimRight(strings.Repeat("?,", count), ",")
}

func (s *SQLiteStore) ListDirectionsByLinkIDs(linkIDs []string) ([]*models.Direction, error) {
	if len(linkIDs) == 0 {
		return []*models.Direction{}, nil
	}
	args := make([]any, len(linkIDs))
	for i, id := range linkIDs {
		args[i] = id
	}
	rows, err := s.db.Query(fmt.Sprintf(`
		SELECT id, link_id, source_agent_id, destination_agent_id, target_address,
			ping_interval_seconds, mtr_interval_seconds, ping_enabled, mtr_enabled,
			mtr_threshold_loss_percent, mtr_threshold_latency_ms, created_at, updated_at
		FROM directions WHERE link_id IN (%s) ORDER BY link_id, id
	`, placeholders(len(linkIDs))), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	directions := make([]*models.Direction, 0)
	for rows.Next() {
		direction, err := scanDirection(rows)
		if err != nil {
			return nil, err
		}
		directions = append(directions, direction)
	}
	return directions, rows.Err()
}
