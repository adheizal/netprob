package storage

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"netprob/internal/models"
)

// VMMetricsStore mirrors ping, MTR summaries, and controller inventory to VictoriaMetrics.
// Complete MTR snapshots and hops remain in SQLite.
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

func (s *VMMStore) QueryPing(sourceAgentID, destAgentID string, from, to time.Time, limit int) ([]*models.PingResult, error) {
	query := fmt.Sprintf(
		`{__name__=~"netprob_ping_(rtt_(avg|min|max)_ms|packet_loss_percent|jitter_ms|packets_(sent|received))",source_agent_id=%q,destination_agent_id=%q}`,
		sourceAgentID, destAgentID,
	)

	params := url.Values{}
	params.Set("query", query)
	params.Set("start", strconv.FormatInt(from.Unix(), 10))
	params.Set("end", strconv.FormatInt(to.Unix(), 10))
	params.Set("step", "1s")

	endpoint := fmt.Sprintf("%s/api/v1/query_range?%s", s.url, params.Encode())
	resp, err := s.client.Get(endpoint)
	if err != nil {
		return nil, fmt.Errorf("vm query: %w", err)
	}
	defer resp.Body.Close()

	var vmResp vmQueryResponse
	if err := json.NewDecoder(resp.Body).Decode(&vmResp); err != nil {
		return nil, fmt.Errorf("decode vm response: %w", err)
	}

	results, err := s.parseQueryResponse(&vmResp)
	if err != nil {
		return nil, err
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].Timestamp.After(results[j].Timestamp)
	})
	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

type vmQueryResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string               `json:"resultType"`
		Result     []vmTimeSeriesResult `json:"result"`
	} `json:"data"`
}

type vmTimeSeriesResult struct {
	Metric map[string]string `json:"metric"`
	Values [][2]string       `json:"values"` // [timestamp, value]
}

func (s *VMMStore) parseQueryResponse(resp *vmQueryResponse) ([]*models.PingResult, error) {
	if resp.Status != "success" {
		return nil, fmt.Errorf("vm query status: %s", resp.Status)
	}

	results := make([]*models.PingResult, 0)
	seen := make(map[int64]*models.PingResult)

	for _, ts := range resp.Data.Result {
		metricName := ts.Metric["__name__"]
		if len(ts.Values) == 0 {
			continue
		}

		for _, val := range ts.Values {
			tsFloat, err := strconv.ParseFloat(val[0], 64)
			if err != nil {
				continue
			}
			tsMs := int64(tsFloat * 1000)
			tsTime := time.Unix(0, tsMs*int64(time.Millisecond))

			valFloat, err := strconv.ParseFloat(val[1], 64)
			if err != nil {
				continue
			}

			r, ok := seen[tsMs]
			if !ok {
				r = &models.PingResult{
					SourceAgentID:      ts.Metric["source_agent_id"],
					DestinationAgentID: ts.Metric["destination_agent_id"],
					DirectionID:        ts.Metric["direction_id"],
					Timestamp:          tsTime,
				}
				seen[tsMs] = r
				results = append(results, r)
			}

			switch metricName {
			case "netprob_ping_rtt_avg_ms":
				r.AvgRTT = &valFloat
			case "netprob_ping_rtt_min_ms":
				r.MinRTT = &valFloat
			case "netprob_ping_rtt_max_ms":
				r.MaxRTT = &valFloat
			case "netprob_ping_packet_loss_percent":
				r.PacketLoss = &valFloat
			case "netprob_ping_jitter_ms":
				r.Jitter = &valFloat
			case "netprob_ping_packets_sent":
				r.PacketsSent = int(valFloat)
			case "netprob_ping_packets_received":
				r.PacketsReceived = int(valFloat)
			}
		}
	}

	return results, nil
}
