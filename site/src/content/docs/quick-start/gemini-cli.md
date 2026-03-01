---
title: Gemini CLI Setup
sidebar:
  order: 3
---

Configure gopls-mcp with Gemini CLI.

---

### Setup gopls-mcp for Your Project

Ensure [gemini-cli](https://github.com/google-gemini/gemini-cli) is installed, and run command below to add mcp server.

```bash
gemini mcp add gopls-mcp gopls-mcp
```

You will see the similar log below if succeed.

```
MCP server "gopls-mcp" added to project settings. (stdio)
```

---

### Configure Project Instructions (GEMINI.md)

Gemini needs specific instructions to know when to use the semantic tools. Run this command in your project root to add the rules:

```bash
curl -sL https://gopls-mcp.org/gopls-mcp.prompt >> GEMINI.md
```

### Verify gopls-mcp

Inside gemini-cli, run mcp list command to verify `gopls-mcp` is available.

```
$ gemini mcp list
```

If the tool is successfully added, you will see gopls-mcp in the server list.

```
Configured MCP servers:

✓ gopls-mcp: gopls-mcp  (stdio) - Connected
```
