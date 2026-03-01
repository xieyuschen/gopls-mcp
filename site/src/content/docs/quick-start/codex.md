---
title: Codex CLI Setup
sidebar:
  order: 4
---

Configure gopls-mcp with Codex CLI.

---

### Setup gopls-mcp for Your Project

Ensure [codex-cli](https://github.com/openai/codex) is installed, and run command below to add mcp server.

```bash
codex mcp add gopls-mcp -- gopls-mcp
```

You will see the similar log below if succeed.

```
Added global MCP server 'gopls-mcp'.
```

---

### Configure Project Instructions (AGENTS.md)

Codex needs specific instructions to know when to use the semantic tools. Run this command in your project root to add the rules:

```bash
curl -sL https://gopls-mcp.org/gopls-mcp.prompt >> AGENTS.md
```

### Verify gopls-mcp

Inside codex-cli, run mcp list command to verify `gopls-mcp` is available.

```
$ codex mcp list
Name       Command    Args  Env  Cwd  Status   Auth       
gopls-mcp  gopls-mcp  -     -    -    enabled  Unsupported
```

If the tool is successfully added, you will see gopls-mcp in the server list.

---

Enter `codex` and run command `/mcp`:

```
/mcp

🔌  MCP Tools

  • gopls-mcp
    • Status: enabled
    • Auth: Unsupported
    • Command: gopls-mcp
    ...
```