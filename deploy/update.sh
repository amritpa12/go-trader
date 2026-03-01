#!/usr/bin/env bash
set -euo pipefail

# ─────────────────────────────────────────────────
# Pull latest changes and redeploy go-trader
# Run as root: bash /opt/go-trader/deploy/update.sh
# ─────────────────────────────────────────────────

INSTALL_DIR="/opt/go-trader"
cd "$INSTALL_DIR"

echo "=== Pulling latest changes ==="
git pull origin main

echo "=== Updating Python dependencies ==="
uv sync

echo "=== Checking for Go changes ==="
if git diff HEAD~1 --name-only | grep -q '^scheduler/.*\.go$'; then
    echo "Go files changed — rebuilding binary..."
    cd scheduler
    go build -o ../go-trader .
    cd ..
else
    echo "No Go file changes — skipping rebuild"
fi

echo "=== Setting permissions ==="
chown -R go-trader:go-trader "$INSTALL_DIR"

echo "=== Restarting service ==="
systemctl restart go-trader

echo "=== Done! Checking status ==="
sleep 2
systemctl status go-trader --no-pager -l | head -20
