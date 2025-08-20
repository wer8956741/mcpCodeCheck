#!/bin/bash

set -e

echo "🔄 lint-mcp 更新和发布脚本"

# 检查是否有未提交的更改
if [ -n "$(git status --porcelain)" ]; then
    echo "⚠️  检测到未提交的更改，建议先提交代码"
    read -p "是否继续? (y/N): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "❌ 已取消"
        exit 1
    fi
fi

# 选择版本更新类型
echo "请选择版本更新类型:"
echo "1) patch (修复 bug) - 1.0.0 -> 1.0.1"
echo "2) minor (新功能) - 1.0.0 -> 1.1.0" 
echo "3) major (破坏性更改) - 1.0.0 -> 2.0.0"
echo "4) 自定义版本号"
echo "5) 不更新版本号"

read -p "请选择 (1-5): " -n 1 -r
echo

case $REPLY in
    1)
        echo "📈 更新补丁版本..."
        npm version patch
        ;;
    2)
        echo "📈 更新小版本..."
        npm version minor
        ;;
    3)
        echo "📈 更新大版本..."
        npm version major
        ;;
    4)
        read -p "请输入版本号 (例如: 1.2.3): " VERSION
        echo "📈 更新到版本 $VERSION..."
        npm version $VERSION
        ;;
    5)
        echo "⏭️  跳过版本更新"
        ;;
    *)
        echo "❌ 无效选择"
        exit 1
        ;;
esac

# 显示当前版本
CURRENT_VERSION=$(node -p "require('./package.json').version")
echo "📦 当前版本: $CURRENT_VERSION"

# 测试构建
echo "🔨 测试构建..."
npm run build

if [ ! -f "bin/lint-mcp" ]; then
    echo "❌ 构建失败：二进制文件不存在"
    exit 1
fi

echo "✅ 构建成功"

# 检查 npm 登录状态
if ! npm whoami > /dev/null 2>&1; then
    echo "❌ 请先登录 npm: npm login"
    exit 1
fi

echo "✅ npm 登录状态正常"

# 显示将要发布的内容
echo "📦 将要发布的内容:"
npm pack --dry-run

# 确认发布
echo ""
echo "🚀 准备发布版本 $CURRENT_VERSION"
read -p "确认发布? (y/N): " -n 1 -r
echo

if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "❌ 发布已取消"
    exit 1
fi

# 发布
echo "🚀 发布到 npm..."
npm publish

echo ""
echo "✅ 发布成功!"
echo "📝 版本: $CURRENT_VERSION"
echo "🔗 用户可以通过以下方式更新:"
echo "   npx lint-mcp@latest"
echo "   npm update -g lint-mcp"

# 可选：推送 git 标签
if [ -n "$(git status --porcelain)" ] || [ -n "$(git log --oneline origin/main..HEAD 2>/dev/null)" ]; then
    echo ""
    read -p "是否推送 git 标签到远程仓库? (y/N): " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        git push origin main --tags
        echo "✅ Git 标签已推送"
    fi
fi

echo ""
echo "🎉 更新完成!"
