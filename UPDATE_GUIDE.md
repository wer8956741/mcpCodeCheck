# 🔄 更新和发布指南

## 更新 MCP 服务的完整流程

### 1. 修改代码
```bash
# 修改 main.go 或其他 Go 源文件
# 例如：添加新功能、修复 bug、优化性能等
```

### 2. 测试更改
```bash
# 本地测试编译
go build -o test-lint-mcp main.go
./test-lint-mcp

# 或使用 npm 脚本测试
npm run build
node index.js
```

### 3. 更新版本号
根据更改类型选择合适的版本更新：

```bash
# 补丁版本 (1.0.0 -> 1.0.1) - 修复 bug
npm version patch

# 小版本 (1.0.0 -> 1.1.0) - 新增功能，向后兼容
npm version minor

# 大版本 (1.0.0 -> 2.0.0) - 破坏性更改
npm version major
```

### 4. 更新文档（如需要）
```bash
# 更新 README.md 中的新功能说明
# 更新 API 文档
# 添加更改日志
```

### 5. 重新发布
```bash
# 使用发布脚本
npm run publish-package

# 或手动发布
npm run build
npm publish
```

## 📋 更新检查清单

### 代码更改后必须检查：
- [ ] Go 代码编译无错误
- [ ] 新功能测试通过
- [ ] 现有功能未被破坏
- [ ] 更新了版本号
- [ ] 更新了相关文档

### 发布前验证：
- [ ] `npm run build` 成功
- [ ] 二进制文件大小合理
- [ ] `npm pack --dry-run` 内容正确
- [ ] 本地测试 MCP 服务正常

## 🚀 快速更新脚本

让我为你创建一个快速更新脚本：
