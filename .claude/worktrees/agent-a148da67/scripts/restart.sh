#!/bin/bash
# Restart all Polymr services
set -e

UID_NUM=$(id -u)

echo "Rebuilding Go router..."
cd /Users/pg/Desktop/personal-polymr
go build -o bin/polymr ./cmd/polymr

echo "Rebuilding Next.js frontend..."
cd /Users/pg/Desktop/personal-polymr/web
npm run build --silent

echo "Restarting services..."
launchctl bootout gui/$UID_NUM /Users/pg/Library/LaunchAgents/com.polymr.router.plist 2>/dev/null || true
launchctl bootout gui/$UID_NUM /Users/pg/Library/LaunchAgents/com.polymr.frontend.plist 2>/dev/null || true

# Kill any stragglers
lsof -ti :3000 2>/dev/null | xargs kill -9 2>/dev/null || true
lsof -ti :8080 2>/dev/null | xargs kill -9 2>/dev/null || true
sleep 1

launchctl bootstrap gui/$UID_NUM /Users/pg/Library/LaunchAgents/com.polymr.frontend.plist
launchctl bootstrap gui/$UID_NUM /Users/pg/Library/LaunchAgents/com.polymr.router.plist
sleep 3

echo "Checking services..."
launchctl list | grep polymr
echo ""
curl -s https://personal-api.polymr.io/health
echo ""
echo "Done."
