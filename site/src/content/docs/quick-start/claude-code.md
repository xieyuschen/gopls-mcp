---
title: Claude Code Setup
sidebar:
  order: 2
---

Configure gopls-mcp with Claude Code.

---

## Install via Plugin

```
/plugin marketplace add https://github.com/xieyuschen/gopls-mcp.git
/plugin install gopls-mcp
```

That's it. The plugin handles everything automatically:

- Downloads and installs the `gopls-mcp` binary to `~/.local/bin/`
- Registers the MCP server
- Injects the routing skill (which tools to use and when)
- Adds a session-start hook that activates the skill each session

Running `/plugin update gopls-mcp` upgrades the binary and rules together.

### Verify

```
/mcp
```

You should see `gopls-mcp · ✔ connected`.

---

## Manual Install (without plugin)

Use this only if you cannot use the plugin system.

### 1. Install binary

**Linux / macOS:**
```bash
curl -sSL https://gopls-mcp.org/install.sh | bash
```

**Windows (PowerShell):**
```powershell
irm https://gopls-mcp.org/install.ps1 | iex
```

### 2. Register the MCP server

```bash
claude mcp add gopls-mcp -- gopls-mcp
```

### 3. Add routing rules to CLAUDE.md

```bash
curl -sL https://gopls-mcp.org/gopls-mcp.prompt >> CLAUDE.md
```

### 4. Verify

```
/mcp
❯ gopls-mcp · ✔ connected
```
