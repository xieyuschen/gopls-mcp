---
title: Gemini CLI Setup
sidebar:
  order: 3
---

Configure gopls-mcp with Gemini CLI.

---

### 1. Install binary

**Linux / macOS:**
```bash
curl -sSL https://gopls-mcp.org/install.sh | bash
```

**Windows (PowerShell):**
```powershell
irm https://gopls-mcp.org/install.ps1 | iex
```

### 2. Setup gopls-mcp for Your Project

Ensure [gemini-cli](https://github.com/google-gemini/gemini-cli) is installed, and run the command below to add the MCP server.

```bash
gemini mcp add gopls-mcp gopls-mcp
```

You will see the similar log below if succeed.

```
MCP server "gopls-mcp" added to project settings. (stdio)
```

---

### 3. Configure Project Instructions (GEMINI.md)

Gemini needs specific instructions to know when to use the semantic tools. Run this command in your project root to add the rules:

```bash
curl -sL https://gopls-mcp.org/gopls-mcp.prompt >> GEMINI.md
```

### 4. Verify gopls-mcp

Inside gemini-cli, run mcp list command to verify `gopls-mcp` is available.

```
$ gemini mcp list
```

If the tool is successfully added, you will see gopls-mcp in the server list.

```
Configured MCP servers:

✓ gopls-mcp: gopls-mcp  (stdio) - Connected
```
