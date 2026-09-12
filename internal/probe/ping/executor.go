package ping

import (
	"context"
	"fmt"
	"math"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"netprob/internal/models"
)

// Execute runs a ping to the given target address using the system ping command.
// On Linux, it uses raw ICMP sockets (may require CAP_NET_RAW).
// count defaults to 5 packets if <= 0.
// interval is seconds between packets.
func Execute(target string, count int, interval int) (*models.PingResult, error) {
	return ExecuteContext(context.Background(), target, count, interval)
}

// ExecuteContext runs ping and terminates the child process when ctx is done.
func ExecuteContext(ctx context.Context, target string, count int, interval int) (*models.PingResult, error) {
	if target == "" || strings.HasPrefix(target, "-") {
		return nil, fmt.Errorf("invalid ping target %q", target)
	}
	if count <= 0 {
		count = 5
	}
	if interval <= 0 {
		interval = 1
	}

	// Detect OS to use the right ping flags
	timeout := time.Duration(count*(interval+3)+5) * time.Second
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := buildPingCommand(ctx, target, count, interval)
	output, err := cmd.CombinedOutput()
	return interpretPingOutput(string(output), target, err, ctx.Err())
}

func interpretPingOutput(output, target string, commandErr, contextErr error) (*models.PingResult, error) {
	result := parsePingOutput(string(output), target)
	if commandErr != nil {
		if contextErr != nil {
			return nil, fmt.Errorf("ping timed out or was canceled: %w", contextErr)
		}
		// iputils ping exits with status 1 when every packet is lost, while still
		// producing a valid summary that must be persisted as 100% loss.
		if result.PacketsSent > 0 {
			return result, nil
		}
		return nil, fmt.Errorf("ping failed: %w: %s", commandErr, output)
	}

	if result.PacketsSent == 0 {
		return nil, fmt.Errorf("ping returned no packet summary")
	}
	return result, nil
}

func buildPingCommand(ctx context.Context, target string, count, interval int) *exec.Cmd {
	// Linux ping syntax
	args := []string{
		"-c", strconv.Itoa(count),
		"-i", strconv.Itoa(interval),
		"-W", "3", // deadline per packet
		target,
	}
	return exec.CommandContext(ctx, "ping", args...)
}

// PingResult fields
type pingStats struct {
	packetsTransmitted int
	packetsReceived    int
	packetLoss         float64
	rttMin             float64
	rttAvg             float64
	rttMax             float64
	rttJitter          float64
}

func parsePingOutput(output string, target string) *models.PingResult {
	stats := &pingStats{}

	// Parse packets transmitted/received and loss
	// Linux: "5 packets transmitted, 5 received, 0% packet loss"
	// macOS: "5 packets transmitted, 5 packets received, 0.0% packet loss"
	packetRe := regexp.MustCompile(`(\d+) packets? transmitted, (\d+) (?:packets? )?received, ([\d.]+)% packet loss`)
	if matches := packetRe.FindStringSubmatch(output); len(matches) == 4 {
		stats.packetsTransmitted, _ = strconv.Atoi(matches[1])
		stats.packetsReceived, _ = strconv.Atoi(matches[2])
		stats.packetLoss, _ = strconv.ParseFloat(matches[3], 64)
	}

	// Parse rtt line: "rtt min/avg/max/mdev = 1.100/1.200/1.300/0.050 ms"
	// Or on macOS: "round-trip min/avg/max/stddev = 1.100/1.200/1.300/0.050 ms"
	rttRe := regexp.MustCompile(`(?:rtt|round-trip|round_trip) min/avg/max/(?:mdev|stddev) = ([\d.]+)/([\d.]+)/([\d.]+)/([\d.]+) ms`)
	hasRTT := false
	if matches := rttRe.FindStringSubmatch(output); len(matches) == 5 {
		hasRTT = true
		stats.rttMin, _ = strconv.ParseFloat(matches[1], 64)
		stats.rttAvg, _ = strconv.ParseFloat(matches[2], 64)
		stats.rttMax, _ = strconv.ParseFloat(matches[3], 64)
		stats.rttJitter, _ = strconv.ParseFloat(matches[4], 64)
	}

	now := time.Now().UTC()
	result := &models.PingResult{
		Timestamp:          now,
		SourceAgentID:      "", // filled by caller
		DestinationAgentID: "", // filled by caller
		DirectionID:        "", // filled by caller
		PacketsSent:        stats.packetsTransmitted,
		PacketsReceived:    stats.packetsReceived,
	}

	result.PacketLoss = &stats.packetLoss
	if hasRTT {
		result.MinRTT = &stats.rttMin
		result.AvgRTT = &stats.rttAvg
		result.MaxRTT = &stats.rttMax
	}
	if hasRTT && !math.IsNaN(stats.rttJitter) {
		result.Jitter = &stats.rttJitter
	}

	return result
}
