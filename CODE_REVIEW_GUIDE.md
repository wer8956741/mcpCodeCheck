# 🚀 Code Review 工具 - 智能代码上下文分析器

## 📋 工具概述

`code_review` 是一个基于 MCP (Model Context Protocol) 的智能代码分析工具，专门为 Go 语言项目设计。它能够智能检测代码变更，识别资源作用域，并为外部 AI 工具（如 Cursor、Claude 等）提供精准的代码上下文和结构化分析数据。

## ✨ 核心功能亮点

### 🎯 **1. 智能变更检测**
- **多策略自动检测**：未推送提交 → 分支分叉点 → 工作区变更 → 扩展范围 → 最近提交
- **精准定位**：自动识别真正需要审查的代码变更
- **零配置**：无需手动指定文件，工具自动发现所有相关变更

### 🔍 **2. 资源作用域识别** ⭐ **核心创新**
工具能够识别 **11 种** 常见的 Go 资源管理模式：

| 资源类型 | 开启模式 | 关闭模式 | 典型风险 |
|---------|---------|---------|---------|
| **数据库事务** | `tx, err := db.Begin()` | `tx.Commit()/Rollback()` | 错误路径缺少回滚 |
| **互斥锁** | `mu.Lock()` | `mu.Unlock()` | 锁未释放导致死锁 |
| **文件句柄** | `file, err := os.Open()` | `file.Close()` | 文件描述符泄漏 |
| **HTTP响应** | `resp, err := http.Get()` | `resp.Body.Close()` | 连接池资源泄漏 |
| **上下文取消** | `ctx, cancel := context.WithCancel()` | `cancel()` | goroutine泄漏 |
| **SQL查询结果** | `rows, err := db.Query()` | `rows.Close()` | 连接池耗尽 |
| **gRPC连接** | `conn, err := grpc.Dial()` | `conn.Close()` | 连接资源泄漏 |
| **临时目录** | `dir, err := os.MkdirTemp()` | `os.RemoveAll(dir)` | 磁盘空间泄漏 |
| **Redis连接** | `conn := pool.Get()` | `conn.Close()` | 连接池资源耗尽 |
| **消息队列** | `ch, err := conn.Channel()` | `ch.Close()` | AMQP连接泄漏 |
| **定时器** | `ticker := time.NewTicker()` | `ticker.Stop()` | goroutine泄漏 |

### 🧠 **3. 智能上下文扩展** ⭐ **解决核心痛点**

**问题场景**：
```go
// 用户只修改了第 47-49 行
func processOrder(orderID string) error {
    tx, err := db.Begin()  // 第 40 行
    if err != nil {
        return err
    }
    
    // 用户新增的代码 ⬇️
    result, err := validateOrder(orderID)  // 第 47 行
    if err != nil {
        return err  // ⚠️ 第 48 行：没有回滚事务！
    }
    // 用户新增的代码 ⬆️
    
    return tx.Commit()  // 第 52 行
}
```

**传统 AI 分析**：只看到第 47-49 行，无法发现事务管理问题

**我们的解决方案**：
- ✅ 检测到变更在事务作用域内（第 40-52 行）
- ✅ 提供完整的事务上下文代码
- ✅ 标记潜在风险："错误路径中缺少事务回滚"
- ✅ 生成针对性的检查提示

### 📊 **4. 结构化输出**

工具输出专门为 AI 工具优化的结构化数据：

```json
{
  "summary": {
    "totalChangedFiles": 2,
    "totalResourceScopes": 3,
    "riskLevel": "high",
    "focusAreas": ["transaction", "concurrency"],
    "baseCommit": "abc123",
    "strategy": "智能检测"
  },
  "changedFiles": [
    {
      "file": "service.go",
      "changedLines": [
        {"lineNumber": 47, "content": "result, err := validateOrder(orderID)", "type": "added"},
        {"lineNumber": 48, "content": "if err != nil {", "type": "added"},
        {"lineNumber": 49, "content": "return err", "type": "added"}
      ],
      "affectedScopes": ["scope_1"],
      "riskLevel": "high",
      "focusAreas": ["transaction"]
    }
  ],
  "resourceScopes": [
    {
      "id": "scope_1",
      "type": "transaction",
      "startLine": 40,
      "endLine": 52,
      "variable": "tx",
      "hasChanges": true,
      "riskPatterns": [
        "事务开启后未正确关闭",
        "错误路径中缺少事务回滚"
      ],
      "contextCode": "40: tx, err := db.Begin()\n41: if err != nil {\n42:   return err\n43: }\n44: \n45: // 用户新增的代码\n46: result, err := validateOrder(orderID)\n47: if err != nil {\n48:   return err  // ⚠️ 没有回滚事务！\n49: }\n50: \n51: return tx.Commit()"
    }
  ],
  "reviewPrompt": "请分析以下Go代码变更，重点检查资源管理问题...",
  "rawContext": "完整的代码上下文...",
  "processingTime": "150ms"
}
```

## 🎯 使用场景

### **场景 1：在 Cursor 中进行代码审查**
```
用户: "帮我 review 这段代码的事务处理"
Cursor: 调用 code_review 工具 → 获得完整事务上下文 → 精准 AI 分析
结果: AI 能看到完整的事务生命周期，准确识别错误处理问题
```

### **场景 2：CI/CD 流程集成**
```bash
# 在 GitHub Actions 中使用
- name: Code Review
  run: |
    curl -X POST http://mcp-server/code_review \
      -d '{"projectPath": ".", "reviewFocus": ["transaction", "concurrency"]}'
```

### **场景 3：开发工具集成**
- **VS Code 插件**：实时代码审查提示
- **Git Hooks**：提交前自动检查
- **IDE 集成**：编码时实时反馈

## 🚀 核心优势

### **1. 解决 AI 的盲点**
- ❌ **传统方式**：AI 只能看到变更的几行代码
- ✅ **我们的方式**：AI 获得完整的资源生命周期上下文

### **2. 提高分析精度**
- ❌ **传统方式**：AI 可能误判或遗漏关键问题
- ✅ **我们的方式**：基于完整上下文，AI 分析更准确

### **3. 节省开发时间**
- ❌ **传统方式**：需要手动复制粘贴大段代码
- ✅ **我们的方式**：自动提供精准的上下文

### **4. 专业化检测**
- ❌ **通用工具**：无法理解 Go 特有的资源管理模式
- ✅ **我们的工具**：专门针对 Go 语言优化

## 📋 API 参数

### **请求参数**
```json
{
  "projectPath": "项目根目录（可选）",
  "reviewFocus": ["concurrency", "transaction", "resource", "performance"]
}
```

### **关注点说明**
- `concurrency`: 并发安全（锁、goroutine、竞态条件）
- `transaction`: 事务管理（数据库事务、错误处理）
- `resource`: 资源管理（文件、连接、内存）
- `performance`: 性能问题（内存分配、循环效率）

## 🔧 安装和使用

### **1. 编译工具**
```bash
go build -o bin/go-guard-review main.go
```

### **2. 启动 MCP 服务**
```bash
./bin/go-guard-review
```

### **3. 在 Cursor 中配置**
```json
{
  "mcpServers": {
    "code-review": {
      "command": "/path/to/go-guard-review"
    }
  }
}
```

### **4. 使用工具**
在 Cursor 中直接调用：
```
@code-review 请分析当前代码变更的资源管理问题
```

## 🎯 最佳实践

### **1. 提交前检查**
```bash
# Git Hook 示例
#!/bin/sh
./bin/go-guard-review | jq '.summary.riskLevel' | grep -q "high" && exit 1
```

### **2. 重点关注高风险场景**
- 数据库事务处理
- 并发锁管理
- 网络连接处理
- 文件操作

### **3. 结合静态分析工具**
- 先用 `golangci-lint` 检查基础问题
- 再用 `code_review` 检查资源管理问题
- 最后用 AI 进行深度分析

## 🔮 未来规划

- [ ] 支持更多编程语言（Python、Java、Rust）
- [ ] 增加自定义资源模式配置
- [ ] 集成更多 AI 模型（Claude、GPT-4、本地模型）
- [ ] 提供 Web 界面和可视化报告
- [ ] 支持团队协作和规则共享

---

**让代码审查变得更智能、更精准、更高效！** 🚀
