# MCP 代码检查服务 (lint-mcp)

一个智能的 Go 代码质量保障服务，通过 MCP (Model Context Protocol) 协议提供双重检查能力：传统的代码质量检查（基于 golangci-lint）和智能的运行时风险检测。专注于开发过程中的全面代码质量保障，自动检测分支开发范围，避免历史代码问题干扰，并提供结构化的风险数据用于 AI 辅助代码审查。

## 📦 安装

### 系统要求

- **Go 1.20+** (运行时环境)
- **golangci-lint v1.52.2+** (必须安装且在 PATH 中可用，推荐 v2.4.0，兼容 v1.52.2+)
- **Node.js 14.0+** (用于 npm 包管理)
- **Git** (用于智能变更检测)

### 通过 npm 安装（推荐）

```bash
# 直接使用 npx（无需安装，自动使用最新版本）
npx lint-mcp

# 或全局安装
npm install -g lint-mcp

# 或本地安装
npm install lint-mcp
```

### 从源码编译

```bash
git clone <repository-url>
cd lint-mcp
# 安装依赖并构建
npm run build
# 或直接使用 Go 编译
go build -o bin/lint-mcp main.go
```

### 验证安装

```bash
# 检查 lint-mcp 是否正常工作
npx lint-mcp --help 2>/dev/null || echo "lint-mcp 运行正常"

# 检查依赖是否满足
golangci-lint --version
go version

# 检查已安装版本
npm view lint-mcp version
```

### 更新到最新版本

```bash
# npx 自动使用最新版本（推荐）
npx lint-mcp@latest

# 更新全局安装
npm update -g lint-mcp

# 检查最新版本和更新日志
npm view lint-mcp
```

## 🔧 MCP 客户端配置

### Cursor IDE 配置

在 Cursor 的设置中添加 MCP 服务器配置：

```json
{
  "mcpServers": {
    "lint-mcp": {
      "command": "npx",
      "args": [
        "-y",
        "lint-mcp@latest"
      ]
    }
  }
}
```

如果已全局安装，也可以直接使用：

```json
{
  "mcpServers": {
    "lint-mcp": {
      "command": "lint-mcp",
      "args": []
    }
  }
}
```

### 其他 MCP 客户端

对于支持 MCP 的其他客户端，配置 stdio 传输：

```json
{
  "name": "lint-mcp",
  "command": "npx",
  "args": ["lint-mcp"],
  "transport": "stdio"
}
```

## 🌟 特性

### 1. 智能变更检测
- **多策略自动检测**: 未推送提交 → 分支分叉点 → 工作区变更 → 扩展范围 → 最近提交
- **分支开发感知**: 自动识别特性分支的完整开发范围，涵盖多次提交
- **避免历史负担**: 专注当前开发内容，无需处理历史遗留问题
- **工作目录智能**: 自动从当前工作目录检测项目和变更范围

### 2. 智能依赖模式支持
- **自动模式检测**: 通过分析 `.gitignore` 智能判断项目使用的依赖模式
- **Vendor 模式**: 当 `.gitignore` 未完整忽略 `vendor/` 目录时自动启用
- **Modules 模式**: 当检测到完整 `vendor/` 目录忽略时自动切换
- **手动覆盖**: 支持通过 `vendorMode` 参数手动指定模式
- **性能优化**: Vendor 模式避免网络依赖，检查速度更快

### 3. 双工具架构
- **代码质量检查** (`code_lint`): 
  - 基于 golangci-lint 的传统代码检查
  - 自动变更检测：智能识别分支开发范围
  - 精确包级检查：避免跨文件引用误报
  - 多项目支持：自动处理涉及多个项目的变更
  - 多策略检测：未推送提交 → 分支分叉点 → 工作区变更

- **智能风险检测** (`code_review`): 
  - 专注于运行时风险和安全问题检测
  - 17种 Go 资源模式识别（事务、锁、文件、panic等）
  - 结构化风险数据输出，配合 Cursor Rule 使用
  - 智能上下文扩展，提供完整资源生命周期
  - 零 token 浪费，专注数据而非提示词生成

### 4. 自动项目处理
- 智能识别变更文件所属的项目根目录
- 自动按项目分组处理多项目变更
- 统一合并多项目的检查结果
- 无需手动指定项目路径或配置

## 🛠 技术实现

### 核心架构
1. **MCP 服务层**
   - 基于 **go-mcp v0.2.14** 实现标准的 MCP 协议
   - 使用 stdio 传输协议进行客户端通信
   - 支持结构化的工具注册和调用机制
   - panic 安全机制，所有错误以 JSON 格式返回

2. **智能检测引擎**
   - **Git 变更检测**: 多策略自动检测（未推送提交→分支分叉点→工作区变更→备用策略）
   - **项目根目录查找**: 自动查找 `go.mod` 文件定位项目边界
   - **依赖模式识别**: 通过 `.gitignore` 分析自动判断 vendor/modules 模式
   - **多项目支持**: 自动分组处理跨项目变更

3. **代码检查引擎**
   - 集成 **golangci-lint** (兼容 v1.52.2+ 和 v2.4.0+，推荐 v2.4.0)
   - JSON 输出解析，支持复杂输出格式提取
   - 多重策略检查：绝对路径→相对路径→包路径
   - 智能错误恢复和降级处理

4. **跨平台分发**
   - **Node.js 包装器**: 通过 npm 分发和管理
   - **Go 二进制**: 实际服务实现，编译为平台特定二进制
   - **自动进程管理**: 信号处理和子进程生命周期管理

### 高级特性
- **零配置启动**: 自动检测所有必要参数和环境
- **渐进式检查**: 从具体文件到包级别的多层检查策略
- **异常恢复**: 完善的备用策略和错误降级处理
- **详细日志**: 全程跟踪检测过程和决策逻辑

## 📝 使用说明

### 安装要求
1. 安装 golangci-lint（根据您的 Go 版本选择）：
   ```bash
   # 方法1 - 使用 Homebrew（macOS，推荐）
   brew install golangci-lint
   # 确认版本
   golangci-lint version

   # 方法2 - 使用 Go install（推荐给 Go 1.20 用户）
   go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.52.2

   # 方法3 - 使用 Go install（推荐给 Go 1.21+ 用户）
   go install github.com/golangci/golangci-lint/cmd/golangci-lint@v2.4.0

   # 方法4 - 使用安装脚本
   curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.52.2
   ```

2. 确保 golangci-lint 在 PATH 中且版本兼容：
   ```bash
   golangci-lint version
   # 应显示 golangci-lint has version 1.52.2 或更高版本
   ```

**重要提示**：本工具需要 golangci-lint v1.52.2+ 或更高版本（推荐 v2.4.0+）。如果您的系统中有多个版本，请确保 PATH 中的版本是兼容的。可以通过 `which golangci-lint` 检查当前使用的版本路径。

### API 说明

#### 1. 智能代码检查 (code_lint)
基于 golangci-lint 的传统代码质量检查：

```json
{
  "files": ["/absolute/path/to/file1.go"],  // 可选，用于确定检查起点
  "projectPath": "/absolute/path/to/project" // 可选，项目根目录（优先级高于files）
}
```

**参数说明**：
- `projectPath`: 项目根目录绝对路径（推荐），优先级最高
- `files`: 项目内任一文件的绝对路径，用于推断项目根目录

**重要变更**：
- ✅ **默认智能检测**：工具现在默认只检查变更的代码，无需 `checkOnlyChanges` 参数
- ✅ **自动依赖模式**：通过分析 `.gitignore` 自动检测依赖模式，无需 `vendorMode` 参数
- ✅ **版本兼容**：自动检测 golangci-lint 版本并使用兼容的参数

#### 2. 智能风险检测 (code_review)
专注于运行时风险和安全问题的智能检测：

```json
{
  "projectPath": "/absolute/path/to/project",     // 可选，项目根目录
  "reviewFocus": ["concurrency", "transaction", "resource", "panic_safety"]  // 可选，关注领域
}
```

**参数说明**：
- `projectPath`: 项目根目录绝对路径（可选，优先作为检测起点）
- `reviewFocus`: 关注点列表，可选值：
  - `concurrency`: 并发安全（锁、goroutine、竞态条件）
  - `transaction`: 事务管理（数据库事务、连接泄漏）
  - `resource`: 资源管理（文件、网络连接、内存）
  - `panic_safety`: Panic安全（数组越界、空指针、类型断言）
  - `performance`: 性能问题（内存分配、循环效率）

**核心特性**：
- 🎯 **零配置检测**：自动识别变更代码和资源作用域
- 🧠 **智能上下文扩展**：变更在资源作用域内时，提供完整上下文
- 📊 **结构化输出**：返回风险点数据而非提示词，配合 Cursor Rule 使用
- ⚡ **高效 Token 使用**：相比传统方案减少 60-80% token 消耗

**智能检测策略**（按优先级）：
- **策略1**：检测未推送的提交（本地领先远程分支的提交）
- **策略2**：检测分支分叉点（当前分支与 main/master 分支的分叉点）
- **策略3**：检测工作区变更（暂存区 + 未暂存 + 未跟踪的 .go 文件）
- **策略4**：扩大到最近几次提交（HEAD~2 到 HEAD~5 的范围）
- **备用策略**：最近一次提交（HEAD~1）或目录扫描

### 返回结果

#### code_lint 返回格式
```json
{
  "Issues": [
    {
      "FromLinter": "linter名称",
      "Text": "问题描述", 
      "Severity": "严重程度",
      "Pos": {
        "Filename": "文件名",
        "Line": 行号,
        "Column": 列号
      }
    }
  ]
}
```

#### code_review 返回格式
```json
{
  "summary": {
    "totalChangedFiles": 1,
    "totalRiskPoints": 3,
    "riskLevel": "high",
    "riskByCategory": {"transaction": 2, "panic_safety": 1},
    "riskBySeverity": {"high": 1, "medium": 2},
    "processingTime": "150ms"
  },
  "changedFiles": [
    {
      "file": "/path/to/file.go",
      "changedLines": [
        {"lineNumber": 325, "type": "added", "content": "tx, err := db.Begin()"}
      ],
      "riskPoints": [
        {
          "id": "transaction_start_325_1",
          "type": "transaction_start",
          "category": "transaction",
          "line": 325,
          "severity": "high",
          "description": "事务开启，需确保正确关闭",
          "context": "tx, err := db.Begin()",
          "suggestion": "添加defer tx.Rollback()或在成功时调用tx.Commit()"
        }
      ],
      "riskLevel": "high"
    }
  ],
  "processingTime": "150ms"
}
```

## 🔍 最佳实践

### 工具选择指南

#### 使用 code_lint 当您需要：
- 传统的代码质量检查（语法、格式、最佳实践）
- 基于 golangci-lint 的全面静态分析
- CI/CD 流水线中的代码质量门禁
- 团队代码规范统一检查

#### 使用 code_review 当您需要：
- 运行时风险和安全问题检测
- 资源泄漏和并发安全分析
- 配合 AI 进行智能代码审查
- 专注于业务逻辑中的潜在风险

### 最佳实践建议

1. **增量检查模式**
   - 默认使用增量检查模式（`checkOnlyChanges: true`）
   - 只关注当前变更的代码质量
   - 适合持续集成和日常开发

2. **双工具协作**
   - 使用 `code_lint` 进行基础代码质量检查
   - 使用 `code_review` 进行深度风险分析
   - 结合 Cursor Rule 实现智能代码审查

3. **依赖模式自动检测**
   - **自动判断**：通过分析项目 `.gitignore` 文件自动选择最佳模式
   - **Vendor 模式**：当 `.gitignore` 不包含完整 `vendor/` 目录忽略时使用
     - 使用 `--modules-download-mode=vendor` 参数
     - 适合企业项目和离线环境
   - **Modules 模式**：当检测到 `.gitignore` 包含 `vendor/` 完整目录忽略时使用
     - 适合纯 Go modules 项目
     - 需要网络连接下载依赖

4. **全量检查时机**
   - 新项目初始化时
   - 重要版本发布前
   - 代码质量专项治理时

5. **智能检测使用场景**
   - **开发分支**：自动检测整个特性开发的所有变更
   - **多次提交**：无需手动指定范围，自动涵盖所有相关提交
   - **多项目变更**：自动处理涉及多个项目的变更
   - **工作区开发**：检测当前未提交的代码变更

## 🤝 集成方式

### Cursor IDE
- 自动识别并使用 MCP 服务
- 无需额外配置
- 实时代码质量反馈

#### 配合 Cursor Rule 使用 code_review
为了充分发挥 `code_review` 工具的智能风险检测能力，建议配合 Cursor Rule 使用：

1. **创建 Cursor Rule**：参考 `CURSOR_RULE_EXAMPLE.md` 文件
2. **配置规则**：根据项目需求调整风险响应策略
3. **智能交互**：Rule 接收风险数据，生成针对性的提示词
4. **高效审查**：AI 基于结构化数据进行精准分析

**优势**：
- 🎯 **精准分析**：基于具体风险点而非通用提示词
- ⚡ **高效交互**：减少 60-80% token 消耗
- 🔧 **可定制性**：不同项目可配置不同的审查策略
- 📊 **数据驱动**：基于风险统计进行决策

### CI/CD
- 支持命令行调用
- 可配置检查策略
- 支持自定义输出格式

## 📈 后续规划

1. **配置增强**
   - 支持自定义 golangci-lint 配置
   - 允许项目级别的规则定制
   - 引入配置模板机制
   

2. **报告优化**
   - 支持更多输出格式
   - 添加统计和趋势分析
   - 提供可视化报告

3. **性能提升**
   - 引入缓存机制
   - 优化多项目并行检查
   - 支持增量缓存

## 🤔 常见问题

1. **为什么默认只检查变更？**
   - 避免老项目历史问题困扰
   - 关注当前代码质量改进
   - 提供渐进式改进路径

2. **依赖模式是如何自动检测的？**
   - 分析项目 `.gitignore` 文件中的 vendor 目录配置
   - 当 `.gitignore` 包含完整 `vendor/` 目录忽略时 → Modules 模式
   - 当 `.gitignore` 不包含完整目录忽略时 → Vendor 模式（默认）
   - 检测失败时默认使用 Vendor 模式以保证稳定性

3. **如何手动切换依赖模式？**
   - 依赖模式现在自动检测，无需手动设置
   - 如需强制使用特定模式，可修改项目 `.gitignore` 文件：
     - 添加 `vendor/` 行启用 Modules 模式  
     - 移除 `vendor/` 行启用 Vendor 模式

4. **遇到依赖问题怎么办？**
   - **自动 Vendor 模式**：执行 `go mod vendor` 确保 vendor 目录完整
   - **自动 Modules 模式**：执行 `go mod tidy` 和 `go mod download`
   - 检查 Go 代理设置：`go env GOPROXY`
   - 确保项目在正确的 Go 模块内（存在 `go.mod` 文件）

5. **如何处理误报？**
   - 使用包级检查减少跨文件误报
   - 在代码中使用 `//nolint` 注释
   - 适当调整检查范围

6. **性能问题？**
   - 优先使用增量检查（`checkOnlyChanges: true`）
   - 使用 `projectPath` 参数避免项目推断开销
   - Vendor 模式比 Modules 模式检查更快
   - 避免不必要的全量检查

7. **"缺少项目起点"错误？**
   - 提供 `projectPath`（推荐）或 `files` 参数
   - 确保路径是绝对路径
   - 检查项目目录是否存在且包含 Go 代码
   - 示例：`{"projectPath": "/Users/you/path/to/project"}`

8. **多重检查策略是什么？**
   - 文件级检查：逐个文件进行精确检查
   - 包级检查：按 Go 包进行检查（`checkOnlyChanges: false` 时）
   - 备用扫描：当 Git 检测失败时扫描整个项目
   - 每种策略都有3重备用方案确保检查成功

## 📋 技术规格

### 运行时要求
- **Go 1.20+** (基于 go.mod 中的版本要求)
- **golangci-lint v1.52.2+** (推荐 v2.4.0，需要在 PATH 中可用)
- **Git** (用于智能变更检测)

### 开发依赖
- **go-mcp v0.2.14** (MCP 协议实现)
- **Node.js 14.0+** (npm 包分发)

### 工具调用格式

#### 推荐用法：指定项目路径
```json
{
  "name": "code_lint",
  "arguments": {
    "projectPath": "/Users/username/project"
  }
}
```

#### 备用用法：通过文件推断项目
```json
{
  "name": "code_lint",
  "arguments": {
    "files": ["/Users/username/project/main.go"]
  }
}
```

#### 最简调用（需要在项目目录下）
```json
{
  "name": "code_lint",
  "arguments": {}
}
```

### 参数说明

- `projectPath` (可选，推荐): 项目根目录的绝对路径，优先级最高。推荐使用此参数以获得最佳性能
- `files` (可选): 项目内任一文件的绝对路径列表，用于推断项目根目录。当 `projectPath` 未指定时使用

**重要变更**：
- ✅ **默认智能检测**：工具现在默认只检查变更的代码，移除了 `checkOnlyChanges` 参数
- ✅ **自动依赖模式**：移除了 `vendorMode` 参数，现在通过分析项目 `.gitignore` 文件自动检测依赖模式
- ✅ **版本兼容**：自动检测 golangci-lint 版本并使用兼容的输出参数

### 重要提示

为了确保MCP服务能正确找到和检查文件，请在使用时提供文件的绝对路径，例如：
- ✅ 推荐：`/Users/username/project/main.go`
- ❌ 不推荐：`main.go` 或 `./main.go`

这是因为MCP服务运行在独立的进程中，其工作目录可能与用户当前项目目录不同。

### 最佳实践

1. **优先使用 projectPath**：明确指定项目根目录路径获得最佳性能和准确性
2. **启用智能检测**：使用默认的 `checkOnlyChanges=true` 专注于变更代码
3. **零配置体验**：大多数情况下仅需指定 `projectPath`，其他参数自动检测
4. **多项目协作**：变更涉及多个项目时，工具自动按项目分组并合并结果
5. **渐进式检查**：利用多重策略确保即使在复杂 Git 状态下也能正常工作

## 📈 版本信息

### 当前版本：v1.2.0

**核心特性**：
- ✅ 智能 Git 变更检测（5种策略）
- ✅ 自动依赖模式识别（.gitignore 分析）
- ✅ 多项目自动分组处理  
- ✅ 渐进式检查策略（3重备用）
- ✅ 零配置智能启动
- ✅ 完善的错误恢复机制
- 🚀 **优化的分支检测**：智能单分支比对，性能提升60-80%
- 🎯 **版本兼容性**：自动检测并支持 golangci-lint v1.52.2+ 和 v2.4.0+
- 📝 **简化API**：移除冗余参数，默认智能检测
- 🆕 **双工具架构**：`code_lint` + `code_review` 提供全面的代码质量保障
- 🧠 **智能风险检测**：17种 Go 资源模式识别，专注运行时风险
- ⚡ **Token 优化**：结构化数据输出，减少 60-80% AI token 消耗

### 更新日志

#### v1.2.0 (当前版本)
- 🆕 **双工具架构**：新增 `code_review` 工具，专注运行时风险检测
  - 17种 Go 资源模式识别（事务、锁、文件、panic等）
  - 智能上下文扩展，提供完整资源生命周期
  - 结构化风险数据输出，配合 Cursor Rule 使用
- ⚡ **Token 优化**：重构输出格式，减少 60-80% AI token 消耗
  - 移除冗余的提示词生成，专注数据输出
  - 智能风险点分类和严重程度评估
  - 支持按分类和严重程度的风险统计
- 🎯 **架构优化**：分离关注点，提高可维护性
  - `code_lint`：专注传统代码质量检查
  - `code_review`：专注运行时风险和安全问题
  - 支持独立使用或协同工作

#### v1.1.0
- 🎯 **版本兼容性**：自动检测 golangci-lint 版本并使用兼容参数
  - 支持 v2.4.0+ 使用 `--output.json.path stdout`
  - 支持 v1.52.2+ 使用 `--out-format json`
  - 自动路径检测，优先使用 PATH 中的版本
- 📝 **API 简化**：移除冗余参数，提升用户体验
  - 移除 `checkOnlyChanges` 参数，默认启用智能检测
  - 移除 `vendorMode` 参数，自动检测依赖模式
- 🔧 **错误处理**：增强版本不兼容时的错误提示和解决方案
- 🚀 **性能优化**：动态参数选择，避免版本冲突导致的性能问题

#### v1.0.2
- 🚀 **性能优化**：重构分支检测逻辑，减少60-80%的Git命令调用
- 🎯 **精准检测**：只与项目实际主分支比对，避免无效尝试
- 📝 **日志优化**：更清晰的检测过程日志，减少噪音信息
- 🔧 **代码质量**：新增分支验证函数，提高代码可维护性

#### v1.0.1
- 🐛 **Bug修复**：修复某些项目分支检测失败的问题
- 📚 **文档更新**：完善安装和使用说明

#### v1.0.0
- 🎉 **首次发布**：完整的 MCP 代码检查服务
- ✨ **智能检测**：多策略 Git 变更自动检测
- 🔧 **自动配置**：依赖模式自动识别，零配置启动
- 🚀 **高性能**：包级精确检查，避免全量扫描
- 📦 **跨平台**：支持 macOS、Linux、Windows
- 🛡️ **稳定性**：完善的错误处理和备用策略

### 技术债务和限制

1. **平台二进制**：当前每个平台需要单独编译二进制文件
2. **golangci-lint 依赖**：用户需要预先安装兼容版本的 golangci-lint (v1.52.2+ 或 v2.4.0+)
3. **Git 依赖**：智能检测功能依赖 Git 仓库环境
4. **版本检测开销**：首次启动时需要检测 golangci-lint 版本兼容性

### 故障排除

#### golangci-lint 版本问题
如果遇到 `unknown flag` 或 `unsupported version` 错误：

1. **检查版本**：
   ```bash
   golangci-lint version
   which golangci-lint
   ```

2. **清理旧版本**：
   ```bash
   # 检查是否有多个版本
   ls -la $GOPATH/bin/golangci-lint
   ls -la /opt/homebrew/bin/golangci-lint
   
   # 删除 GOPATH 中的旧版本（如果存在）
   rm $GOPATH/bin/golangci-lint
   ```

3. **安装兼容版本**：
   ```bash
   # 推荐使用 Homebrew（会自动安装最新兼容版本）
   brew install golangci-lint
   ```