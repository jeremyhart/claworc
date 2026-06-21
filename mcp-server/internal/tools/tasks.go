package tools

import (
	"github.com/gluk-w/claworc/mcp-server/internal/client"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type taskIDInput struct {
	ID string `json:"id" jsonschema:"the task ID"`
}

func registerTasks(s *mcp.Server, c *client.Client) {
	addTool(s, c, &mcp.Tool{
		Name:        "claworc_list_tasks",
		Description: "List long-running background tasks (instance create/restart/clone, image updates, backups, skill deploys).",
	}, func(struct{}) (apiCall, error) {
		return apiCall{Method: "GET", Path: "/tasks"}, nil
	})

	addTool(s, c, &mcp.Tool{
		Name:        "claworc_get_task",
		Description: "Get the status of a single background task by ID.",
	}, func(in taskIDInput) (apiCall, error) {
		return apiCall{Method: "GET", Path: "/tasks/" + in.ID}, nil
	})

	addTool(s, c, &mcp.Tool{
		Name:        "claworc_cancel_task",
		Description: "Cancel a running background task.",
	}, func(in taskIDInput) (apiCall, error) {
		return apiCall{Method: "POST", Path: "/tasks/" + in.ID + "/cancel"}, nil
	})
}
