# 📋 发布检查清单

## ✅ 发布前必须完成的步骤

### 1. 包信息检查
- [ ] 确认 `package.json` 中的包名 `go-guard` 在 npm 上可用
- [ ] 更新 `author` 字段为你的信息
- [ ] 更新 `repository` URL 为你的 GitHub 仓库
- [ ] 确认版本号正确

### 2. 文件检查
- [ ] `bin/go-guard` 二进制文件存在且可执行
- [ ] `index.js` 有执行权限 (`chmod +x index.js`)
- [ ] `README.md` 内容完整
- [ ] `LICENSE` 文件存在

### 3. 功能测试
- [ ] 运行 `npm run build` 成功
- [ ] 运行 `node index.js` 能启动 MCP 服务
- [ ] 运行 `npm pack --dry-run` 检查包内容

### 4. npm 账户准备
- [ ] 拥有 npm 账户
- [ ] 运行 `npm login` 登录成功
- [ ] 运行 `npm whoami` 确认登录状态

## 🚀 发布命令

### 方式一：使用发布脚本（推荐）
```bash
npm run publish-package
```

### 方式二：手动发布
```bash
# 1. 构建
npm run build

# 2. 发布
npm publish
```

## 📝 发布后验证

```bash
# 检查包是否发布成功
npm view go-guard

# 测试 npx 安装和运行
npx go-guard@latest --version
```

## 🔄 更新版本

```bash
# 补丁版本更新
npm version patch && npm publish

# 小版本更新  
npm version minor && npm publish

# 大版本更新
npm version major && npm publish
```

## 📖 用户使用方式

发布成功后，用户可以通过以下方式使用：

```bash
# 直接使用（推荐）
npx go-guard

# 全局安装后使用
npm install -g go-guard
go-guard

# 在 Cursor 中配置
{
  "mcpServers": {
    "go-guard": {
      "command": "npx",
      "args": ["go-guard"]
    }
  }
}
```

## ⚠️ 注意事项

1. **包名冲突**: 如果 `go-guard` 已被占用，考虑使用 scoped 包名 `@yourusername/go-guard`
2. **平台限制**: 当前只支持当前编译平台，用户需要在相同平台使用
3. **依赖要求**: 用户系统需要安装 `golangci-lint`
4. **版本管理**: 每次发布都需要更新版本号
