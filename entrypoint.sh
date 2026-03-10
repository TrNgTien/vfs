#!/bin/sh
set -e

if [ $# -gt 0 ]; then
  exec /usr/local/bin/vfs "$@"
fi

VFS_PORT="${VFS_PORT:-8080}"
VFS_DASHBOARD_PORT="${VFS_DASHBOARD_PORT:-3000}"

exec /usr/local/bin/vfs serve --port "$VFS_PORT" --dashboard-port "$VFS_DASHBOARD_PORT"
