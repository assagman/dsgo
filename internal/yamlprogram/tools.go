package yamlprogram

import (
	"fmt"
	"os"

	"github.com/assagman/dsgo/internal/core"
	"github.com/assagman/dsgo/internal/mcp"
)

func (b *Builder) buildToolSources() error {
	for name, src := range b.spec.ToolSources {
		tools, err := b.buildToolSource(name, src)
		if err != nil {
			return err
		}
		b.toolSources[name] = tools
	}
	return nil
}

func (b *Builder) buildToolSource(name string, src ToolSource) ([]core.Tool, error) {
	switch src.Kind {
	case "builtin":
		var out []core.Tool
		for _, tname := range src.Tools {
			tool, err := builtinTool(tname)
			if err != nil {
				return nil, fmt.Errorf("tool_sources.%s: %w", name, err)
			}
			out = append(out, *tool)
		}
		return out, nil
	case "mcp":
		client, err := b.buildMCPClient(src)
		if err != nil {
			return nil, fmt.Errorf("tool_sources.%s: %w", name, err)
		}
		if err := client.Initialize(b.ctx); err != nil {
			return nil, fmt.Errorf("tool_sources.%s: initialize mcp client: %w", name, err)
		}
		return client.GetTools(), nil
	default:
		return nil, fmt.Errorf("tool_sources.%s: unsupported kind %q", name, src.Kind)
	}
}

func (b *Builder) buildMCPClient(src ToolSource) (*mcp.Client, error) {
	apiKey := src.APIKey
	if apiKey == "" && src.APIKeyEnv != "" {
		apiKey = os.Getenv(src.APIKeyEnv)
	}
	// Backward-compat convenience: fall back to standard env vars by type.
	if apiKey == "" {
		switch src.Type {
		case "exa":
			apiKey = os.Getenv("EXA_API_KEY")
		case "jina":
			apiKey = os.Getenv("JINA_API_KEY")
		case "tavily":
			apiKey = os.Getenv("TAVILY_API_KEY")
		}
	}

	switch src.Type {
	case "exa":
		c, err := mcp.NewExaClient(apiKey)
		if err != nil {
			return nil, err
		}
		return c, nil
	case "jina":
		c, err := mcp.NewJinaClient(apiKey)
		if err != nil {
			return nil, err
		}
		return c, nil
	case "tavily":
		c, err := mcp.NewTavilyClient(apiKey)
		if err != nil {
			return nil, err
		}
		return c, nil
	case "filesystem":
		if len(src.AllowedDirs) == 0 {
			return nil, fmt.Errorf("filesystem tool source requires allowed_dirs")
		}
		c, err := mcp.NewFilesystemClient(src.AllowedDirs[0])
		if err != nil {
			return nil, err
		}
		return c, nil
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
		c, err := mcp.NewClient(mcp.ClientConfig{Transport: transport})
		if err != nil {
			return nil, err
		}
		return c, nil
	case "custom":
		if src.URL == "" {
			return nil, fmt.Errorf("custom tool source requires url")
		}
		transport := mcp.NewHTTPTransport(src.URL, apiKey)
		c, err := mcp.NewClient(mcp.ClientConfig{Transport: transport})
		if err != nil {
			return nil, err
		}
		return c, nil
	default:
		return nil, fmt.Errorf("unsupported mcp type %q", src.Type)
	}
}
