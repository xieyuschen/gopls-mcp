# gopls-mcp

Give your AI Agent the compiler's brain, not a text searcher.

Documentation: https://gopls-mcp.org

gopls-mcp delivers lightning-fast, language-level analysis directly to your LLM.
Unlike standard retrieval tools that flood the context window with irrelevant text, this tool performs surgical code navigation.

By providing only the scientifically accurate definitions and references, it maximizes your model's attention span and keeps the reasoning chain pure. Zero noise, absolute structural accuracy, and instant response times.

## Install

### Claude Code (plugin — recommended)

```
/plugin marketplace add https://github.com/xieyuschen/gopls-mcp.git
/plugin install gopls-mcp
```

The plugin automatically installs the binary and injects the routing skill — no manual setup required.

### Codex (plugin — recommended)

```bash
codex plugin marketplace add https://github.com/xieyuschen/gopls-mcp.git
codex plugin add gopls-mcp
```

### Manual install (all clients)

**Linux / macOS:**
```bash
curl -sSL https://gopls-mcp.org/install.sh | bash
```

**Windows (PowerShell):**
```powershell
irm https://gopls-mcp.org/install.ps1 | iex
```

Then follow the per-client setup at https://gopls-mcp.org/quick-start.

## Contribute

The project is actively developing, and feel free to raise PRs or issues if you find anything to improve.
AI generated code will also be accepted but do remember to narrow the change to a specific feature for reviewer to quickly review them.

*Disclaimer*: gopls-mcp is a fork of [gopls](https://tip.golang.org/gopls/) and is a community-driven project. It is not an official Go team product and is not affiliated with or endorsed by Google LLC. This project is licensed under the same BSD license as its [upstream](https://go.googlesource.com/tools) source.
