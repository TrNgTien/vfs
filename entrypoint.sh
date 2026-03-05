#!/bin/sh
set -e

if [ $# -gt 0 ]; then
  exec /usr/local/bin/vfs "$@"
fi

exec /usr/local/bin/vfs serve
