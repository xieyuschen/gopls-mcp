---
title: Claude Code Setup
sidebar:
  order: 2
---

Configure gopls-mcp with Claude Code.

---

### Setup gopls-mcp for Your Project

Ensure [claude code](https://code.claude.com/docs/en/overview) is installed, and run command below to add mcp server into claude code.

```bash
claude mcp add gopls-mcp -- gopls-mcp
```

You will see the similar log below if succeed.

```
Added stdio MCP server gopls-mcp with command: gopls-mcp to local config
File modified: /home/xieyuschen/.claude.json
[project: /home/xieyuschen/codespace/gopls-mcproot]
```

---

### Configure Project Instructions (CLAUDE.md)

Claude needs specific instructions to know when to use the semantic tools. Run this command in your project root to add the rules (safe for both new and existing projects):
The script creates the file if it doesn't exist; preserves content if it does)

```bash
curl -sL https://gopls-mcp.org/gopls-mcp.prompt >> CLAUDE.md
```

### Verify gopls-mcp

Inside claude code, run `/mcp` command to verify `gopls-mcp` is availble.

```
(claude)> /mcp
```

If the tool is successfully added, you will see similiar output below:

```
 Manage MCP servers
 1 server

   Local MCPs (/home/xieyuschen/.claude.json
   [project: /home/xieyuschen/codespace/gopls-mcproot])
 ❯ gopls-mcp · ✔ connected

 https://code.claude.com/docs/en/mcp for help
```

---
