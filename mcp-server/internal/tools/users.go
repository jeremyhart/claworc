package tools

import (
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type createUserInput struct {
	Username string `json:"username" jsonschema:"login username"`
	Password string `json:"password" jsonschema:"initial password"`
	Role     string `json:"role" jsonschema:"user role: admin or user"`
}

type userID struct {
	UserID uint `json:"user_id" jsonschema:"the numeric user ID"`
}

type updateRoleInput struct {
	UserID uint   `json:"user_id" jsonschema:"the numeric user ID"`
	Role   string `json:"role" jsonschema:"new role: admin or user"`
}

type resetPasswordInput struct {
	UserID   uint   `json:"user_id" jsonschema:"the numeric user ID"`
	Password string `json:"password" jsonschema:"the new password"`
}

type setUserInstancesInput struct {
	UserID      uint   `json:"user_id" jsonschema:"the numeric user ID"`
	InstanceIDs []uint `json:"instance_ids" jsonschema:"the full set of instance IDs the user should be assigned to"`
}

func registerUsers(s *mcp.Server, d Doer) {
	addTool(s, d, &mcp.Tool{
		Name:        "claworc_list_users",
		Description: "List all users (admin only).",
	}, func(struct{}) (apiCall, error) {
		return apiCall{Method: "GET", Path: "/users"}, nil
	})

	addTool(s, d, &mcp.Tool{
		Name:        "claworc_create_user",
		Description: "Create a new user (admin only).",
	}, func(in createUserInput) (apiCall, error) {
		if in.Username == "" || in.Password == "" {
			return apiCall{}, fmt.Errorf("username and password are required")
		}
		return apiCall{Method: "POST", Path: "/users", Body: in}, nil
	})

	addTool(s, d, &mcp.Tool{
		Name:        "claworc_delete_user",
		Description: "Delete a user (admin only).",
	}, func(in userID) (apiCall, error) {
		return apiCall{Method: "DELETE", Path: idPath("/users", in.UserID, "")}, nil
	})

	addTool(s, d, &mcp.Tool{
		Name:        "claworc_update_user_role",
		Description: "Change a user's role to admin or user (admin only).",
	}, func(in updateRoleInput) (apiCall, error) {
		return apiCall{Method: "PUT", Path: idPath("/users", in.UserID, "/role"), Body: map[string]string{"role": in.Role}}, nil
	})

	addTool(s, d, &mcp.Tool{
		Name:        "claworc_reset_user_password",
		Description: "Reset a user's password (admin only).",
	}, func(in resetPasswordInput) (apiCall, error) {
		return apiCall{Method: "POST", Path: idPath("/users", in.UserID, "/reset-password"), Body: map[string]string{"password": in.Password}}, nil
	})

	addTool(s, d, &mcp.Tool{
		Name:        "claworc_get_user_teams",
		Description: "List the teams a user belongs to (admin only).",
	}, func(in userID) (apiCall, error) {
		return apiCall{Method: "GET", Path: idPath("/users", in.UserID, "/teams")}, nil
	})

	addTool(s, d, &mcp.Tool{
		Name:        "claworc_get_user_instances",
		Description: "List the instances assigned to a user (admin only).",
	}, func(in userID) (apiCall, error) {
		return apiCall{Method: "GET", Path: idPath("/users", in.UserID, "/instances")}, nil
	})

	addTool(s, d, &mcp.Tool{
		Name:        "claworc_set_user_instances",
		Description: "Replace the set of instances a user is assigned to (admin only).",
	}, func(in setUserInstancesInput) (apiCall, error) {
		return apiCall{Method: "PUT", Path: idPath("/users", in.UserID, "/instances"), Body: map[string]any{"instance_ids": in.InstanceIDs}}, nil
	})
}
