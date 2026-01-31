#!/bin/bash
# gopls-mcp installer for Linux and macOS
# Usage: curl -sSL https://gopls-mcp.org/install.sh | bash
#        GOPLS_MCP_VERSION=v1.0.0 curl -sSL https://gopls-mcp.org/install.sh | bash

set -e

# Configuration
REPO="xieyuschen/gopls-mcp"
NAME="gopls-mcp"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

error() {
    echo -e "${RED}[ERROR]${NC} $1"
    exit 1
}

# Detect OS and architecture
detect_os_arch() {
    OS="$(uname -s)"
    ARCH="$(uname -m)"

    case "$OS" in
        Linux*)     OS='linux';;
        Darwin*)    OS='darwin';;
        *)          error "Unsupported OS: $OS";;
    esac

    case "$ARCH" in
        x86_64*)    ARCH='amd64';;
        aarch64*)   ARCH='arm64';;
        arm64*)     ARCH='arm64';;
        *)          error "Unsupported architecture: $ARCH";;
    esac

    info "Detected OS: $OS, Architecture: $ARCH"
}

# Get latest release version
get_latest_version() {
    if [ -n "$GOPLS_MCP_VERSION" ]; then
        VERSION="$GOPLS_MCP_VERSION"
        info "Using specified version: $VERSION"
        return
    fi

    local API_URL="https://api.github.com/repos/$REPO/releases/latest"
    info "Fetching latest release from GitHub API..."
    echo "  -> GET $API_URL"

    local response
    response=$(curl -sS "$API_URL" 2>&1) || {
        error "Failed to connect to GitHub API. Please check your internet connection."
    }

    # Check if we got a valid JSON response
    if [[ ! "$response" =~ "tag_name" ]]; then
        error "Invalid response from GitHub API. Response: $response"
    fi

    # Check for rate limiting
    if echo "$response" | grep -q "API rate limit exceeded"; then
        error "GitHub API rate limit exceeded. Try: GOPLS_MCP_VERSION=v1.0.0 curl -sSL https://gopls-mcp.org/install.sh | bash"
    fi

    VERSION=$(echo "$response" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')

    if [ -z "$VERSION" ]; then
        error "Failed to extract version from GitHub response. Response: $response"
    fi

    info "Latest version: $VERSION"
}

# Determine install directory ($HOME/.local/bin)
get_install_dir() {
    INSTALL_DIR="$HOME/.local/bin"

    # Create directory if it doesn't exist
    if [ ! -d "$INSTALL_DIR" ]; then
        info "Creating directory: $INSTALL_DIR"
        mkdir -p "$INSTALL_DIR"
    fi

    info "Install directory: $INSTALL_DIR"
}

# Download and install
download_and_install() {
    # Remove 'v' prefix from VERSION for filename (GoReleaser convention)
    # e.g., v1.0.0 -> 1.0.0
    CLEAN_VERSION="${VERSION#v}"
    FILENAME="${NAME}_${CLEAN_VERSION}_${OS}_${ARCH}"
    URL="https://github.com/${REPO}/releases/download/${VERSION}/${FILENAME}.tar.gz"
    TEMP_FILE=$(mktemp).tar.gz

    info "Downloading release binary..."
    echo "  -> GET $URL"

    if ! curl -fSL "$URL" -o "$TEMP_FILE"; then
        error "Failed to download. Verify the release exists at: https://github.com/${REPO}/releases/tag/${VERSION}"
    fi

    # Verify the file was downloaded and is not empty
    if [ ! -s "$TEMP_FILE" ]; then
        error "Downloaded file is empty. The release may not exist for $OS $ARCH."
    fi

    local file_size=$(du -h "$TEMP_FILE" | cut -f1)
    info "Downloaded $file_size, extracting..."

    if ! tar -xzf "$TEMP_FILE" -C "$INSTALL_DIR" "$NAME" 2>/dev/null; then
        rm -f "$TEMP_FILE"
        error "Failed to extract archive. The downloaded file may be corrupted."
    fi

    chmod +x "$INSTALL_DIR/$NAME"
    rm -f "$TEMP_FILE"

    info "Installed: $INSTALL_DIR/$NAME"
}

# Verify installation
verify() {
    if command -v "$NAME" &> /dev/null; then
        VERSION_OUTPUT=$("$NAME" --version 2>&1 || true)
        info "Successfully installed $NAME!"
        info "$VERSION_OUTPUT"
        info "Installation location: $INSTALL_DIR/$NAME"
    else
        warn "Installation completed, but $NAME is not in PATH"
        warn "Add $HOME/.local/bin to your PATH:"
        warn "  echo 'export PATH=\"\$HOME/.local/bin:\$PATH\"' >> ~/.bashrc  # or ~/.zshrc"
        warn "  source ~/.bashrc  # or ~/.zshrc"
    fi
}

# Main execution
main() {
    echo ""
    echo "gopls-mcp Installer"
    echo "==================="
    echo ""

    detect_os_arch
    get_latest_version
    get_install_dir
    download_and_install
    verify

    echo ""
    info "Installation complete!"
}

main
