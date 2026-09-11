package mtr

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"netprob/internal/models"

	"github.com/google/uuid"
)

// Execute runs MTR to the given target address with JSON output.
// On Linux, the `mtr` command supports --json for machine-readable output.
func Execute(target string, sourceAgentID, destAgentID, directionID string) (*models.MTRRun, error) {
	run := &models.MTRRun{
		ID:                 uuid.NewString(),
		DirectionID:        directionID,
		SourceAgentID:      sourceAgentID,
		DestinationAgentID: destAgentID,
		Timestamp:          time.Now().UTC(),
		Status:             "error",
	}
	if target == "" || strings.HasPrefix(target, "-") {
		err := fmt.Errorf("invalid mtr target %q", target)
		run.Error = err.Error()
		return run, err
	}
	cmd := exec.Command("mtr", mtrArgs(target, 20)...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		err = fmt.Errorf("mtr failed: %w: %s", err, strings.TrimSpace(string(output)))
		run.Error = err.Error()
		return run, err
	}

	hops, err := parseMTRJSON(output)
	if err != nil {
		err = fmt.Errorf("failed to parse mtr json: %w", err)
		run.Error = err.Error()
		return run, err
	}
	run.Status = "success"
	run.Error = ""
	run.Hops = hops
	return run, nil
}

func mtrArgs(target string, cycles int) []string {
	// Output modes are mutually exclusive in mtr and the last one wins. Keep
	// -j after -r so mtr 0.93 and 0.95 both emit JSON rather than text reports.
	return []string{"-r", "-c", strconv.Itoa(cycles), "-j", target}
}

func parseMTRJSON(output []byte) ([]models.MTRHop, error) {
	decoder := json.NewDecoder(bytes.NewReader(output))
	var mtrJSON mtrJSONOutput
	if err := decoder.Decode(&mtrJSON); err != nil {
		return nil, err
	}
	hubs := mtrJSON.Report.Hubs
	if len(hubs) == 0 {
		hubs = mtrJSON.Hubs
	}
	if len(hubs) == 0 {
		return nil, fmt.Errorf("mtr response contains no hops")
	}

	hops := make([]models.MTRHop, 0, len(hubs))
	for _, h := range hubs {
		hop := models.MTRHop{
			HopNumber:   int(h.Count),
			Host:        h.Host,
			LossPercent: h.Loss,
			Sent:        int(h.Sent),
		}
		if net.ParseIP(h.Host) != nil {
			hop.IP = h.Host
		}
		if h.Avg != nil {
			f := *h.Avg
			hop.AvgMs = &f
		}
		if h.Last != nil {
			f := *h.Last
			hop.LastMs = &f
		}
		if h.Best != nil {
			f := *h.Best
			hop.BestMs = &f
		}
		if h.Worst != nil {
			f := *h.Worst
			hop.WorstMs = &f
		}
		hops = append(hops, hop)
	}
	return hops, nil
}

// mtrJSONOutput represents the JSON report emitted by mtr 0.93 and newer.
type mtrJSONOutput struct {
	Report struct {
		Hubs []mtrHub `json:"hubs"`
	} `json:"report"`
	Hubs []mtrHub `json:"hubs"` // compatibility with flat fixtures/providers
}

type mtrHub struct {
	Count flexibleInt `json:"count"`
	Host  string      `json:"host"`
	Loss  float64     `json:"Loss%"`
	Sent  flexibleInt `json:"Snt"`
	Last  *float64    `json:"Last"`
	Best  *float64    `json:"Best"`
	Worst *float64    `json:"Wrst"`
	Avg   *float64    `json:"Avg"`
}

type flexibleInt int

func (n *flexibleInt) UnmarshalJSON(data []byte) error {
	var value int
	if err := json.Unmarshal(data, &value); err == nil {
		*n = flexibleInt(value)
		return nil
	}
	var text string
	if err := json.Unmarshal(data, &text); err != nil {
		return fmt.Errorf("invalid integer %s", data)
	}
	value, err := strconv.Atoi(text)
	if err != nil {
		return fmt.Errorf("invalid integer %q: %w", text, err)
	}
	*n = flexibleInt(value)
	return nil
}
