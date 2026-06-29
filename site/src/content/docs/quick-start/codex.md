---
title: Codex CLI Setup
sidebar:
  order: 4
---

Configure gopls-mcp with Codex CLI.

---

## Install via Plugin

```bash
codex plugin marketplace add https://github.com/xieyuschen/gopls-mcp.git
codex plugin add gopls-mcp
```

That's it. The plugin handles everything automatically:

- Downloads and installs the `gopls-mcp` binary to `~/.local/bin/`
- Registers the MCP server
- Injects the routing skill (which tools to use and when)
- Adds a session-start hook that activates the skill each session

Running `codex plugin update gopls-mcp` upgrades the binary and rules together.

### Verify

```bash
codex mcp list
```

You should see `gopls-mcp` with status `enabled`.
