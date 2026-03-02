---
title: Cursor Setup
sidebar:
  order: 5
---

Configure gopls-mcp with Cursor IDE.

---

### Setup gopls-mcp for Your Project

Ensure [Cursor](https://cursor.sh) is installed, and gopls-mcp is already installed on your system.

---

### Configure MCP Server

Cursor uses JSON configuration files to manage MCP servers. You can configure gopls-mcp at either the **project level** or **global level**.

#### Option 1: Project-Level Configuration (Recommended)

Create or edit `.cursor/mcp.json` in your project root:

```bash
# Create the directory if it doesn't exist
mkdir -p .cursor
```

Then add the following configuration to `.cursor/mcp.json`:

```json
{
  "mcpServers": {
    "gopls-mcp": {
      "command": "gopls-mcp"
    }
  }
}
```

> **If the file already exists**, add the `"gopls-mcp"` entry to the existing `mcpServers` object.

#### Option 2: Global Configuration

Configure gopls-mcp for all projects by creating `~/.cursor/mcp.json`:

```bash
# Create the directory if it doesn't exist
mkdir -p ~/.cursor
```

Then add the same configuration to `~/.cursor/mcp.json`.

> **If the file already exists**, add the `"gopls-mcp"` entry to the existing `mcpServers` object.

> **Note**: Project-level configuration takes precedence over global configuration.

---

### Configure Project Instructions (.cursorrules)

Cursor needs specific instructions to know when to use the semantic tools. Run this command in your project root:

```bash
curl -sL https://gopls-mcp.org/gopls-mcp.prompt >> .cursorrules
```

This creates a `.cursorrules` file in your project directory with instructions for Cursor to use gopls-mcp tools effectively.

---

### Verify gopls-mcp

1. **Open Cursor Settings**:
   - Click the gear icon in the top-right corner
   - Go to **Tools & MCP**

2. **Enable gopls-mcp Server**:
   - You should see `gopls-mcp` in the server list
   - **Click the toggle** to enable it (it may be disabled by default)
   - The status indicator should turn **green**

### Auto-Approve MCP Tool Calls

By default, Cursor asks for confirmation before each MCP tool call. To automatically approve all gopls-mcp tool calls:

1. Go to **Settings → Agents → MCP Allowlists**
2. Add `gopls-mcp:*` to the allowlist

This allows Cursor to execute all gopls-mcp tools without manual confirmation.

> **Security Note**: Only configure MCP allowlists in trusted projects. Keep Cursor updated to the latest version to ensure security patches are applied.

3. **Test with Agent (Composer)**:
   - Open a new **Agent (Composer)** conversation
   - Try a Go-related task like:
     - "Find all implementations of the `Handler` interface"
     - "Show me the call hierarchy for the `ServeHTTP` method"
   - Cursor should automatically use gopls-mcp tools

> **Important**: MCP tools are **only available in Agent (Composer) mode**. The regular **Ask (Chat)** mode does not support MCP calls. If you see a message like "gopls-mcp MCP calls are limited in Ask mode", switch to Agent (Composer) to use gopls-mcp tools.

---

### Troubleshooting

**Server not showing up?**
1. Check that the configuration file is valid JSON
2. Try toggling the server on/off in Cursor Settings
3. Restart Cursor to reload the configuration

**Server status is red?**
1. Click "View Logs" next to the server to see error messages
2. Verify `gopls-mcp` is installed and in your PATH:
   ```bash
   which gopls-mcp
   gopls-mcp --version
   ```

**Agent not using gopls-mcp tools?**
1. Make sure you're using **Agent (Composer)**, not regular Chat
2. Check that `.cursorrules` exists in your project root
3. Try explicitly mentioning Go-related tasks

---

