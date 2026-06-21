package tools

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"

	"github.com/gluk-w/claworc/mcp-server/internal/client"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type instanceID struct {
	ID uint `json:"id" jsonschema:"the numeric instance ID"`
}

type createInstanceInput struct {
	DisplayName      string            `json:"display_name" jsonschema:"human-friendly name for the instance"`
	TeamID           *uint             `json:"team_id,omitempty" jsonschema:"team to create the instance in; omit for the caller's default/first managed team"`
	ContainerImage   *string           `json:"container_image,omitempty" jsonschema:"override the agent container image"`
	DefaultModel     string            `json:"default_model,omitempty" jsonschema:"default LLM model identifier"`
	CPURequest       string            `json:"cpu_request,omitempty" jsonschema:"Kubernetes CPU request, e.g. 500m"`
	CPULimit         string            `json:"cpu_limit,omitempty" jsonschema:"Kubernetes CPU limit, e.g. 2"`
	MemoryRequest    string            `json:"memory_request,omitempty" jsonschema:"memory request, e.g. 512Mi"`
	MemoryLimit      string            `json:"memory_limit,omitempty" jsonschema:"memory limit, e.g. 2Gi"`
	StorageHome      string            `json:"storage_home,omitempty" jsonschema:"home volume size, e.g. 10Gi"`
	StorageHomebrew  string            `json:"storage_homebrew,omitempty" jsonschema:"homebrew volume size, e.g. 5Gi"`
	EnabledProviders []uint            `json:"enabled_providers,omitempty" jsonschema:"LLM gateway provider IDs to enable for this instance"`
	EnvVarsSet       map[string]string `json:"env_vars_set,omitempty" jsonschema:"environment variables to set on the agent"`
}

type updateInstanceInput struct {
	ID               uint    `json:"id" jsonschema:"the numeric instance ID"`
	DisplayName      *string `json:"display_name,omitempty" jsonschema:"new display name (admin only)"`
	DefaultModel     *string `json:"default_model,omitempty" jsonschema:"new default model"`
	CPURequest       *string `json:"cpu_request,omitempty" jsonschema:"new CPU request (admin only)"`
	CPULimit         *string `json:"cpu_limit,omitempty" jsonschema:"new CPU limit (admin only)"`
	MemoryRequest    *string `json:"memory_request,omitempty" jsonschema:"new memory request (admin only)"`
	MemoryLimit      *string `json:"memory_limit,omitempty" jsonschema:"new memory limit (admin only)"`
	Timezone         *string `json:"timezone,omitempty" jsonschema:"IANA timezone, e.g. America/New_York"`
	AllowedSourceIPs *string `json:"allowed_source_ips,omitempty" jsonschema:"comma-separated IPs/CIDRs allowed to reach the instance (admin only)"`
	EnabledProviders *[]uint `json:"enabled_providers,omitempty" jsonschema:"replace the enabled LLM provider IDs (admin only)"`
}

type cloneInstanceInput struct {
	ID          uint    `json:"id" jsonschema:"the source instance ID to clone"`
	DisplayName *string `json:"display_name,omitempty" jsonschema:"display name for the cloned instance"`
}

type updateConfigInput struct {
	ID     uint            `json:"id" jsonschema:"the numeric instance ID"`
	Config json.RawMessage `json:"config" jsonschema:"the full OpenClaw config object to write"`
}

type logsInput struct {
	ID   uint   `json:"id" jsonschema:"the numeric instance ID"`
	Tail int    `json:"tail,omitempty" jsonschema:"number of trailing log lines to return (default 100)"`
	Type string `json:"type,omitempty" jsonschema:"log type: openclaw (default), agent, or other supported log stream"`
}

func registerInstances(s *mcp.Server, c *client.Client) {
	addTool(s, c, &mcp.Tool{
		Name:        "claworc_list_instances",
		Description: "List all OpenClaw instances visible to the authenticated user, including live status, resources, and team.",
	}, func(struct{}) (apiCall, error) {
		return apiCall{Method: "GET", Path: "/instances"}, nil
	})

	addTool(s, c, &mcp.Tool{
		Name:        "claworc_get_instance",
		Description: "Get full details for a single instance by ID.",
	}, func(in instanceID) (apiCall, error) {
		return apiCall{Method: "GET", Path: idPath("/instances", in.ID, "")}, nil
	})

	addTool(s, c, &mcp.Tool{
		Name:        "claworc_create_instance",
		Description: "Create a new OpenClaw instance. Requires admin or a team-manager role.",
	}, func(in createInstanceInput) (apiCall, error) {
		if in.DisplayName == "" {
			return apiCall{}, fmt.Errorf("display_name is required")
		}
		return apiCall{Method: "POST", Path: "/instances", Body: in}, nil
	})

	addTool(s, c, &mcp.Tool{
		Name:        "claworc_update_instance",
		Description: "Update an instance's configuration (display name, resources, model, timezone, providers, etc.). Only the provided fields change.",
	}, func(in updateInstanceInput) (apiCall, error) {
		return apiCall{Method: "PUT", Path: idPath("/instances", in.ID, ""), Body: in}, nil
	})

	addTool(s, c, &mcp.Tool{
		Name:        "claworc_delete_instance",
		Description: "Permanently delete an instance and its data. Admin only. This cannot be undone.",
	}, func(in instanceID) (apiCall, error) {
		return apiCall{Method: "DELETE", Path: idPath("/instances", in.ID, "")}, nil
	})

	addTool(s, c, &mcp.Tool{
		Name:        "claworc_start_instance",
		Description: "Start a stopped instance.",
	}, func(in instanceID) (apiCall, error) {
		return apiCall{Method: "POST", Path: idPath("/instances", in.ID, "/start")}, nil
	})

	addTool(s, c, &mcp.Tool{
		Name:        "claworc_stop_instance",
		Description: "Stop a running instance.",
	}, func(in instanceID) (apiCall, error) {
		return apiCall{Method: "POST", Path: idPath("/instances", in.ID, "/stop")}, nil
	})

	addTool(s, c, &mcp.Tool{
		Name:        "claworc_restart_instance",
		Description: "Restart an instance (stop then start).",
	}, func(in instanceID) (apiCall, error) {
		return apiCall{Method: "POST", Path: idPath("/instances", in.ID, "/restart")}, nil
	})

	addTool(s, c, &mcp.Tool{
		Name:        "claworc_clone_instance",
		Description: "Clone an existing instance, including its volumes, into a new instance.",
	}, func(in cloneInstanceInput) (apiCall, error) {
		body := map[string]any{}
		if in.DisplayName != nil {
			body["display_name"] = *in.DisplayName
		}
		return apiCall{Method: "POST", Path: idPath("/instances", in.ID, "/clone"), Body: body}, nil
	})

	addTool(s, c, &mcp.Tool{
		Name:        "claworc_update_instance_image",
		Description: "Pull and roll the instance onto the latest version of its container image.",
	}, func(in instanceID) (apiCall, error) {
		return apiCall{Method: "POST", Path: idPath("/instances", in.ID, "/update-image")}, nil
	})

	addTool(s, c, &mcp.Tool{
		Name:        "claworc_get_instance_config",
		Description: "Get the instance's OpenClaw configuration (JSON).",
	}, func(in instanceID) (apiCall, error) {
		return apiCall{Method: "GET", Path: idPath("/instances", in.ID, "/config")}, nil
	})

	addTool(s, c, &mcp.Tool{
		Name:        "claworc_update_instance_config",
		Description: "Replace the instance's OpenClaw configuration with the provided JSON object.",
	}, func(in updateConfigInput) (apiCall, error) {
		if len(in.Config) == 0 {
			return apiCall{}, fmt.Errorf("config is required")
		}
		return apiCall{Method: "PUT", Path: idPath("/instances", in.ID, "/config"), Body: in.Config}, nil
	})

	addTool(s, c, &mcp.Tool{
		Name:        "claworc_get_instance_stats",
		Description: "Get live resource usage statistics (CPU, memory, etc.) for an instance.",
	}, func(in instanceID) (apiCall, error) {
		return apiCall{Method: "GET", Path: idPath("/instances", in.ID, "/stats")}, nil
	})

	addTool(s, c, &mcp.Tool{
		Name:        "claworc_get_instance_ssh_status",
		Description: "Get the SSH connection and tunnel status for an instance.",
	}, func(in instanceID) (apiCall, error) {
		return apiCall{Method: "GET", Path: idPath("/instances", in.ID, "/ssh-status")}, nil
	})

	addTool(s, c, &mcp.Tool{
		Name:        "claworc_get_instance_logs",
		Description: "Fetch recent log lines for an instance (non-following snapshot).",
	}, func(in logsInput) (apiCall, error) {
		q := url.Values{}
		q.Set("follow", "false")
		tail := in.Tail
		if tail <= 0 {
			tail = 100
		}
		q.Set("tail", strconv.Itoa(tail))
		if in.Type != "" {
			q.Set("type", in.Type)
		}
		return apiCall{Method: "GET", Path: idPath("/instances", in.ID, "/logs"), Query: q, SSE: true}, nil
	})

	addTool(s, c, &mcp.Tool{
		Name:        "claworc_list_instance_providers",
		Description: "List the LLM providers available to a specific instance.",
	}, func(in instanceID) (apiCall, error) {
		return apiCall{Method: "GET", Path: idPath("/instances", in.ID, "/providers")}, nil
	})
}
