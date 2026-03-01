#!/usr/bin/env bash
set -euo pipefail

# ─────────────────────────────────────────────────
# Hetzner server setup for go-trader
# Run as root: bash setup-server.sh
# ─────────────────────────────────────────────────

INSTALL_DIR="/opt/go-trader"
GO_VERSION="1.23.6"

echo "=== Installing system dependencies ==="
apt-get update
apt-get install -y git python3 python3-venv python3-pip curl build-essential

echo "=== Installing Go $GO_VERSION ==="
if ! command -v go &>/dev/null || [[ "$(go version)" != *"$GO_VERSION"* ]]; then
    curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz" -o /tmp/go.tar.gz
    rm -rf /usr/local/go
    tar -C /usr/local -xzf /tmp/go.tar.gz
    rm /tmp/go.tar.gz
    ln -sf /usr/local/go/bin/go /usr/local/bin/go
    ln -sf /usr/local/go/bin/gofmt /usr/local/bin/gofmt
fi
echo "Go: $(go version)"

echo "=== Installing uv (Python package manager) ==="
if ! command -v uv &>/dev/null; then
    curl -LsSf https://astral.sh/uv/install.sh | sh
    export PATH="$HOME/.local/bin:$PATH"
fi

echo "=== Creating go-trader system user ==="
if ! id go-trader &>/dev/null; then
    useradd --system --no-create-home --shell /usr/sbin/nologin go-trader
fi

echo "=== Cloning repository ==="
if [ ! -d "$INSTALL_DIR" ]; then
    echo "Enter your GitHub repo URL (e.g. https://github.com/YOU/go-trader.git):"
    read -r REPO_URL
    git clone "$REPO_URL" "$INSTALL_DIR"
else
    echo "Directory $INSTALL_DIR already exists, pulling latest..."
    cd "$INSTALL_DIR"
    git pull origin main
fi

cd "$INSTALL_DIR"

echo "=== Installing Python dependencies ==="
uv sync

echo "=== Building Go binary ==="
cd scheduler
go build -o ../go-trader .
cd ..

echo "=== Setting up config ==="
if [ ! -f scheduler/config.json ]; then
    if [ -f scheduler/config.example.json ]; then
        cp scheduler/config.example.json scheduler/config.json
        echo "IMPORTANT: Edit scheduler/config.json with your settings!"
    else
        echo "WARNING: No config.example.json found. You need to create scheduler/config.json"
    fi
else
    echo "Config already exists at scheduler/config.json"
fi

echo "=== Setting permissions ==="
chown -R go-trader:go-trader "$INSTALL_DIR"
chmod 750 "$INSTALL_DIR"
chmod 700 "$INSTALL_DIR/scheduler"

echo "=== Installing systemd service ==="
cp go-trader.service /etc/systemd/system/go-trader.service
systemctl daemon-reload
systemctl enable go-trader

echo ""
echo "========================================="
echo "  Setup complete!"
echo "========================================="
echo ""
echo "Next steps:"
echo "  1. Edit config:    nano $INSTALL_DIR/scheduler/config.json"
echo "  2. Start service:  systemctl start go-trader"
echo "  3. Check status:   systemctl status go-trader"
echo "  4. View logs:      journalctl -u go-trader -f"
echo ""
