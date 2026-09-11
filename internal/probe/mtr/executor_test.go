package mtr

import (
	"reflect"
	"testing"
)

func TestMTRArgsKeepsJSONAsFinalOutputMode(t *testing.T) {
	want := []string{"-r", "-c", "20", "-j", "192.0.2.1"}
	if got := mtrArgs("192.0.2.1", 20); !reflect.DeepEqual(got, want) {
		t.Fatalf("mtrArgs() = %#v, want %#v", got, want)
	}
}

func TestParseMTRJSONVersions(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name: "mtr 0.93 quoted hop number",
			input: `{
  "report": {
    "hubs": [{
      "count": "1",
      "host": "192.0.2.1",
      "Loss%": 0.00,
      "Snt": 20,
      "Last": 0.21,
      "Avg": 0.25,
      "Best": 0.20,
      "Wrst": 0.40
    }]
  }
}`,
		},
		{
			name: "mtr 0.95 numeric hop number",
			input: `{
  "report": {
    "hubs": [{
      "count": 1,
      "host": "gateway.example",
      "Loss%": 5.0,
      "Snt": 20,
      "Last": 1.1,
      "Avg": 1.2,
      "Best": 1.0,
      "Wrst": 1.5
    }]
  }
}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hops, err := parseMTRJSON([]byte(tt.input))
			if err != nil {
				t.Fatalf("parseMTRJSON() error = %v", err)
			}
			if len(hops) != 1 || hops[0].HopNumber != 1 || hops[0].Sent != 20 {
				t.Fatalf("unexpected hops: %#v", hops)
			}
			if hops[0].AvgMs == nil || hops[0].BestMs == nil || hops[0].WorstMs == nil || hops[0].LastMs == nil {
				t.Fatalf("latency fields were not decoded: %#v", hops[0])
			}
		})
	}
}

func TestParseMTRJSONRejectsTextAndEmptyReports(t *testing.T) {
	for _, input := range []string{
		"Start: report text",
		`{"report":{"hubs":[]}}`,
	} {
		if _, err := parseMTRJSON([]byte(input)); err == nil {
			t.Fatalf("parseMTRJSON(%q) unexpectedly succeeded", input)
		}
	}
}
