package generator

import (
	"encoding/json"
)

// MCPServer represents an MCP server configuration for generation.
type MCPServer struct {
	Name    string
	Enabled bool
	Command string
	Args    []string
}

// GenerateMCPConfig produces the contents of .claude/settings.json.
func GenerateMCPConfig(servers []MCPServer) string {
	mcpServers := make(map[string]interface{})
	for _, srv := range servers {
		if srv.Enabled {
			mcpServers[srv.Name] = map[string]interface{}{
				"command": srv.Command,
				"args":    srv.Args,
			}
		}
	}

	config := map[string]interface{}{
		"mcpServers": mcpServers,
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return "{}"
	}
	return string(data)
}
