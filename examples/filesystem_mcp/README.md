# Filesystem MCP Example

Demonstrates using the official MCP Filesystem server with DSGo's ReAct agent.

## Prerequisites

- Node.js with `npx` OR Bun with `bunx`
- OpenAI API key

## Usage

```bash
export OPENAI_API_KEY=your-key-here
cd examples/filesystem_mcp
go run main.go
```

## What It Does

1. Creates a Filesystem MCP client using stdio transport
2. Connects to `@modelcontextprotocol/server-filesystem` via npx/bunx
3. Creates a ReAct agent with filesystem tools
4. Analyzes Go files in the current directory

## Available Filesystem Tools

The official MCP filesystem server provides these tools:

- `read_file` - Read file contents
- `read_multiple_files` - Read multiple files concurrently
- `write_file` - Create or overwrite files
- `edit_file` - Line-based file editing with diff preview
- `create_directory` - Create directories
- `list_directory` - List directory contents
- `directory_tree` - Get recursive directory tree
- `move_file` - Move or rename files/directories
- `search_files` - Search files with patterns and exclusions
- `get_file_info` - Get file metadata (size, permissions, etc.)
- `list_allowed_directories` - View allowed directories

## Security

The filesystem server only has access to directories specified when creating the client. By default, it's restricted to the current working directory.
