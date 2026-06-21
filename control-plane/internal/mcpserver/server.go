package mcpserver

import (
	"net/http"

	"github.com/gluk-w/claworc/control-plane/internal/handlers"
	"github.com/gluk-w/claworc/mcp-server/tools"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// NewHandler returns an http.Handler that serves MCP over Streamable HTTP.
//
// Each new MCP session captures the inbound request's Authorization header and
// binds it to an in-process Doer that replays requests through router. Because
// all replayed requests re-run RequireAuth with the same bearer token, role
// enforcement is identical to a direct external API call — there is no
// privilege bypass.
func NewHandler(router http.Handler) http.Handler {
	return mcp.NewStreamableHTTPHandler(func(req *http.Request) *mcp.Server {
		authHeader := req.Header.Get("Authorization")

		version := handlers.BuildDate
		if version == "" {
			version = "dev"
		}

		server := mcp.NewServer(&mcp.Implementation{
			Name:    "claworc",
			Version: version,
		}, nil)

		doer := &inProcessDoer{
			router:     router,
			authHeader: authHeader,
		}

		tools.Register(server, doer)
		return server
	}, nil)
}
