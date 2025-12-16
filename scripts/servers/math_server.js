const readline = require('readline');

// A minimal MCP-like server over stdio for testing.
// Implements: tools/list, tools/call

const rl = readline.createInterface({
    input: process.stdin,
    output: process.stdout,
    terminal: false
});

function log(msg) {
    process.stderr.write(`[math-js] ${msg}\n`);
}

log("Starting math server...");

rl.on('line', (line) => {
    if (!line.trim()) return;

    try {
        const req = JSON.parse(line);
        const id = req.id;
        const method = req.method;

        if (method === 'initialize') {
            const resp = {
                jsonrpc: "2.0",
                id: id,
                result: {
                    protocolVersion: "2024-11-05",
                    capabilities: { tools: {} },
                    serverInfo: { name: "math-js", version: "1.0" }
                }
            };
            console.log(JSON.stringify(resp));
            return;
        }

        if (method === 'tools/list') {
            const resp = {
                jsonrpc: "2.0",
                id: id,
                result: {
                    tools: [
                        {
                            name: "add",
                            description: "Add two numbers",
                            inputSchema: {
                                type: "object",
                                properties: {
                                    a: { type: "number" },
                                    b: { type: "number" }
                                },
                                required: ["a", "b"]
                            }
                        },
                        {
                            name: "multiply",
                            description: "Multiply two numbers",
                            inputSchema: {
                                type: "object",
                                properties: {
                                    a: { type: "number" },
                                    b: { type: "number" }
                                },
                                required: ["a", "b"]
                            }
                        }
                    ]
                }
            };
            console.log(JSON.stringify(resp));
            return;
        }

        if (method === 'tools/call') {
            const params = req.params || {};
            const name = params.name;
            const args = params.arguments || {};
            let result = "";

            if (name === "add") {
                result = String(Number(args.a) + Number(args.b));
            } else if (name === "multiply") {
                result = String(Number(args.a) * Number(args.b));
            } else {
                // Error
                console.log(JSON.stringify({
                    jsonrpc: "2.0",
                    id: id,
                    error: { code: -32601, message: "Method not found" }
                }));
                return;
            }

            const resp = {
                jsonrpc: "2.0",
                id: id,
                result: {
                    content: [{ type: "text", text: result }]
                }
            };
            console.log(JSON.stringify(resp));
            return;
        }

    } catch (e) {
        log("Error parsing: " + e);
    }
});
