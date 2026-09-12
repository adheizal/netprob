package ping

import (
	"errors"
	"testing"
)

func TestParsePingOutputSuccess(t *testing.T) {
	result := parsePingOutput(`5 packets transmitted, 5 received, 0% packet loss
rtt min/avg/max/mdev = 1.100/1.200/1.300/0.050 ms`, "192.0.2.1")
	if result.PacketsSent != 5 || result.PacketsReceived != 5 || result.PacketLoss == nil || *result.PacketLoss != 0 {
		t.Fatalf("packet summary = %#v", result)
	}
	if result.AvgRTT == nil || *result.AvgRTT != 1.2 || result.Jitter == nil || *result.Jitter != 0.05 {
		t.Fatalf("RTT summary = %#v", result)
	}
}

func TestParsePingOutputTotalLossHasNoRTT(t *testing.T) {
	result := parsePingOutput(`5 packets transmitted, 0 received, 100% packet loss, time 4091ms`, "192.0.2.1")
	if result.PacketsSent != 5 || result.PacketsReceived != 0 || result.PacketLoss == nil || *result.PacketLoss != 100 {
		t.Fatalf("packet summary = %#v", result)
	}
	if result.MinRTT != nil || result.AvgRTT != nil || result.MaxRTT != nil || result.Jitter != nil {
		t.Fatalf("total-loss RTT values must be nil: %#v", result)
	}
}

func TestParsePingOutputMacOS(t *testing.T) {
	result := parsePingOutput(`5 packets transmitted, 4 packets received, 20.0% packet loss
round-trip min/avg/max/stddev = 2.000/3.000/4.000/0.500 ms`, "192.0.2.1")
	if result.PacketsSent != 5 || result.PacketsReceived != 4 || result.PacketLoss == nil || *result.PacketLoss != 20 {
		t.Fatalf("packet summary = %#v", result)
	}
	if result.AvgRTT == nil || *result.AvgRTT != 3 {
		t.Fatalf("RTT summary = %#v", result)
	}
}

func TestInterpretPingOutputAcceptsTotalLossExitStatus(t *testing.T) {
	result, err := interpretPingOutput(
		"5 packets transmitted, 0 received, 100% packet loss, time 4091ms",
		"192.0.2.1", errors.New("exit status 1"), nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.PacketLoss == nil || *result.PacketLoss != 100 || result.PacketsReceived != 0 {
		t.Fatalf("total-loss result = %#v", result)
	}
}

func TestInterpretPingOutputRejectsCommandFailureWithoutSummary(t *testing.T) {
	if _, err := interpretPingOutput("permission denied", "192.0.2.1", errors.New("exit status 2"), nil); err == nil {
		t.Fatal("command failure without a packet summary was accepted")
	}
}
