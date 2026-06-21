package tools

import (
	"github.com/gluk-w/claworc/mcp-server/internal/client"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type createBackupInput struct {
	ID    uint     `json:"id" jsonschema:"the instance ID to back up"`
	Paths []string `json:"paths,omitempty" jsonschema:"specific paths to include; omit for the default set"`
	Note  string   `json:"note,omitempty" jsonschema:"optional note describing the backup"`
}

type backupID struct {
	BackupID string `json:"backup_id" jsonschema:"the backup ID"`
}

type restoreBackupInput struct {
	BackupID   string `json:"backup_id" jsonschema:"the backup ID to restore"`
	InstanceID uint   `json:"instance_id" jsonschema:"the target instance ID to restore into"`
}

func registerBackups(s *mcp.Server, c *client.Client) {
	addTool(s, c, &mcp.Tool{
		Name:        "claworc_create_backup",
		Description: "Start a backup for an instance.",
	}, func(in createBackupInput) (apiCall, error) {
		body := map[string]any{}
		if len(in.Paths) > 0 {
			body["paths"] = in.Paths
		}
		if in.Note != "" {
			body["note"] = in.Note
		}
		return apiCall{Method: "POST", Path: idPath("/instances", in.ID, "/backups"), Body: body}, nil
	})

	addTool(s, c, &mcp.Tool{
		Name:        "claworc_list_instance_backups",
		Description: "List backups for a specific instance.",
	}, func(in instanceID) (apiCall, error) {
		return apiCall{Method: "GET", Path: idPath("/instances", in.ID, "/backups")}, nil
	})

	addTool(s, c, &mcp.Tool{
		Name:        "claworc_list_backups",
		Description: "List all backups across instances the caller can access.",
	}, func(struct{}) (apiCall, error) {
		return apiCall{Method: "GET", Path: "/backups"}, nil
	})

	addTool(s, c, &mcp.Tool{
		Name:        "claworc_get_backup",
		Description: "Get details for a single backup.",
	}, func(in backupID) (apiCall, error) {
		return apiCall{Method: "GET", Path: "/backups/" + in.BackupID}, nil
	})

	addTool(s, c, &mcp.Tool{
		Name:        "claworc_delete_backup",
		Description: "Delete a backup.",
	}, func(in backupID) (apiCall, error) {
		return apiCall{Method: "DELETE", Path: "/backups/" + in.BackupID}, nil
	})

	addTool(s, c, &mcp.Tool{
		Name:        "claworc_restore_backup",
		Description: "Restore a backup into a target instance. Requires admin or a team-manager role.",
	}, func(in restoreBackupInput) (apiCall, error) {
		return apiCall{Method: "POST", Path: "/backups/" + in.BackupID + "/restore", Body: map[string]any{"instance_id": in.InstanceID}}, nil
	})

	addTool(s, c, &mcp.Tool{
		Name:        "claworc_list_backup_schedules",
		Description: "List configured backup schedules.",
	}, func(struct{}) (apiCall, error) {
		return apiCall{Method: "GET", Path: "/backup-schedules"}, nil
	})
}
