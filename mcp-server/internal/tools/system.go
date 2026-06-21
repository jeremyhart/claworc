package tools

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func registerSystem(s *mcp.Server, d Doer) {
	addTool(s, d, &mcp.Tool{
		Name:        "claworc_orchestrator_status",
		Description: "Get the container backend (Docker/Kubernetes) status and diagnostics (admin only).",
	}, func(struct{}) (apiCall, error) {
		return apiCall{Method: "GET", Path: "/orchestrator/status"}, nil
	})

	addTool(s, d, &mcp.Tool{
		Name:        "claworc_get_audit_logs",
		Description: "Get the control-plane audit log (admin only).",
	}, func(struct{}) (apiCall, error) {
		return apiCall{Method: "GET", Path: "/audit-logs"}, nil
	})

	addTool(s, d, &mcp.Tool{
		Name:        "claworc_list_shared_folders",
		Description: "List shared folders configured in the control plane.",
	}, func(struct{}) (apiCall, error) {
		return apiCall{Method: "GET", Path: "/shared-folders"}, nil
	})
}
