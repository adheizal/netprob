package protocol

// MessageType constants for WebSocket communication
const (
	TypeHello   = "hello"
	TypeHelloOK = "hello_ok"
	TypeJob     = "job"
	TypeResult  = "result"
	TypePing    = "ping"
	TypePong    = "pong"
	TypeError   = "error"
)

// Envelope wraps all WebSocket messages with a type discriminator.
type Envelope struct {
	Type    string `json:"type"`
	Payload any    `json:"payload"`
}
