package tools

import (
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type deploySkillInput struct {
	Slug        string `json:"slug" jsonschema:"the skill slug to deploy"`
	InstanceIDs []uint `json:"instance_ids" jsonschema:"instance IDs to deploy the skill to"`
	Source      string `json:"source,omitempty" jsonschema:"deploy source, e.g. library or clawhub"`
	Version     string `json:"version,omitempty" jsonschema:"optional skill version"`
}

func registerSkills(s *mcp.Server, d Doer) {
	addTool(s, d, &mcp.Tool{
		Name:        "claworc_list_skills",
		Description: "List skills available in the library.",
	}, func(struct{}) (apiCall, error) {
		return apiCall{Method: "GET", Path: "/skills"}, nil
	})

	addTool(s, d, &mcp.Tool{
		Name:        "claworc_deploy_skill",
		Description: "Deploy a library skill to one or more instances. The caller must manage each target instance's team.",
	}, func(in deploySkillInput) (apiCall, error) {
		if in.Slug == "" {
			return apiCall{}, fmt.Errorf("slug is required")
		}
		if len(in.InstanceIDs) == 0 {
			return apiCall{}, fmt.Errorf("at least one instance_id is required")
		}
		body := map[string]any{"instance_ids": in.InstanceIDs}
		if in.Source != "" {
			body["source"] = in.Source
		}
		if in.Version != "" {
			body["version"] = in.Version
		}
		return apiCall{Method: "POST", Path: fmt.Sprintf("/skills/%s/deploy", in.Slug), Body: body}, nil
	})
}
