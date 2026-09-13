package storage

import (
	"bytes"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"netprob/internal/models"
)

// VMMetricsStore mirrors ping, MTR summaries and route metadata, and controller
// inventory to VictoriaMetrics. Complete MTR snapshots remain in SQLite.
type VMMStore struct {
	url      string
	client   *http.Client
	httpPath string
}

// NewVMMetricsStore creates a new VictoriaMetrics metrics store.
func NewVMMetricsStore(vmURL string) *VMMStore {
	return &VMMStore{
		url:      vmURL,
		client:   &http.Client{Timeout: 30 * time.Second},
		httpPath: "/api/v1/import/prometheus",
	}
}

func (s *VMMStore) WritePing(result *models.PingResult, context MetricContext) error {
	lines := s.pingToPrometheusLines(result, context)
	return s.writePrometheusLines(lines)
}

func (s *VMMStore) WriteMTR(run *models.MTRRun, context MetricContext) error {
	lines := s.mtrToPrometheusLines(run, context)
	return s.writePrometheusLines(lines)
}

func (s *VMMStore) WriteProbeStatus(probe string, success bool, timestamp time.Time, context MetricContext) error {
	lines := []string{prometheusLine(
		"netprob_probe_success",
		withLabel(metricLabels(context), "probe", probe),
		boolFloat(success),
		timestamp.UnixMilli(),
	)}
	return s.writePrometheusLines(lines)
}

func (s *VMMStore) WriteInventory(snapshot InventorySnapshot) error {
	lines := s.inventoryToPrometheusLines(snapshot)
	return s.writePrometheusLines(lines)
}

func (s *VMMStore) writePrometheusLines(lines []string) error {
	if len(lines) == 0 {
		return nil
	}

	buf := &bytes.Buffer{}
	for _, line := range lines {
		buf.WriteString(line)
		buf.WriteString("\n")
	}

	endpoint, err := url.JoinPath(s.url, s.httpPath)
	if err != nil {
		return fmt.Errorf("join url: %w", err)
	}

	req, err := http.NewRequest("POST", endpoint, buf)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "text/plain")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("vm write request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("vm write: status %d", resp.StatusCode)
	}
	return nil
}

func (s *VMMStore) pingToPrometheusLines(r *models.PingResult, context MetricContext) []string {
	var lines []string
	labels := metricLabels(context)
	ts := r.Timestamp.UnixMilli()

	if r.AvgRTT != nil {
		lines = append(lines, prometheusLine("netprob_ping_rtt_avg_ms", labels, *r.AvgRTT, ts))
	}
	if r.MinRTT != nil {
		lines = append(lines, prometheusLine("netprob_ping_rtt_min_ms", labels, *r.MinRTT, ts))
	}
	if r.MaxRTT != nil {
		lines = append(lines, prometheusLine("netprob_ping_rtt_max_ms", labels, *r.MaxRTT, ts))
	}
	if r.PacketLoss != nil {
		lines = append(lines, prometheusLine("netprob_ping_packet_loss_percent", labels, *r.PacketLoss, ts))
	}
	if r.Jitter != nil {
		lines = append(lines, prometheusLine("netprob_ping_jitter_ms", labels, *r.Jitter, ts))
	}
	lines = append(lines,
		prometheusLine("netprob_ping_packets_sent", labels, float64(r.PacketsSent), ts),
		prometheusLine("netprob_ping_packets_received", labels, float64(r.PacketsReceived), ts),
		prometheusLine("netprob_probe_success", withLabel(labels, "probe", "ping"), 1, ts),
	)
	return lines
}

func (s *VMMStore) mtrToPrometheusLines(run *models.MTRRun, context MetricContext) []string {
	labels := metricLabels(context)
	ts := run.Timestamp.UnixMilli()
	success := 1.0
	if run.Status == "error" || run.Error != "" {
		success = 0
	}
	lines := []string{
		prometheusLine("netprob_probe_success", withLabel(labels, "probe", "mtr"), success, ts),
		prometheusLine("netprob_mtr_hop_count", labels, float64(len(run.Hops)), ts),
	}
	if len(run.Hops) == 0 {
		return lines
	}
	maxLoss := run.Hops[0].LossPercent
	for _, hop := range run.Hops[1:] {
		if hop.LossPercent > maxLoss {
			maxLoss = hop.LossPercent
		}
	}
	lines = append(lines, prometheusLine("netprob_mtr_max_hop_loss_percent", labels, maxLoss, ts))
	if destinationRTT := run.Hops[len(run.Hops)-1].AvgMs; destinationRTT != nil {
		lines = append(lines, prometheusLine("netprob_mtr_destination_rtt_avg_ms", labels, *destinationRTT, ts))
	}
	observedAt := float64(run.Timestamp.UnixMilli()) / 1000
	for _, hop := range run.Hops {
		hopLabels := withLabels(labels, map[string]string{
			"hop_number": strconv.Itoa(hop.HopNumber),
			"hop_host":   hop.Host,
			"hop_ip":     hop.IP,
		})
		lines = append(lines, prometheusLine("netprob_mtr_hop_observed_timestamp_seconds", hopLabels, observedAt, ts))
	}
	return lines
}

func (s *VMMStore) inventoryToPrometheusLines(snapshot InventorySnapshot) []string {
	ts := snapshot.Timestamp.UnixMilli()
	lines := []string{
		prometheusLine("netprob_controller_info", map[string]string{"version": snapshot.ControllerVersion}, 1, ts),
		prometheusLine("netprob_controller_uptime_seconds", nil, snapshot.Timestamp.Sub(snapshot.StartedAt).Seconds(), ts),
		prometheusLine("netprob_controller_agents", nil, float64(len(snapshot.Agents)), ts),
		prometheusLine("netprob_controller_links", nil, float64(len(snapshot.Links)), ts),
		prometheusLine("netprob_controller_directions", nil, float64(len(snapshot.Directions)), ts),
	}

	agents := make(map[string]*models.Agent, len(snapshot.Agents))
	online := 0
	for _, agent := range snapshot.Agents {
		agents[agent.ID] = agent
		value := 0.0
		if agent.Online {
			value = 1
			online++
		}
		labels := agentMetricLabels(agent)
		lines = append(lines, prometheusLine("netprob_agent_online", labels, value, ts))
		lastSeen := 0.0
		if agent.LastSeen != nil {
			lastSeen = float64(agent.LastSeen.Unix())
		}
		lines = append(lines, prometheusLine("netprob_agent_last_seen_seconds", labels, lastSeen, ts))
	}
	lines = append(lines, prometheusLine("netprob_controller_agents_online", nil, float64(online), ts))

	links := make(map[string]*models.Link, len(snapshot.Links))
	for _, link := range snapshot.Links {
		links[link.ID] = link
		lines = append(lines, prometheusLine("netprob_link_info", map[string]string{
			"link_id": link.ID,
			"link":    link.Name,
		}, 1, ts))
	}
	for _, direction := range snapshot.Directions {
		context := metricContextFromInventory(direction, links[direction.LinkID], agents[direction.SourceAgentID], agents[direction.DestinationAgentID])
		labels := metricLabels(context)
		lines = append(lines,
			prometheusLine("netprob_direction_info", labels, 1, ts),
			prometheusLine("netprob_direction_ping_enabled", labels, boolFloat(direction.PingEnabled), ts),
			prometheusLine("netprob_direction_mtr_enabled", labels, boolFloat(direction.MTREnabled), ts),
			prometheusLine("netprob_direction_ping_interval_seconds", labels, float64(direction.PingInterval), ts),
			prometheusLine("netprob_direction_mtr_interval_seconds", labels, float64(direction.MTRInterval), ts),
		)
	}
	return lines
}

func metricContextFromInventory(direction *models.Direction, link *models.Link, source, destination *models.Agent) MetricContext {
	context := MetricContext{
		LinkID:             direction.LinkID,
		DirectionID:        direction.ID,
		TargetAddress:      direction.TargetAddress,
		SourceAgentID:      direction.SourceAgentID,
		DestinationAgentID: direction.DestinationAgentID,
	}
	if link != nil {
		context.LinkName = link.Name
	}
	if source != nil {
		context.SourceHostname = source.Hostname
		context.SourceAddress = metricAgentAddress(source)
		context.SourceRegion = source.Location.Region
		context.SourceProvider = metricAgentProvider(source)
	}
	if destination != nil {
		context.DestinationHostname = destination.Hostname
		context.DestinationAddress = metricAgentAddress(destination)
		context.DestinationRegion = destination.Location.Region
		context.DestinationProvider = metricAgentProvider(destination)
	}
	if context.SourceHostname == "" {
		context.SourceHostname = context.SourceAgentID
	}
	if context.DestinationHostname == "" {
		context.DestinationHostname = context.DestinationAgentID
	}
	return context
}

func metricLabels(context MetricContext) map[string]string {
	return map[string]string{
		"link_id":              context.LinkID,
		"link":                 context.LinkName,
		"direction_id":         context.DirectionID,
		"target_address":       context.TargetAddress,
		"source":               context.SourceHostname,
		"source_agent_id":      context.SourceAgentID,
		"source_address":       context.SourceAddress,
		"source_region":        context.SourceRegion,
		"source_provider":      context.SourceProvider,
		"destination":          context.DestinationHostname,
		"destination_agent_id": context.DestinationAgentID,
		"destination_address":  context.DestinationAddress,
		"destination_region":   context.DestinationRegion,
		"destination_provider": context.DestinationProvider,
	}
}

func agentMetricLabels(agent *models.Agent) map[string]string {
	return map[string]string{
		"agent_id": agent.ID,
		"hostname": agent.Hostname,
		"address":  metricAgentAddress(agent),
		"region":   agent.Location.Region,
		"country":  agent.Location.CountryCode,
		"provider": metricAgentProvider(agent),
		"version":  agent.Version,
	}
}

func withLabel(labels map[string]string, name, value string) map[string]string {
	result := make(map[string]string, len(labels)+1)
	for key, existing := range labels {
		result[key] = existing
	}
	result[name] = value
	return result
}

func withLabels(labels map[string]string, additional map[string]string) map[string]string {
	result := make(map[string]string, len(labels)+len(additional))
	for key, existing := range labels {
		result[key] = existing
	}
	for key, value := range additional {
		result[key] = value
	}
	return result
}

func prometheusLine(name string, labels map[string]string, value float64, timestamp int64) string {
	var labelSet string
	if len(labels) > 0 {
		keys := make([]string, 0, len(labels))
		for key := range labels {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		parts := make([]string, 0, len(keys))
		for _, key := range keys {
			parts = append(parts, fmt.Sprintf("%s=%s", key, strconv.Quote(labels[key])))
		}
		labelSet = "{" + strings.Join(parts, ",") + "}"
	}
	return fmt.Sprintf("%s%s %s %d", name, labelSet, strconv.FormatFloat(value, 'f', -1, 64), timestamp)
}

func boolFloat(value bool) float64 {
	if value {
		return 1
	}
	return 0
}
