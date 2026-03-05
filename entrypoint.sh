#!/bin/sh
set -e

/usr/local/bin/vfs dashboard --port 3000 &
DASH_PID=$!

/usr/local/bin/vfs mcp --http :8080 &
MCP_PID=$!

trap "kill $DASH_PID $MCP_PID 2>/dev/null; exit 0" INT TERM

# Keep the shell alive; if either child exits, stop both.
while kill -0 $DASH_PID 2>/dev/null && kill -0 $MCP_PID 2>/dev/null; do
  wait $DASH_PID $MCP_PID 2>/dev/null || break
done

kill $DASH_PID $MCP_PID 2>/dev/null || true
wait
