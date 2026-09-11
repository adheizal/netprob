package models

// Probe types
const (
	ProbePing = "ping"
	ProbeMTR  = "mtr"
)

// Job represents a probe job dispatched from controller to agent
type Job struct {
	ID          string         `json:"id"`
	Type        string         `json:"type"` // "ping" or "mtr"
	Direction   JobDirection   `json:"direction"`
	Config      map[string]any `json:"config,omitempty"`
	ScheduledAt string         `json:"scheduled_at"`
}

type JobDirection struct {
	SourceAgentID      string `json:"source_agent_id"`
	DestinationAgentID string `json:"destination_agent_id"`
	TargetAddress      string `json:"target_address"`
	LinkID             string `json:"link_id"`
	DirectionID        string `json:"direction_id"`
}

// JobResult is returned by the agent after executing a job
type JobResult struct {
	JobID      string      `json:"job_id"`
	Status     string      `json:"status"` // "success", "error"
	Error      string      `json:"error,omitempty"`
	PingResult *PingResult `json:"ping_result,omitempty"`
	MTRRun     *MTRRun     `json:"mtr_run,omitempty"`
}

// AgentHello is sent by the agent upon WebSocket connection
type AgentHello struct {
	Type           string        `json:"type"` // "hello"
	AgentID        string        `json:"agent_id"`
	Hostname       string        `json:"hostname"`
	Version        string        `json:"version"`
	Addresses      []string      `json:"addresses"`
	PrimaryAddress string        `json:"primary_address"`
	Capabilities   []string      `json:"capabilities"`
	Location       AgentLocation `json:"location"`
	Token          string        `json:"token"`
}

// ControllerMessage wraps messages from controller to agent
type ControllerMessage struct {
	Type  string `json:"type"` // "job", "pong", "hello_ok"
	Job   *Job   `json:"job,omitempty"`
	Error string `json:"error,omitempty"`
}

// AgentMessage wraps messages from agent to controller
type AgentMessage struct {
	Type   string      `json:"type"` // "hello", "result", "ping"
	Hello  *AgentHello `json:"hello,omitempty"`
	Result *JobResult  `json:"result,omitempty"`
}
