package tools

import (
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type kanbanID struct {
	ID uint `json:"id" jsonschema:"the numeric ID"`
}

type kanbanBodyInput struct {
	Body json.RawMessage `json:"body" jsonschema:"the JSON request body for the board/task"`
}

type createKanbanTaskInput struct {
	BoardID uint            `json:"board_id" jsonschema:"the board to create the task in"`
	Body    json.RawMessage `json:"body" jsonschema:"the JSON task body (title, description, etc.)"`
}

func registerKanban(s *mcp.Server, d Doer) {
	addTool(s, d, &mcp.Tool{
		Name:        "claworc_list_kanban_boards",
		Description: "List Kanban boards.",
	}, func(struct{}) (apiCall, error) {
		return apiCall{Method: "GET", Path: "/kanban/boards"}, nil
	})

	addTool(s, d, &mcp.Tool{
		Name:        "claworc_get_kanban_board",
		Description: "Get a Kanban board with its tasks.",
	}, func(in kanbanID) (apiCall, error) {
		return apiCall{Method: "GET", Path: idPath("/kanban/boards", in.ID, "")}, nil
	})

	addTool(s, d, &mcp.Tool{
		Name:        "claworc_create_kanban_board",
		Description: "Create a Kanban board. Provide the board fields as a JSON body.",
	}, func(in kanbanBodyInput) (apiCall, error) {
		if len(in.Body) == 0 {
			return apiCall{}, fmt.Errorf("body is required")
		}
		return apiCall{Method: "POST", Path: "/kanban/boards", Body: in.Body}, nil
	})

	addTool(s, d, &mcp.Tool{
		Name:        "claworc_get_kanban_task",
		Description: "Get a single Kanban task by ID.",
	}, func(in kanbanID) (apiCall, error) {
		return apiCall{Method: "GET", Path: idPath("/kanban/tasks", in.ID, "")}, nil
	})

	addTool(s, d, &mcp.Tool{
		Name:        "claworc_create_kanban_task",
		Description: "Create a task on a Kanban board. Provide the task fields as a JSON body.",
	}, func(in createKanbanTaskInput) (apiCall, error) {
		if len(in.Body) == 0 {
			return apiCall{}, fmt.Errorf("body is required")
		}
		return apiCall{Method: "POST", Path: idPath("/kanban/boards", in.BoardID, "/tasks"), Body: in.Body}, nil
	})

	addTool(s, d, &mcp.Tool{
		Name:        "claworc_start_kanban_task",
		Description: "Start (dispatch to an agent) a Kanban task.",
	}, func(in kanbanID) (apiCall, error) {
		return apiCall{Method: "POST", Path: idPath("/kanban/tasks", in.ID, "/start")}, nil
	})

	addTool(s, d, &mcp.Tool{
		Name:        "claworc_stop_kanban_task",
		Description: "Stop a running Kanban task.",
	}, func(in kanbanID) (apiCall, error) {
		return apiCall{Method: "POST", Path: idPath("/kanban/tasks", in.ID, "/stop")}, nil
	})
}
