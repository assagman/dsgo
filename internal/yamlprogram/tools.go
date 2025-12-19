package yamlprogram

import (
	"fmt"
	"os"

	"github.com/assagman/dsgo/internal/core"
	"github.com/assagman/dsgo/internal/mcp"
)

func (b *Builder) buildToolSources() error {
	ts := b.spec.ToolSources

	// Build builtin tools
	if len(ts.Builtin) > 0 {
		var tools []core.Tool
		for _, tname := range ts.Builtin {
			tool, err := builtinTool(tname)
			if err != nil {
				return fmt.Errorf("tool_sources.builtin: %w", err)
			}
			tools = append(tools, *tool)
		}
		b.toolSources["builtin"] = tools
	}

	// Build MCP tool sources
	for mcpType, src := range ts.MCP {
		tools, err := b.buildMCPToolSource(mcpType, src)
		if err != nil {
			return err
		}
		b.toolSources[mcpType] = tools
	}

	return nil
}

func (b *Builder) buildMCPToolSource(mcpType string, src MCPToolSource) ([]core.Tool, error) {
	client, err := b.buildMCPClient(mcpType, src)
	if err != nil {
		return nil, fmt.Errorf("tool_sources.mcp.%s: %w", mcpType, err)
	}
	if err := client.Initialize(b.ctx); err != nil {
		return nil, fmt.Errorf("tool_sources.mcp.%s: initialize: %w", mcpType, err)
	}
	return client.GetTools(), nil
}

func (b *Builder) buildMCPClient(mcpType string, src MCPToolSource) (*mcp.Client, error) {
	apiKey := src.APIKey
	if apiKey == "" && src.APIKeyEnv != "" {
		apiKey = os.Getenv(src.APIKeyEnv)
	}
	// Fall back to standard env vars by type.
	if apiKey == "" {
		switch mcpType {
		case "exa":
			apiKey = os.Getenv("EXA_API_KEY")
		case "jina":
			apiKey = os.Getenv("JINA_API_KEY")
		case "tavily":
			apiKey = os.Getenv("TAVILY_API_KEY")
		}
	}

	switch mcpType {
	case "exa":
		return mcp.NewExaClient(apiKey)
	case "jina":
		return mcp.NewJinaClient(apiKey)
	case "tavily":
		return mcp.NewTavilyClient(apiKey)
	case "filesystem":
		if len(src.AllowedDirs) == 0 {
			return nil, fmt.Errorf("allowed_dirs is required")
		}
		return mcp.NewFilesystemClient(src.AllowedDirs[0])
	case "shell":
		root := ""
		if len(src.AllowedDirs) > 0 {
			root = src.AllowedDirs[0]
		}
		server, err := mcp.NewShellServer(mcp.ShellServerConfig{RootDir: root})
		if err != nil {
			return nil, err
		}
		transport, err := mcp.NewLocalTransport(server)
		if err != nil {
			return nil, err
		}
		return mcp.NewClient(mcp.ClientConfig{Transport: transport})
	case "custom":
		if src.URL == "" {
			return nil, fmt.Errorf("url is required")
		}
		transport := mcp.NewHTTPTransport(src.URL, apiKey)
		return mcp.NewClient(mcp.ClientConfig{Transport: transport})
	default:
		return nil, fmt.Errorf("unsupported MCP type %q", mcpType)
	}
}
