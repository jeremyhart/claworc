package tools

import (
	"encoding/json"
	"fmt"

	"github.com/gluk-w/claworc/mcp-server/internal/client"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type updateSettingsInput struct {
	Settings json.RawMessage `json:"settings" jsonschema:"a JSON object of settings keys to update, e.g. {\"default_cpu_limit\":\"2\",\"brave_api_key\":\"...\"}"`
}

func registerSettings(s *mcp.Server, c *client.Client) {
	addTool(s, c, &mcp.Tool{
		Name:        "claworc_get_settings",
		Description: "Get the global control-plane settings (admin only). API keys are returned masked.",
	}, func(struct{}) (apiCall, error) {
		return apiCall{Method: "GET", Path: "/settings"}, nil
	})

	addTool(s, c, &mcp.Tool{
		Name:        "claworc_update_settings",
		Description: "Update global control-plane settings (admin only). Provide an object with only the keys you want to change.",
	}, func(in updateSettingsInput) (apiCall, error) {
		if len(in.Settings) == 0 {
			return apiCall{}, fmt.Errorf("settings object is required")
		}
		return apiCall{Method: "PUT", Path: "/settings", Body: in.Settings}, nil
	})
}
