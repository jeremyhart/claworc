// Package tools re-exports the public surface of the internal/tools package
// so that external callers (e.g. the control-plane embedding) can import and
// use the Doer interface and Register function without violating the internal
// package restriction.
package tools

import (
	internaltool "github.com/gluk-w/claworc/mcp-server/internal/tools"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Result is the normalised response from a Doer call.
type Result = internaltool.Result

// Doer executes an API call and returns the raw response. Authentication is
// the Doer's responsibility (bearer header for HTTP; replayed request context
// in-process).
type Doer = internaltool.Doer

// Register adds every Claworc tool to s, bound to d.
func Register(s *mcp.Server, d Doer) {
	internaltool.Register(s, d)
}
