package tools

import (
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type createTeamInput struct {
	Name        string `json:"name" jsonschema:"team name"`
	Description string `json:"description,omitempty" jsonschema:"optional team description"`
}

type updateTeamInput struct {
	ID          uint   `json:"id" jsonschema:"the team ID"`
	Name        string `json:"name,omitempty" jsonschema:"new team name"`
	Description string `json:"description,omitempty" jsonschema:"new team description"`
}

type teamID struct {
	ID uint `json:"id" jsonschema:"the team ID"`
}

type setTeamMemberInput struct {
	ID     uint   `json:"id" jsonschema:"the team ID"`
	UserID uint   `json:"user_id" jsonschema:"the user ID to add or update"`
	Role   string `json:"role" jsonschema:"team role: manager or user (pass empty string to remove the member)"`
}

type removeTeamMemberInput struct {
	ID     uint `json:"id" jsonschema:"the team ID"`
	UserID uint `json:"user_id" jsonschema:"the user ID to remove"`
}

type setTeamProvidersInput struct {
	ID          uint   `json:"id" jsonschema:"the team ID"`
	ProviderIDs []uint `json:"provider_ids" jsonschema:"the full set of global LLM provider IDs to allow for this team"`
}

func registerTeams(s *mcp.Server, d Doer) {
	addTool(s, d, &mcp.Tool{
		Name:        "claworc_list_teams",
		Description: "List teams available to the caller (admins see all).",
	}, func(struct{}) (apiCall, error) {
		return apiCall{Method: "GET", Path: "/teams"}, nil
	})

	addTool(s, d, &mcp.Tool{
		Name:        "claworc_create_team",
		Description: "Create a new team (admin only).",
	}, func(in createTeamInput) (apiCall, error) {
		if in.Name == "" {
			return apiCall{}, fmt.Errorf("name is required")
		}
		return apiCall{Method: "POST", Path: "/teams", Body: in}, nil
	})

	addTool(s, d, &mcp.Tool{
		Name:        "claworc_update_team",
		Description: "Update a team's name or description (admin only).",
	}, func(in updateTeamInput) (apiCall, error) {
		body := map[string]any{}
		if in.Name != "" {
			body["name"] = in.Name
		}
		if in.Description != "" {
			body["description"] = in.Description
		}
		return apiCall{Method: "PUT", Path: idPath("/teams", in.ID, ""), Body: body}, nil
	})

	addTool(s, d, &mcp.Tool{
		Name:        "claworc_delete_team",
		Description: "Delete a team (admin only).",
	}, func(in teamID) (apiCall, error) {
		return apiCall{Method: "DELETE", Path: idPath("/teams", in.ID, "")}, nil
	})

	addTool(s, d, &mcp.Tool{
		Name:        "claworc_list_team_members",
		Description: "List the members of a team (admin only).",
	}, func(in teamID) (apiCall, error) {
		return apiCall{Method: "GET", Path: idPath("/teams", in.ID, "/members")}, nil
	})

	addTool(s, d, &mcp.Tool{
		Name:        "claworc_set_team_member",
		Description: "Add or update a team membership (admin only). Pass role=\"\" to remove the member.",
	}, func(in setTeamMemberInput) (apiCall, error) {
		return apiCall{Method: "POST", Path: idPath("/teams", in.ID, "/members"), Body: map[string]any{"user_id": in.UserID, "role": in.Role}}, nil
	})

	addTool(s, d, &mcp.Tool{
		Name:        "claworc_remove_team_member",
		Description: "Remove a member from a team (admin only).",
	}, func(in removeTeamMemberInput) (apiCall, error) {
		return apiCall{Method: "DELETE", Path: fmt.Sprintf("/teams/%d/members/%d", in.ID, in.UserID)}, nil
	})

	addTool(s, d, &mcp.Tool{
		Name:        "claworc_get_team_providers",
		Description: "Get the LLM provider whitelist for a team (admin only).",
	}, func(in teamID) (apiCall, error) {
		return apiCall{Method: "GET", Path: idPath("/teams", in.ID, "/providers")}, nil
	})

	addTool(s, d, &mcp.Tool{
		Name:        "claworc_set_team_providers",
		Description: "Replace the LLM provider whitelist for a team (admin only).",
	}, func(in setTeamProvidersInput) (apiCall, error) {
		return apiCall{Method: "PUT", Path: idPath("/teams", in.ID, "/providers"), Body: map[string]any{"provider_ids": in.ProviderIDs}}, nil
	})
}
