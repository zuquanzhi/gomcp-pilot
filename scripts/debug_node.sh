#!/bin/bash
echo "Starting node with args: $@" > /tmp/gomcp_start.log
/opt/homebrew/bin/node "$@" 2> /tmp/gomcp_debug.err
