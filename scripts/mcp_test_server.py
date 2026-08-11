"""DocMind 联调测试用 MCP Server（stdio 传输，纯标准库实现，无需 pip install）。

用法：
    python scripts/mcp_test_server.py

然后在 DocMind 中添加 MCP 服务：
    transport_type: stdio
    command: python
    args: ["D:/GoLang/DocMind/scripts/mcp_test_server.py"]  # 绝对路径
    env_vars: 可选

提供两个工具：
    get_current_time  获取当前服务器时间
    add_numbers       计算两个整数之和
"""

import json
import sys
import time

TOOLS = [
    {
        "name": "get_current_time",
        "description": "获取当前服务器的日期和时间",
        "inputSchema": {"type": "object", "properties": {}, "required": []},
    },
    {
        "name": "add_numbers",
        "description": "计算两个整数 a 和 b 的和",
        "inputSchema": {
            "type": "object",
            "properties": {
                "a": {"type": "integer", "description": "第一个整数"},
                "b": {"type": "integer", "description": "第二个整数"},
            },
            "required": ["a", "b"],
        },
    },
]


def handle_tool_call(name, args):
    if name == "get_current_time":
        text = json.dumps({"time": time.strftime("%Y-%m-%d %H:%M:%S")}, ensure_ascii=False)
        return {"content": [{"type": "text", "text": text}], "isError": False}
    if name == "add_numbers":
        try:
            total = int(args.get("a", 0)) + int(args.get("b", 0))
        except (TypeError, ValueError):
            return {"content": [{"type": "text", "text": "参数 a/b 必须为整数"}], "isError": True}
        return {"content": [{"type": "text", "text": json.dumps({"sum": total})}], "isError": False}
    return {"content": [{"type": "text", "text": f"未知工具: {name}"}], "isError": True}


def handle_request(method, params):
    if method == "initialize":
        return {
            "protocolVersion": params.get("protocolVersion", "2025-11-25"),
            "capabilities": {"tools": {}},
            "serverInfo": {"name": "docmind-test-server", "version": "1.0.0"},
        }
    if method == "tools/list":
        return {"tools": TOOLS}
    if method == "tools/call":
        return handle_tool_call(params.get("name"), params.get("arguments") or {})
    if method == "ping":
        return {}
    return None


def main():
    for line in sys.stdin:
        line = line.strip()
        if not line:
            continue
        try:
            msg = json.loads(line)
        except json.JSONDecodeError:
            continue

        method = msg.get("method")
        # 通知类消息无需响应
        if method == "notifications/initialized":
            continue

        result = handle_request(method, msg.get("params", {}))
        response = {"jsonrpc": "2.0", "id": msg.get("id")}
        if result is None:
            response["error"] = {"code": -32601, "message": f"Method not found: {method}"}
        else:
            response["result"] = result
        sys.stdout.write(json.dumps(response) + "\n")
        sys.stdout.flush()


if __name__ == "__main__":
    main()
