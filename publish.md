# 发布到 npm 指南

## 发布前检查清单

### 1. 确保所有文件就绪
```bash
# 检查文件结构
ls -la
# 应该包含：
# - package.json
# - index.js
# - bin/lint-mcp (编译后的二进制文件)
# - README.md
# - LICENSE
# - .npmignore
```

### 2. 构建二进制文件
```bash
npm run build
```

### 3. 测试本地包
```bash
# 测试 Node.js 包装器
node index.js --version

# 测试 npx 本地调用
npx . --version
```

### 4. 检查包内容
```bash
# 预览将要发布的文件
npm pack --dry-run

# 或创建实际的 tarball 检查
npm pack
tar -tzf lint-mcp-1.0.0.tgz
```

## 发布步骤

### 1. 登录 npm
```bash
npm login
# 输入你的 npm 用户名、密码和邮箱
```

### 2. 发布包
```bash
# 首次发布
npm publish

# 如果包名被占用，可以使用 scoped 包名
# 修改 package.json 中的 name 为 "@yourusername/lint-mcp"
# npm publish --access public
```

### 3. 验证发布
```bash
# 检查包是否发布成功
npm view lint-mcp

# 测试从 npm 安装
npx lint-mcp@latest --version
```

## 更新版本

### 1. 更新版本号
```bash
# 补丁版本 (1.0.0 -> 1.0.1)
npm version patch

# 小版本 (1.0.0 -> 1.1.0)
npm version minor

# 大版本 (1.0.0 -> 2.0.0)
npm version major
```

### 2. 重新发布
```bash
npm publish
```

## 注意事项

1. **包名唯一性**: 确保 `lint-mcp` 这个包名在 npm 上可用
2. **版本管理**: 每次发布都需要更新版本号
3. **二进制文件**: 确保 `bin/lint-mcp` 是编译好的可执行文件
4. **跨平台**: 当前只包含当前平台的二进制文件，如需支持多平台需要额外配置
5. **依赖**: 确保用户系统有 `golangci-lint` 可用

## 多平台支持（可选）

如果需要支持多平台，可以考虑：

1. 使用 GitHub Actions 构建多平台二进制文件
2. 在 postinstall 脚本中根据平台下载对应的二进制文件
3. 或者发布多个平台特定的包
