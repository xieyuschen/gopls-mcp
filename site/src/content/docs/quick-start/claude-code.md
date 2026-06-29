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
