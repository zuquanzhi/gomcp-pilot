#!/bin/bash
set -e

# Directory for local MCP servers
mkdir -p local_servers
cd local_servers

echo "Installing MCP server dependencies..."
if [ ! -f package.json ]; then
    npm init -y
fi

npm install @modelcontextprotocol/server-filesystem

echo "Done. Dependencies installed in ./local_servers"
