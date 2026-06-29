#!/bin/bash
# gopls-mcp plugin launcher
#
# Design rationale — why this script exists:
#
#   The Claude Code / Codex plugin system has no PostInstall or PostUpdate
#   lifecycle hook. There is therefore no dedicated moment to download or
#   upgrade the gopls-mcp binary after a plugin install or update. The only
#   reliable hook we have is the MCP server's own startup, so this script
#   acts as both installer and launcher: it checks whether the binary is
#   present and at the right version, downloads from GitHub Releases if not,
#   then execs the binary. On a warm path the check is a fast string compare
#   and adds no perceptible latency.
#
# Update mechanism — how version upgrades propagate:
#
#   EXPECTED_VERSION is hardcoded here. When a new gopls-mcp release is cut,
#   this value is bumped and the change is committed to the plugin repo.
#   Running `/plugin update` performs a git pull of the plugin repo, which
#   delivers the new EXPECTED_VERSION. On the next session start this script
#   detects that the installed binary (`gopls-mcp -version`) does not match
#   EXPECTED_VERSION and downloads the correct release tarball.
#
# Binary location — why $HOME/.local/bin:
#
#   The binary lives in the user's own PATH directory, not inside
#   ${CLAUDE_PLUGIN_ROOT}. The plugin root is an internal directory managed
#   by Claude Code / Codex whose layout and lifetime are implementation
#   details we should not depend on. $HOME/.local/bin is a stable,
#   user-owned location that survives plugin reinstalls and host tool updates.

set -euo pipefail

EXPECTED_VERSION="1.0.5"
BINARY="${HOME}/.local/bin/gopls-mcp"
REPO="xieyuschen/gopls-mcp"

needs_install() {
    [ ! -f "${BINARY}" ] && return 0
    local current
    current=$("${BINARY}" -version 2>/dev/null | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' | head -1 || echo "")
    [ "${current}" != "${EXPECTED_VERSION}" ]
}

install_binary() {
    local os arch filename url tmpdir

    case "$(uname -s)" in
        Linux*)  os=linux ;;
        Darwin*) os=darwin ;;
        *)       echo "[gopls-mcp] Unsupported OS: $(uname -s)" >&2; exit 1 ;;
    esac

    case "$(uname -m)" in
        x86_64*)         arch=amd64 ;;
        aarch64*|arm64*) arch=arm64 ;;
        *)               echo "[gopls-mcp] Unsupported arch: $(uname -m)" >&2; exit 1 ;;
    esac

    filename="gopls-mcp_${EXPECTED_VERSION}_${os}_${arch}.tar.gz"
    url="https://github.com/${REPO}/releases/download/v${EXPECTED_VERSION}/${filename}"

    tmpdir=$(mktemp -d)
    trap 'rm -rf "${tmpdir}"' EXIT

    echo "[gopls-mcp] Downloading v${EXPECTED_VERSION} (${os}/${arch})..." >&2

    if ! curl -fsSL "${url}" -o "${tmpdir}/${filename}"; then
        echo "[gopls-mcp] Failed to download from ${url}" >&2
        exit 1
    fi

    mkdir -p "${HOME}/.local/bin"
    tar -xzf "${tmpdir}/${filename}" -C "${tmpdir}" gopls-mcp
    mv "${tmpdir}/gopls-mcp" "${BINARY}"
    chmod +x "${BINARY}"

    echo "[gopls-mcp] Installed v${EXPECTED_VERSION} to ${BINARY}" >&2
}

if needs_install; then
    install_binary
fi

exec "${BINARY}" "$@"
