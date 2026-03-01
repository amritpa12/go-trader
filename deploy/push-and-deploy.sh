#!/usr/bin/env bash
set -euo pipefail

# ─────────────────────────────────────────────────
# Run FROM YOUR MAC to push changes and deploy
# Usage: bash deploy/push-and-deploy.sh [user@your-server-ip]
# ─────────────────────────────────────────────────

SERVER="${1:-}"
if [ -z "$SERVER" ]; then
    echo "Usage: bash deploy/push-and-deploy.sh user@your-server-ip"
    echo "Example: bash deploy/push-and-deploy.sh root@65.21.xxx.xxx"
    exit 1
fi

echo "=== Pushing to GitHub ==="
git push origin main

echo "=== Deploying to $SERVER ==="
ssh "$SERVER" "bash /opt/go-trader/deploy/update.sh"

echo ""
echo "=== Deployed! ==="
echo "View logs: ssh $SERVER 'journalctl -u go-trader -f'"
