#!/bin/bash
# Uninstall Polymr background services (macOS launchd or Linux systemd)
set -e

OS="$(uname -s)"

uninstall_macos() {
    local AGENTS_DIR="$HOME/Library/LaunchAgents"
    local UID_NUM
    UID_NUM=$(id -u)

    echo "Stopping services..."
    launchctl bootout "gui/$UID_NUM/com.winston.router" 2>/dev/null || true
    launchctl bootout "gui/$UID_NUM/com.winston.frontend" 2>/dev/null || true

    echo "Removing plist files..."
    rm -f "$AGENTS_DIR/com.winston.router.plist"
    rm -f "$AGENTS_DIR/com.winston.frontend.plist"

    # Kill any stragglers on known ports
    lsof -ti :3000 2>/dev/null | xargs kill -9 2>/dev/null || true
    lsof -ti :8080 2>/dev/null | xargs kill -9 2>/dev/null || true

    echo ""
    echo "Services uninstalled."
    echo "Log files remain at ~/Library/Logs/winston-*.log (remove manually if desired)."
}

uninstall_linux() {
    echo "Stopping and disabling services..."
    sudo systemctl stop winston-router winston-frontend 2>/dev/null || true
    sudo systemctl disable winston-router winston-frontend 2>/dev/null || true

    echo "Removing service files..."
    sudo rm -f /etc/systemd/system/winston-router.service
    sudo rm -f /etc/systemd/system/winston-frontend.service
    sudo systemctl daemon-reload

    echo ""
    echo "Services uninstalled."
}

case "$OS" in
    Darwin)
        uninstall_macos
        ;;
    Linux)
        uninstall_linux
        ;;
    *)
        echo "Unsupported OS: $OS"
        exit 1
        ;;
esac
