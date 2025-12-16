import os
import sys
import json
import requests
from openai import OpenAI

# Configuration
GOMCP_BASE_URL = os.getenv("GOMCP_BASE_URL", "http://localhost:8080")
GOMCP_TOKEN = os.getenv("GOMCP_TOKEN", "TEST")
OPENAI_API_KEY = os.getenv("OPENAI_API_KEY") # Or DEEPSEEK_API_KEY
OPENAI_BASE_URL = os.getenv("OPENAI_BASE_URL", "https://api.deepseek.com") # Default to DeepSeek

if not OPENAI_API_KEY:
    print("Error: OPENAI_API_KEY (or DEEPSEEK_API_KEY) environment variable is required.")
    print("Usage: export OPENAI_API_KEY='sk-...' && python3 scripts/ai_agent.py")
    sys.exit(1)

client = OpenAI(api_key=OPENAI_API_KEY, base_url=OPENAI_BASE_URL)

def get_mcp_tools():
    """Fetch tools from gomcp-pilot and convert to OpenAI format."""
    url = f"{GOMCP_BASE_URL}/tools/list"
    headers = {"Authorization": f"Bearer {GOMCP_TOKEN}"}
    
    try:
        resp = requests.get(url, headers=headers)
        resp.raise_for_status()
        data = resp.json()
    except Exception as e:
        print(f"Error fetching tools from Gateway: {e}")
        sys.exit(1)

    openai_tools = []
    # Map "safe_name" back to "upstream/tool_name"
    tool_map = {} 

    for tool in data.get("tools", []):
        # OpenAI tool names must be regex ^[a-zA-Z0-9_-]+$
        # Our tools are like "filesystem", "math-js", etc. but name is just "add".
        # Wait, the Gateway returns a list of tools, but does it include the upstream name in the tool object?
        # Looking at gomcp code: `manager.ListTools` returns `[]mcp.Tool`. 
        # The Gateway's /tools/list API accepts ?upstream=... or lists ALL?
        # If I call /tools/list without upstream, what do I get?
        # In http.go: `upstream := r.URL.Query().Get("upstream")` -> `s.manager.ListTools(upstream)`
        # If upstream is empty, manager iterates ALL upstreams.
        # But `mcp.Tool` struct usually just has Name. 
        # If generic listing, we might have collisions if multiple upstreams have "echo".
        # Let's hope the Gateway Manager prefixes them or we need to handle it.
        # Checking manager.go ListTools implementation... 
        # It aggregates. If multiple have "echo", they will conflict or just be listed.
        # Actually proper MCP usage via Gateway usually requires specifying upstream in the CALL.
        
        # WE NEED A WAY to know which upstream a tool belongs to from the list, 
        # OR we need to query each upstream specifically.
        # For this demo, let's just query our known upstreams.
        pass

    return openai_tools, tool_map

def get_tools_from_gateway():
    """
    Since the Gateway's /tools/list might return tools without upstream info (depending on implementation),
    and /tools/call NEEDS upstream.
    
    Current gomcp-pilot implementation of /tools/list returns `[]mcp.Tool` which only has Name.
    So if we just call /tools/list, we get names like "echo", "add". We don't know if "echo" is from "local-utils" or another.
    
    Workaround for this Agent: We configured config.yaml, so we know the upstreams.
    We will query each upstream individually to build a robust map.
    """
    known_upstreams = ["filesystem", "local-utils", "crypto-py", "math-js"]
    
    openai_tools = []
    tool_map = {} # safe_name -> (upstream, tool_name)

    headers = {"Authorization": f"Bearer {GOMCP_TOKEN}"}

    print(f"Discovering tools from {GOMCP_BASE_URL}...")

    for upstream in known_upstreams:
        url = f"{GOMCP_BASE_URL}/tools/list?upstream={upstream}"
        try:
            resp = requests.get(url, headers=headers)
            if resp.status_code != 200:
                print(f"  [Warn] Could not list {upstream}: {resp.status_code}")
                continue
            
            data = resp.json()
            if isinstance(data, list):
                tools = data
            else:
                tools = data.get("tools", [])
                
            print(f"  [{upstream}] Found {len(tools)} tools: {[t['name'] for t in tools]}")

            for t in tools:
                # Create unique name for OpenAI: upstream__toolname
                # e.g. math-js__add
                safe_upstream = upstream.replace("-", "_")
                safe_toolname = t['name'].replace("-", "_")
                unique_name = f"{safe_upstream}__{safe_toolname}"
                
                tool_map[unique_name] = (upstream, t['name'])

                # OpenAI requires parameters to be an object
                schema = t.get("inputSchema")
                if not schema or schema.get("type") != "object":
                     schema = {"type": "object", "properties": {}}

                openai_tool = {
                    "type": "function",
                    "function": {
                        "name": unique_name,
                        "description": f"[{upstream}] {t.get('description', '')}",
                        "parameters": schema
                    }
                }
                openai_tools.append(openai_tool)

        except Exception as e:
            print(f"  [Error] Failed to query {upstream}: {e}")

    print(f"Total tools mapped: {len(openai_tools)}")
    return openai_tools, tool_map

def call_mcp_tool(upstream, tool_name, arguments):
    url = f"{GOMCP_BASE_URL}/tools/call"
    headers = {
        "Authorization": f"Bearer {GOMCP_TOKEN}",
        "Content-Type": "application/json"
    }
    payload = {
        "upstream": upstream,
        "tool": tool_name,
        "arguments": arguments
    }
    
    print(f"  >> Calling MCP: {upstream}/{tool_name} {json.dumps(arguments)}")
    try:
        resp = requests.post(url, json=payload, headers=headers)
        if resp.status_code == 200:
            res_json = resp.json()
            # extract content text
            # result structure from gomcp: {"result": { "content": [...] }, ... }
            content = res_json.get("result", {}).get("content", [])
            text_out = ""
            for item in content:
                if item.get("type") == "text":
                    text_out += item.get("text", "")
            print(f"  << Result: {text_out[:100]}...")
            return text_out
        else:
            err_msg = resp.text
            print(f"  << Error {resp.status_code}: {err_msg}")
            return f"Error: {err_msg}"
            
    except Exception as e:
        return f"Exception: {e}"

def chat_loop():
    tools, tool_map = get_tools_from_gateway()
    
    messages = [
        {"role": "system", "content": "You are an AI assistant powered by gomcp-pilot. You have access to various local tools. Use them to answer user questions."}
    ]

    print("\n--- AI Agent Started (Type 'quit' to exit) ---")
    
    while True:
        try:
            user_input = input("\nUser: ")
            if user_input.lower() in ["quit", "exit"]:
                break
            
            messages.append({"role": "user", "content": user_input})

            # First turn
            response = client.chat.completions.create(
                model="deepseek-chat",
                messages=messages,
                tools=tools,
                tool_choice="auto"
            )

            msg = response.choices[0].message
            messages.append(msg)

            if msg.tool_calls:
                print(f"[AI] Requesting {len(msg.tool_calls)} tool calls...")
                
                for tool_call in msg.tool_calls:
                    fn_name = tool_call.function.name
                    args_str = tool_call.function.arguments
                    args = json.loads(args_str)

                    if fn_name in tool_map:
                        upstream, real_tool_name = tool_map[fn_name]
                        result = call_mcp_tool(upstream, real_tool_name, args)
                    else:
                        result = f"Error: Tool {fn_name} not found in map."

                    messages.append({
                        "role": "tool",
                        "tool_call_id": tool_call.id,
                        "content": result
                    })
                
                # Second turn (with results)
                final_resp = client.chat.completions.create(
                    model="deepseek-chat",
                    messages=messages,
                    # We usually don't need tools for final answer unless it chains, but let's keep it simple
                    tools=tools 
                )
                final_msg = final_resp.choices[0].message
                messages.append(final_msg)
                print(f"AI: {final_msg.content}")

            else:
                print(f"AI: {msg.content}")

        except KeyboardInterrupt:
            break
        except Exception as e:
            print(f"Error: {e}")

if __name__ == "__main__":
    chat_loop()
