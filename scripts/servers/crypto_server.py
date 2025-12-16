import sys
import json
import hashlib
import uuid

# A minimal MCP-like server over stdio for testing.
# Implements: tools/list, tools/call
# Protocol: JSON-RPC 2.0 (Simplified for this demo)

def log(msg):
    sys.stderr.write(f"[crypto-py] {msg}\n")
    sys.stderr.flush()

def handle_request(line):
    try:
        req = json.loads(line)
    except:
        return

    req_id = req.get("id")
    method = req.get("method")
    
    # Initialize
    if method == "initialize":
        resp = {
            "jsonrpc": "2.0",
            "id": req_id,
            "result": {
                "protocolVersion": "2024-11-05",
                "capabilities": {"tools": {}},
                "serverInfo": {"name": "crypto-py", "version": "1.0"}
            }
        }
        print(json.dumps(resp))
        sys.stdout.flush()
        return

    # List Tools
    if method == "tools/list":
        resp = {
            "jsonrpc": "2.0",
            "id": req_id,
            "result": {
                "tools": [
                    {
                        "name": "hash_text",
                        "description": "Calculate MD5 or SHA256 hash of text",
                        "inputSchema": {
                            "type": "object",
                            "properties": {
                                "text": {"type": "string"},
                                "alg": {"type": "string", "enum": ["md5", "sha256"], "default": "sha256"}
                            },
                            "required": ["text"]
                        }
                    },
                    {
                        "name": "uuid_v4",
                        "description": "Generate a random UUID v4",
                        "inputSchema": {
                            "type": "object",
                            "properties": {}
                        }
                    }
                ]
            }
        }
        print(json.dumps(resp))
        sys.stdout.flush()
        return

    # Call Tool
    if method == "tools/call":
        params = req.get("params", {})
        name = params.get("name")
        args = params.get("arguments", {})
        
        result_content = ""
        
        if name == "hash_text":
            text = args.get("text", "")
            alg = args.get("alg", "sha256")
            if alg == "md5":
                res = hashlib.md5(text.encode()).hexdigest()
            else:
                res = hashlib.sha256(text.encode()).hexdigest()
            result_content = res
            
        elif name == "uuid_v4":
            result_content = str(uuid.uuid4())
            
        else:
            # Error
            resp = {
                "jsonrpc": "2.0",
                "id": req_id,
                "error": {"code": -32601, "message": "Method not found"}
            }
            print(json.dumps(resp))
            sys.stdout.flush()
            return

        resp = {
            "jsonrpc": "2.0",
            "id": req_id,
            "result": {
                "content": [{"type": "text", "text": result_content}]
            }
        }
        print(json.dumps(resp))
        sys.stdout.flush()
        return

    # Fallback for unknown methods
    resp = {
        "jsonrpc": "2.0",
        "id": req_id,
        "error": {"code": -32601, "message": f"Method {method} not found"}
    }
    print(json.dumps(resp))
    sys.stdout.flush()

def main():
    log("Starting crypto server...")
    for line in sys.stdin:
        if line.strip():
            handle_request(line)

if __name__ == "__main__":
    main()
