"""Regression checks for the distributed plugin's host-specific contracts."""

import json
from pathlib import Path
import subprocess
import unittest


ROOT = Path(__file__).resolve().parent


class PluginContractTest(unittest.TestCase):
    def test_session_start_outputs(self):
        script = ROOT / "scripts/on_session_start.sh"
        for host, args in (("codex", ["--codex"]), ("claude", [])):
            with self.subTest(host=host):
                output = subprocess.check_output([str(script), *args], cwd="/tmp")
                fields = json.loads(output)["hookSpecificOutput"]
                self.assertEqual(fields["hookEventName"], "SessionStart")
                self.assertIn("gopls-mcp is active", fields["additionalContext"])
                if host == "codex":
                    self.assertEqual(set(fields), {"hookEventName", "additionalContext"})
                else:
                    self.assertIs(fields["reloadSkills"], True)

    def test_codex_hook_selects_codex_output(self):
        hooks = json.loads((ROOT / "hooks/codex-hooks.json").read_text())
        command = hooks["hooks"]["SessionStart"][0]["hooks"][0]["command"]
        self.assertIn("${PLUGIN_ROOT}", command)
        self.assertTrue(command.endswith(" --codex"))

    def test_skill_documents_locator_wrapper(self):
        skill = (ROOT / "skills/gopls-mcp/SKILL.md").read_text()
        self.assertIn('"locator": {', skill)
        self.assertIn('"symbol_name": "Start"', skill)
        self.assertIn('"context_file": "/absolute/path/to/server.go"', skill)


if __name__ == "__main__":
    unittest.main()
