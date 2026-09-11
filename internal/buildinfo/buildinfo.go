package buildinfo

import "fmt"

// These values are replaced through -ldflags when release artifacts are built.
var (
	Version = "dev"
	Commit  = "unknown"
	Date    = "unknown"
)

func String() string {
	return fmt.Sprintf("netprob %s (commit %s, built %s)", Version, Commit, Date)
}
