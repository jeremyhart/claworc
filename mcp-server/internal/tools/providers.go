package tools

import (
	"encoding/json"
	"fmt"

	"github.com/gluk-w/claworc/mcp-server/internal/client"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type createProviderInput struct {
	Name     string          `json:"name" jsonschema:"display name for the provider"`
	Key      string          `json:"key,omitempty" jsonschema:"unique provider key/slug"`
	Provider string          `json:"provider,omitempty" jsonschema:"catalog provider key (e.g. openai, anthropic), optional"`
	BaseURL  string          `json:"base_url,omitempty" jsonschema:"API base URL"`
	APIType  string          `json:"api_type,omitempty" jsonschema:"API type, e.g. openai or anthropic"`
	APIKey   string          `json:"api_key,omitempty" jsonschema:"the secret API key for the upstream provider"`
	Models   json.RawMessage `json:"models,omitempty" jsonschema:"array of model objects to expose"`
}

type updateProviderInput struct {
	ID      uint            `json:"id" jsonschema:"the provider ID"`
	Name    string          `json:"name,omitempty" jsonschema:"new display name"`
	BaseURL string          `json:"base_url,omitempty" jsonschema:"new API base URL"`
	APIType string          `json:"api_type,omitempty" jsonschema:"new API type"`
	APIKey  string          `json:"api_key,omitempty" jsonschema:"new API key (leave empty to keep existing)"`
	Models  json.RawMessage `json:"models,omitempty" jsonschema:"replacement array of model objects"`
}

type providerID struct {
	ID uint `json:"id" jsonschema:"the provider ID"`
}

func registerProviders(s *mcp.Server, c *client.Client) {
	addTool(s, c, &mcp.Tool{
		Name:        "claworc_list_providers",
		Description: "List configured LLM gateway providers (admin only).",
	}, func(struct{}) (apiCall, error) {
		return apiCall{Method: "GET", Path: "/llm/providers"}, nil
	})

	addTool(s, c, &mcp.Tool{
		Name:        "claworc_create_provider",
		Description: "Create a new LLM gateway provider (admin only).",
	}, func(in createProviderInput) (apiCall, error) {
		if in.Name == "" {
			return apiCall{}, fmt.Errorf("name is required")
		}
		return apiCall{Method: "POST", Path: "/llm/providers", Body: in}, nil
	})

	addTool(s, c, &mcp.Tool{
		Name:        "claworc_update_provider",
		Description: "Update an existing LLM gateway provider (admin only).",
	}, func(in updateProviderInput) (apiCall, error) {
		return apiCall{Method: "PUT", Path: idPath("/llm/providers", in.ID, ""), Body: in}, nil
	})

	addTool(s, c, &mcp.Tool{
		Name:        "claworc_delete_provider",
		Description: "Delete an LLM gateway provider (admin only).",
	}, func(in providerID) (apiCall, error) {
		return apiCall{Method: "DELETE", Path: idPath("/llm/providers", in.ID, "")}, nil
	})

	addTool(s, c, &mcp.Tool{
		Name:        "claworc_sync_provider_models",
		Description: "Refresh the available model list for a provider from upstream (admin only).",
	}, func(in providerID) (apiCall, error) {
		return apiCall{Method: "POST", Path: idPath("/llm/providers", in.ID, "/sync")}, nil
	})

	addTool(s, c, &mcp.Tool{
		Name:        "claworc_get_usage_stats",
		Description: "Get aggregated LLM gateway usage statistics (tokens, cost, requests).",
	}, func(struct{}) (apiCall, error) {
		return apiCall{Method: "GET", Path: "/llm/usage/stats"}, nil
	})

	addTool(s, c, &mcp.Tool{
		Name:        "claworc_get_provider_catalog",
		Description: "List the provider catalog (known providers and their default settings) from claworc.com.",
	}, func(struct{}) (apiCall, error) {
		return apiCall{Method: "GET", Path: "/llm/catalog"}, nil
	})
}
