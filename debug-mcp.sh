#!/bin/bash

# MCP服务调试脚本
# 使用方法：./debug-mcp.sh

LOG_FILE="/tmp/lint-mcp-debug.log"
BIN_PATH="$(dirname "$0")/bin/lint-mcp-debug"

echo "🔍 启动MCP服务调试模式..."
echo "📝 日志文件: $LOG_FILE"
echo "🚀 二进制文件: $BIN_PATH"
echo ""
echo "请在另一个终端中使用MCP客户端调用服务"
echo "按 Ctrl+C 停止服务"
echo ""

# 启动MCP服务并将日志输出到文件和控制台
"$BIN_PATH" 2>&1 | tee "$LOG_FILE"

