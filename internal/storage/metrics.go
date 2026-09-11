package storage

import (
	"strings"
	"time"

	"netprob/internal/models"
)

// MetricContext contains stable IDs plus human-readable endpoint metadata.
// The IDs remain available for exact matching while Grafana dashboards can use
// hostnames, addresses, regions, and providers directly.
type MetricContext struct {
	LinkID              string
	LinkName            string
	DirectionID         string
	TargetAddress       string
	SourceAgentID       string
	SourceHostname      string
	SourceAddress       string
	SourceRegion        string
	SourceProvider      string
	DestinationAgentID  string
	DestinationHostname string
	DestinationAddress  string
	DestinationRegion   string
	DestinationProvider string
}

// InventorySnapshot is the periodically exported controller inventory.
type InventorySnapshot struct {
	Timestamp         time.Time
	StartedAt         time.Time
	ControllerVersion string
	Agents            []*models.Agent
	Links             []*models.Link
	Directions        []*models.Direction
}

func (s *Store) metricContext(directionID, sourceAgentID, destinationAgentID string) MetricContext {
	ctx := MetricContext{
		DirectionID:        directionID,
		SourceAgentID:      sourceAgentID,
		DestinationAgentID: destinationAgentID,
	}

	if direction, err := s.DB.GetDirectionByID(directionID); err == nil {
		ctx.LinkID = direction.LinkID
		ctx.TargetAddress = direction.TargetAddress
		if ctx.SourceAgentID == "" {
			ctx.SourceAgentID = direction.SourceAgentID
		}
		if ctx.DestinationAgentID == "" {
			ctx.DestinationAgentID = direction.DestinationAgentID
		}
		if link, err := s.DB.GetLinkByID(direction.LinkID); err == nil {
			ctx.LinkName = link.Name
		}
	}

	if agent, err := s.DB.GetAgentByID(ctx.SourceAgentID); err == nil {
		ctx.SourceHostname = agent.Hostname
		ctx.SourceAddress = metricAgentAddress(agent)
		ctx.SourceRegion = agent.Location.Region
		ctx.SourceProvider = metricAgentProvider(agent)
	}
	if agent, err := s.DB.GetAgentByID(ctx.DestinationAgentID); err == nil {
		ctx.DestinationHostname = agent.Hostname
		ctx.DestinationAddress = metricAgentAddress(agent)
		ctx.DestinationRegion = agent.Location.Region
		ctx.DestinationProvider = metricAgentProvider(agent)
	}

	if ctx.SourceHostname == "" {
		ctx.SourceHostname = ctx.SourceAgentID
	}
	if ctx.DestinationHostname == "" {
		ctx.DestinationHostname = ctx.DestinationAgentID
	}
	return ctx
}

func metricAgentAddress(agent *models.Agent) string {
	if strings.TrimSpace(agent.PrimaryAddress) != "" {
		return agent.PrimaryAddress
	}
	if strings.TrimSpace(agent.Location.PublicIP) != "" {
		return agent.Location.PublicIP
	}
	if len(agent.Addresses) > 0 {
		return agent.Addresses[0]
	}
	return ""
}

func metricAgentProvider(agent *models.Agent) string {
	if strings.TrimSpace(agent.Location.ASName) != "" {
		return agent.Location.ASName
	}
	return agent.Location.ISP
}
