#!/bin/bash
# SessionStart hook for gopls-mcp plugin.
#
# Injects a brief activation notice into Claude's context and reloads skills
# so the gopls-mcp routing skill is guaranteed to be active. The full routing
# protocol (which tools to use, constraints, error handling) lives in the
# gopls-mcp SKILL.md; this hook only supplies the one-liner reminder that
# keeps the tool top-of-mind without duplicating the skill content.

set -euo pipefail

printf '%s' '{
  "hookSpecificOutput": {
    "hookEventName": "SessionStart",
    "additionalContext": "gopls-mcp is active. Use go_definition, go_implementation, go_symbol_references, go_get_call_hierarchy, go_get_dependency_graph, go_dryrun_rename_symbol for semantic Go analysis — these are type-aware and cannot be replaced by grep. See the gopls-mcp skill for the full routing protocol.",
    "reloadSkills": true
  }
}'
