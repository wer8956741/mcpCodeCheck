#!/bin/bash

set -e

echo "🚀 准备发布 lint-mcp 到 npm..."

# 检查是否已登录 npm
if ! npm whoami > /dev/null 2>&1; then
    echo "❌ 请先登录 npm: npm login"
    exit 1
fi

echo "✅ npm 登录状态正常"

# 构建二进制文件
echo "🔨 构建二进制文件..."
npm run build

# 检查二进制文件是否存在
if [ ! -f "bin/lint-mcp" ]; then
    echo "❌ 二进制文件 bin/lint-mcp 不存在"
    exit 1
fi

echo "✅ 二进制文件构建完成"

# 检查包内容
echo "📦 检查包内容..."
npm pack --dry-run

# 询问是否继续发布
read -p "是否继续发布? (y/N): " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "❌ 发布已取消"
    exit 1
fi

# 发布
echo "🚀 发布到 npm..."
npm publish

echo "✅ 发布成功!"
echo "📝 可以通过以下方式使用:"
echo "   npx lint-mcp"
echo "   npm install -g lint-mcp"
