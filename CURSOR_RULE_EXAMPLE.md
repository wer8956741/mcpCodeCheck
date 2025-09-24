# Go Code Review Cursor Rule 示例

## 概述

这是配合重构后的 `code_review` MCP 工具使用的 Cursor Rule 示例。工具现在专注于风险检测，返回结构化的风险数据，由 Cursor Rule 负责生成提示词和调用 AI。

## Cursor Rule 配置

```markdown
# Go Code Review Rule

当收到 MCP 工具返回的代码审查数据时，根据风险点进行分析：

## 数据结构理解

工具返回的数据结构：
- `summary`: 概览信息，包含总风险点数、风险等级、按分类和严重程度的统计
- `changedFiles`: 变更文件列表，每个文件包含变更行和风险点
- `riskPoints`: 具体的风险点，包含类型、严重程度、描述和修复建议

## 分析策略

### 高风险 (high severity)
对于 severity 为 "high" 的风险点：
1. **详细说明问题**：解释风险的具体影响和可能后果
2. **提供具体修复代码**：给出完整的修复示例
3. **解释潜在后果**：说明不修复可能导致的问题

示例响应：
```
🚨 **高风险问题发现**

**问题**: {risk.description}
**位置**: 第{risk.line}行
**代码**: `{risk.context}`

**风险分析**:
这是一个严重的{risk.category}问题，可能导致：
- 程序崩溃或异常终止
- 数据不一致或丢失
- 安全漏洞

**修复建议**:
{risk.suggestion}

**修复代码示例**:
```go
// 修复前
{risk.context}

// 修复后
[提供具体的修复代码]
```
```

### 中风险 (medium severity)
对于 severity 为 "medium" 的风险点：
1. **简要说明问题**：概述风险内容
2. **提供修复建议**：给出修复方向和要点

示例响应：
```
⚠️ **中风险提醒**

**问题**: {risk.description}
**建议**: {risk.suggestion}
```

### 低风险 (low severity)
对于 severity 为 "low" 的风险点：
1. **简单提醒**：指出需要注意的地方

示例响应：
```
💡 **优化建议**: {risk.description}
```

## 按分类处理

### Transaction (事务)
重点关注：
- 事务的开启和关闭
- 错误处理中的回滚
- 连接泄漏风险

### Panic Safety (Panic安全)
重点关注：
- 数组越界访问
- 类型断言安全
- 空指针检查
- recover 机制

### Concurrency (并发)
重点关注：
- 竞态条件
- 锁的获取和释放
- goroutine 泄漏

### Resource (资源)
重点关注：
- 文件句柄关闭
- 网络连接管理
- defer 语句使用

## 响应格式

根据风险统计生成概览：

```
## 📊 代码审查报告

**概览**:
- 变更文件: {summary.totalChangedFiles} 个
- 发现风险: {summary.totalRiskPoints} 个
- 整体风险等级: {summary.riskLevel}

**风险分布**:
- 高风险: {summary.riskBySeverity.high} 个
- 中风险: {summary.riskBySeverity.medium} 个  
- 低风险: {summary.riskBySeverity.low} 个

**分类统计**:
- 事务问题: {summary.riskByCategory.transaction} 个
- Panic风险: {summary.riskByCategory.panic_safety} 个
- 并发问题: {summary.riskByCategory.concurrency} 个
- 资源管理: {summary.riskByCategory.resource} 个

[然后按文件逐一分析风险点]
```

## 使用示例

当 MCP 工具返回如下数据时：

```json
{
  "summary": {
    "totalChangedFiles": 1,
    "totalRiskPoints": 2,
    "riskLevel": "high",
    "riskByCategory": {"transaction": 1, "panic_safety": 1},
    "riskBySeverity": {"high": 1, "medium": 1}
  },
  "changedFiles": [
    {
      "file": "service.go",
      "riskPoints": [
        {
          "id": "panic_call_325_1",
          "type": "panic_call",
          "category": "panic_safety", 
          "line": 325,
          "severity": "high",
          "description": "直接调用panic可能导致程序崩溃",
          "context": "panic(err)",
          "suggestion": "使用错误返回值替代panic，或添加recover机制"
        }
      ]
    }
  ]
}
```

Cursor Rule 应该生成：

```
## 📊 代码审查报告

**概览**: 发现 2 个风险点，整体风险等级为 high

🚨 **高风险问题发现**

**文件**: service.go 第325行
**问题**: 直接调用panic可能导致程序崩溃
**代码**: `panic(err)`

**风险分析**: 
直接调用panic会导致程序异常终止，在生产环境中可能造成服务不可用。

**修复建议**: 使用错误返回值替代panic，或添加recover机制

**修复代码示例**:
```go
// 修复前
panic(err)

// 修复后
return fmt.Errorf("操作失败: %w", err)
```
```

## 配置要点

1. **专注数据分析**：工具只返回风险数据，不生成提示词
2. **灵活响应**：根据风险等级和类型调整响应详细程度  
3. **结构化输出**：使用清晰的格式展示分析结果
4. **可定制性**：不同项目可以调整 Rule 的响应策略

这种架构分离了关注点：
- **MCP 工具**：专注风险检测和数据提取
- **Cursor Rule**：专注提示词生成和 AI 交互
- **AI 模型**：专注代码分析和建议生成
