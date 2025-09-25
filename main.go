package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// buildErrorResult 统一将错误以 JSON Issues 形式返回，避免上层只显示 "Error:"
func buildErrorResult(message string) *mcp.CallToolResult {
	lr := &LintResult{Issues: []Issue{{
		FromLinter: "lint-mcp",
		Text:       message,
		Pos:        Pos{Filename: "system", Line: 0, Column: 0},
	}}}
	b, _ := json.Marshal(lr)
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Type: "text", Text: string(b)}}}
}

// CodeLintRequest 定义智能代码检查请求结构
type CodeLintRequest struct {
	Files       []string `json:"files" description:"参考文件列表（可选，用于确定检查起点）。工具将自动智能检测当前工作目录的所有变更文件。" required:"false"`
	ProjectPath string   `json:"projectPath" description:"项目根目录（可选，优先作为检测起点，建议为Git仓库或包含go.mod的目录）"`
}

// CodeReviewRequest 定义代码审查请求结构（简化版）
type CodeReviewRequest struct {
	ProjectPath string   `json:"projectPath" description:"项目根目录（可选，优先作为检测起点，建议为Git仓库或包含go.mod的目录）"`
	ReviewFocus []string `json:"reviewFocus" description:"关注点列表，如 ['concurrency', 'transaction', 'resource', 'performance']，默认全部" required:"false"`
}

// FileAnalysis 文件分析结果（重构版）
type FileAnalysis struct {
	File         string        `json:"file"`         // 文件路径
	ChangedLines []ChangedLine `json:"changedLines"` // 变更行信息
	RiskPoints   []RiskPoint   `json:"riskPoints"`   // 风险点列表
	RiskLevel    string        `json:"riskLevel"`    // "high", "medium", "low", "none"
}

// ResourceScope 资源作用域信息（优化版）
type ResourceScope struct {
	ID           string   `json:"id"`           // 唯一标识符
	Type         string   `json:"type"`         // "transaction", "mutex", "file", "http_response"
	StartLine    int      `json:"startLine"`    // 资源开启行号
	EndLine      int      `json:"endLine"`      // 资源关闭行号
	OpenPattern  string   `json:"openPattern"`  // 开启模式，如 "tx, err := db.Begin()"
	ClosePattern string   `json:"closePattern"` // 关闭模式，如 "tx.Commit()"
	Variable     string   `json:"variable"`     // 资源变量名，如 "tx", "mu", "file"
	HasChanges   bool     `json:"hasChanges"`   // 变更是否在此作用域内
	RiskPatterns []string `json:"riskPatterns"` // 潜在风险模式
	ContextCode  string   `json:"contextCode"`  // 完整的作用域代码
}

// ResourcePattern 资源模式定义
type ResourcePattern struct {
	Type         string   `json:"Type"`         // 资源类型
	Name         string   `json:"Name"`         // 资源名称（中文）
	OpenRegex    string   `json:"OpenRegex"`    // 开启模式正则
	CloseRegex   string   `json:"CloseRegex"`   // 关闭模式正则
	RiskPatterns []string `json:"RiskPatterns"` // 风险模式描述
}

// ChangedLine 变更行信息
type ChangedLine struct {
	LineNumber int    `json:"LineNumber"` // 行号
	Content    string `json:"Content"`    // 行内容
	Type       string `json:"Type"`       // "added", "modified", "deleted"
}

// ContextBlock 上下文代码块
type ContextBlock struct {
	Type      string `json:"Type"`      // 上下文类型，如 "transaction", "function"
	StartLine int    `json:"StartLine"` // 起始行号
	EndLine   int    `json:"EndLine"`   // 结束行号
	Code      string `json:"Code"`      // 代码内容
	Reason    string `json:"Reason"`    // 包含此上下文的原因
}

// EnhancedContext 增强的代码上下文
type EnhancedContext struct {
	ChangedLines   []ChangedLine   `json:"ChangedLines"`   // 变更的代码行
	ResourceScopes []ResourceScope `json:"ResourceScopes"` // 检测到的资源作用域
	ContextBlocks  []ContextBlock  `json:"ContextBlocks"`  // 相关上下文块
	ProjectPath    string          `json:"ProjectPath"`    // 项目路径
	BaseCommit     string          `json:"BaseCommit"`     // 基准提交
	Strategy       string          `json:"Strategy"`       // 检测策略
}

// RiskPoint 风险点定义
type RiskPoint struct {
	ID          string `json:"id"`          // 唯一标识符
	Type        string `json:"type"`        // 风险类型: "transaction_leak", "panic_risk", "resource_leak"
	Category    string `json:"category"`    // 风险分类: "transaction", "concurrency", "panic_safety", "resource"
	Line        int    `json:"line"`        // 风险行号
	Severity    string `json:"severity"`    // 严重程度: "high", "medium", "low"
	Description string `json:"description"` // 风险描述
	Context     string `json:"context"`     // 关键代码片段
	Suggestion  string `json:"suggestion"`  // 修复建议
}

// AIInstructions AI结构化指令对象（优化版）
type AIInstructions struct {
	Type                string   `json:"type"`                 // 指令类型，如 "code_review_prompt"
	Priority            string   `json:"priority"`             // 优先级: "high", "medium", "low"
	Action              string   `json:"action"`               // 行动指令: "analyze_and_respond"
	Prompt              string   `json:"prompt"`               // 精简提示词
	FocusAreas          []string `json:"focus_areas"`          // 关注领域
	ExpectedOutput      string   `json:"expected_output"`      // 期望输出格式
	Language            string   `json:"language"`             // 编程语言
	AnalysisDepth       string   `json:"analysis_depth"`       // 分析深度: "shallow", "medium", "deep"
	IncludeSuggestions  bool     `json:"include_suggestions"`  // 是否包含修复建议
	IncludeExamples     bool     `json:"include_examples"`     // 是否包含代码示例
	RiskThreshold       string   `json:"risk_threshold"`       // 风险阈值: "low", "medium", "high"
	OutputFormat        string   `json:"output_format"`        // 输出格式: "json", "markdown", "structured"
	ContextType         string   `json:"context_type"`         // 上下文类型
	ConfidenceThreshold float64  `json:"confidence_threshold"` // 置信度阈值
}

// CodeReviewOutput 代码审查输出结果（重构版 - 专注风险数据）
type CodeReviewOutput struct {
	Summary        ReviewSummary  `json:"summary"`        // 概览信息
	ChangedFiles   []FileAnalysis `json:"changedFiles"`   // 变更文件分析（包含风险点）
	ProcessingTime string         `json:"processingTime"` // 处理时间
}

// ReviewSummary 审查概览（重构版 - 专注风险统计）
type ReviewSummary struct {
	TotalChangedFiles int            `json:"totalChangedFiles"` // 变更文件数
	TotalRiskPoints   int            `json:"totalRiskPoints"`   // 总风险点数
	RiskLevel         string         `json:"riskLevel"`         // "high", "medium", "low", "none"
	RiskByCategory    map[string]int `json:"riskByCategory"`    // 按分类统计风险: {"transaction": 2, "panic_safety": 1}
	RiskBySeverity    map[string]int `json:"riskBySeverity"`    // 按严重程度统计: {"high": 1, "medium": 2}
	ProcessingTime    string         `json:"processingTime"`    // 处理时间
}

// GolangciLintOutput golangci-lint 的实际输出格式
type GolangciLintOutput struct {
	Issues []Issue `json:"Issues"`
	Report struct {
		Linters []struct {
			Name    string `json:"Name"`
			Enabled bool   `json:"Enabled"`
		} `json:"Linters"`
	} `json:"Report"`
}

// LintResult 表示代码检查结果
type LintResult struct {
	Issues []Issue `json:"Issues"`
}

// Issue 表示单个代码问题
type Issue struct {
	FromLinter           string       `json:"FromLinter"`
	Text                 string       `json:"Text"`
	Severity             string       `json:"Severity"`
	SourceLines          []string     `json:"SourceLines"`
	Replacement          *Replacement `json:"Replacement"`
	Pos                  Pos          `json:"Pos"`
	ExpectNoLint         bool         `json:"ExpectNoLint"`
	ExpectedNoLintLinter string       `json:"ExpectedNoLintLinter"`
}

type Replacement struct {
	NewLines []string `json:"NewLines"`
}

type Pos struct {
	Filename string `json:"Filename"`
	Offset   int    `json:"Offset"`
	Line     int    `json:"Line"`
	Column   int    `json:"Column"`
}

// findGoModRoot 从指定目录开始向上查找go.mod文件
func findGoModRoot(startDir string) (string, error) {
	dir := startDir
	for {
		goModPath := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(goModPath); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break // 已经到达根目录
		}
		dir = parent
	}
	return "", fmt.Errorf("未找到go.mod文件")
}

// getProjectRootFromFile 从文件路径获取项目根目录
func getProjectRootFromFile(filePath string) (string, error) {
	if !filepath.IsAbs(filePath) {
		return "", fmt.Errorf("文件路径必须是绝对路径: %s", filePath)
	}

	// 从文件所在目录开始向上查找go.mod
	searchDir := filepath.Dir(filePath)
	log.Printf("从文件 %s 开始搜索项目根目录，搜索目录: %s", filePath, searchDir)

	if goModRoot, err := findGoModRoot(searchDir); err == nil {
		log.Printf("找到Go模块根目录: %s", goModRoot)
		return goModRoot, nil
	}

	// 如果没有找到go.mod，使用文件所在目录作为项目根目录
	log.Printf("未找到go.mod文件，使用文件所在目录作为项目根目录: %s", searchDir)
	return searchDir, nil
}

// getPackagesFromFiles 从文件列表中获取包路径列表（去重）
func getPackagesFromFiles(files []string) (map[string][]string, error) {
	// 返回 map[projectRoot][]packagePaths 的结构
	projectPackages := make(map[string]map[string]bool)
	result := make(map[string][]string)

	for _, file := range files {
		if file == "" {
			continue
		}

		if !filepath.IsAbs(file) {
			return nil, fmt.Errorf("文件路径必须是绝对路径: %s", file)
		}

		// 检查文件是否存在且是.go文件
		if _, err := os.Stat(file); err != nil {
			log.Printf("警告：文件 %s 不存在，跳过", file)
			continue
		}

		if !strings.HasSuffix(file, ".go") {
			log.Printf("警告：文件 %s 不是Go文件，跳过", file)
			continue
		}

		// 获取项目根目录
		projectRoot, err := getProjectRootFromFile(file)
		if err != nil {
			return nil, fmt.Errorf("获取项目根目录失败: %v", err)
		}

		// 获取文件所在目录作为包路径
		packageDir := filepath.Dir(file)

		// 转换为相对于项目根目录的包路径
		relPackagePath, err := filepath.Rel(projectRoot, packageDir)
		if err != nil {
			log.Printf("警告：无法计算包路径 %s 相对于项目根目录 %s 的路径：%v", packageDir, projectRoot, err)
			continue
		}

		// 在项目根目录的情况下使用 "."
		if relPackagePath == "" || relPackagePath == "." {
			relPackagePath = "."
		} else {
			// 确保使用正斜杠（Go模块路径格式）
			relPackagePath = strings.ReplaceAll(relPackagePath, "\\", "/")
			// 如果路径不以./开头，添加它
			if !strings.HasPrefix(relPackagePath, "./") {
				relPackagePath = "./" + relPackagePath
			}
		}

		// 按项目根目录分组包路径
		if projectPackages[projectRoot] == nil {
			projectPackages[projectRoot] = make(map[string]bool)
		}

		if !projectPackages[projectRoot][relPackagePath] {
			projectPackages[projectRoot][relPackagePath] = true
			log.Printf("发现包: %s (项目: %s, 文件: %s)", relPackagePath, projectRoot, file)
		}
	}

	// 转换为最终结果格式
	for projectRoot, packages := range projectPackages {
		var packageList []string
		for pkg := range packages {
			packageList = append(packageList, pkg)
		}
		result[projectRoot] = packageList
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("没有找到有效的Go包")
	}

	return result, nil
}

// getGolangciLintPath 获取可用的golangci-lint路径
func getGolangciLintPath() (string, error) {
	// 优先尝试GOPATH/bin中的版本（通常是go install安装的）
	if gopath := os.Getenv("GOPATH"); gopath != "" {
		gopathBin := filepath.Join(gopath, "bin", "golangci-lint")
		if _, err := os.Stat(gopathBin); err == nil {
			return gopathBin, nil
		}
	}

	// 尝试go env GOPATH
	cmd := exec.Command("go", "env", "GOPATH")
	if output, err := cmd.Output(); err == nil {
		gopath := strings.TrimSpace(string(output))
		if gopath != "" {
			gopathBin := filepath.Join(gopath, "bin", "golangci-lint")
			if _, err := os.Stat(gopathBin); err == nil {
				return gopathBin, nil
			}
		}
	}

	// 最后尝试PATH中的版本
	if path, err := exec.LookPath("golangci-lint"); err == nil {
		return path, nil
	}

	return "", fmt.Errorf("golangci-lint 未找到")
}

// checkGolangciLintVersion 检查golangci-lint版本是否符合要求
func checkGolangciLintVersion(golangciLintPath string) error {
	cmd := exec.Command(golangciLintPath, "--version")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("无法获取golangci-lint版本: %v", err)
	}

	versionOutput := string(output)
	log.Printf("检测到golangci-lint版本: %s", strings.TrimSpace(versionOutput))

	// 检查版本兼容性
	cmd = exec.Command(golangciLintPath, "run", "--help")
	helpOutput, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("无法获取golangci-lint run帮助信息: %v", err)
	}

	helpStr := string(helpOutput)

	// 检查是否支持JSON输出
	// v2.4.0+支持--output.json.path（新版本格式）
	supportsNewJsonOutput := strings.Contains(helpStr, "--output.json.path")
	// v1.52.2+支持--out-format json（旧版本格式）
	supportsOldJsonOutput := strings.Contains(helpStr, "--out-format")

	if !supportsNewJsonOutput && !supportsOldJsonOutput {
		return fmt.Errorf(`golangci-lint 版本不兼容

当前版本: %s
要求版本: v1.52.2 或更高版本

请升级到兼容版本：
# 推荐版本（适合Go 1.20+）
go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.52.2

# 或最新版本（适合Go 1.21+）
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

升级后请重启MCP服务。`, strings.TrimSpace(versionOutput))
	}

	return nil
}

// getGolangciLintOutputArgs 根据版本获取JSON输出参数
func getGolangciLintOutputArgs() ([]string, error) {
	golangciLintPath, err := getGolangciLintPath()
	if err != nil {
		return nil, err
	}

	cmd := exec.Command(golangciLintPath, "run", "--help")
	helpOutput, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("无法获取golangci-lint run帮助信息: %v", err)
	}

	helpStr := string(helpOutput)

	// v2.4.0+支持--output.json.path（优先检查新版本）
	if strings.Contains(helpStr, "--output.json.path") {
		return []string{"--output.json.path", "stdout"}, nil
	}

	// v1.52.2+支持--out-format（向后兼容）
	if strings.Contains(helpStr, "--out-format") {
		return []string{"--out-format", "json", "--print-issued-lines=false", "--print-linter-name=true"}, nil
	}

	return nil, fmt.Errorf("不支持的golangci-lint版本")
}

// checkGolangciLintInstalled 检查golangci-lint是否已安装并且版本兼容
func checkGolangciLintInstalled() error {
	golangciLintPath, err := getGolangciLintPath()
	if err != nil {
		return err
	}

	// 检查版本兼容性
	return checkGolangciLintVersion(golangciLintPath)
}

// autoDetectVendorMode 自动检测项目的依赖模式
func autoDetectVendorMode(projectRoot string) bool {
	log.Printf("检测项目 %s 的vendor模式", projectRoot)

	// 规则：仅依据项目根目录 .gitignore 是否包含完整的 'vendor/' 目录忽略
	// - 存在 'vendor/' 完整目录忽略 -> modules 模式（返回 false）
	// - 不存在完整目录忽略       -> vendor 模式（返回 true）
	gitignorePath := filepath.Join(projectRoot, ".gitignore")
	content, err := os.ReadFile(gitignorePath)
	if err != nil {
		log.Printf("无法读取 .gitignore 文件: %v，默认使用 vendor 模式", err)
		return true
	}

	lines := strings.Split(string(content), "\n")
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// 精确匹配：只有完整的 vendor/ 目录忽略才是 modules 模式
		// 兼容 '/vendor/' 与 'vendor/' 等写法，但不匹配 vendor/xxx/yyy.go 这种具体文件
		clean := strings.TrimPrefix(strings.TrimSuffix(line, "/"), "/")
		if clean == "vendor" {
			log.Printf(".gitignore 含完整 vendor/ 目录忽略（行: %s），使用 modules 模式", line)
			return false
		}
	}

	log.Printf(".gitignore 未包含完整 vendor/ 目录忽略，使用 vendor 模式")
	return true
}

// findAllGoFiles 在指定目录下查找所有Go文件（备用策略）
func findAllGoFiles(projectRoot string) ([]string, error) {
	log.Printf("扫描目录中的所有Go文件: %s", projectRoot)

	var goFiles []string

	err := filepath.Walk(projectRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 跳过vendor目录和隐藏目录
		if info.IsDir() && (info.Name() == "vendor" || strings.HasPrefix(info.Name(), ".")) {
			return filepath.SkipDir
		}

		// 只处理Go文件
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".go") {
			// 跳过测试文件和生成的文件
			if !strings.HasSuffix(info.Name(), "_test.go") &&
				!strings.Contains(info.Name(), ".pb.go") &&
				!strings.Contains(info.Name(), ".gen.go") {
				absPath, err := filepath.Abs(path)
				if err == nil {
					goFiles = append(goFiles, absPath)
					log.Printf("找到Go文件: %s", absPath)
				}
			}
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("扫描目录失败: %v", err)
	}

	if len(goFiles) == 0 {
		return nil, fmt.Errorf("目录中没有找到Go文件")
	}

	log.Printf("总共扫描到 %d 个Go文件", len(goFiles))
	return goFiles, nil
}

// detectBaseCommit 智能检测基准提交点
func detectBaseCommit(projectRoot string) (string, string, error) {
	log.Printf("智能检测项目 %s 的基准提交点", projectRoot)

	// 直接尝试各策略，若命令失败则跳过到下一策略

	// 策略1: 检测未推送的提交
	log.Printf("策略1: 尝试检测未推送的提交...")
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = projectRoot
	branchOutput, err := cmd.Output()
	if err == nil {
		currentBranch := strings.TrimSpace(string(branchOutput))
		log.Printf("当前分支: %s", currentBranch)
		remoteBranches := []string{"origin/" + currentBranch, "upstream/" + currentBranch, "remote/" + currentBranch}
		for _, remoteBranch := range remoteBranches {
			cmd = exec.Command("git", "rev-parse", "--verify", remoteBranch)
			cmd.Dir = projectRoot
			if err := cmd.Run(); err == nil {
				cmd = exec.Command("git", "rev-list", "--count", remoteBranch+"..HEAD")
				cmd.Dir = projectRoot
				countOutput, err := cmd.Output()
				if err == nil {
					count := strings.TrimSpace(string(countOutput))
					if count != "0" {
						return remoteBranch, fmt.Sprintf("未推送的提交(%s个)", count), nil
					}
				}
			}
		}
	}

	// 策略2: 检测与主分支的分叉点
	log.Printf("策略2: 尝试检测与主分支的分叉点...")
	mainBranch := getActualMainBranch(projectRoot)
	if mainBranch != "" {
		cmd = exec.Command("git", "merge-base", "HEAD", mainBranch)
		cmd.Dir = projectRoot
		output, err := cmd.Output()
		if err == nil {
			mergeBase := strings.TrimSpace(string(output))
			cmd = exec.Command("git", "rev-list", "--count", mergeBase+"..HEAD")
			cmd.Dir = projectRoot
			countOutput, err := cmd.Output()
			if err == nil {
				count := strings.TrimSpace(string(countOutput))
				if count != "0" {
					log.Printf("✅ 找到与%s的分叉点: %s (%s个提交)", mainBranch, mergeBase, count)
					return mergeBase, fmt.Sprintf("分支分叉点(vs %s, %s个提交)", mainBranch, count), nil
				}
			}
		}
	} else {
		log.Printf("⚠️ 未找到有效主分支，跳过策略2")
	}

	// 策略3: 工作区变更
	cmd = exec.Command("git", "status", "--porcelain")
	cmd.Dir = projectRoot
	statusOutput, err := cmd.Output()
	if err == nil && len(strings.TrimSpace(string(statusOutput))) > 0 {
		return "", "工作区变更", nil
	}

	// 策略4: 最近几次提交
	for i := 2; i <= 5; i++ {
		base := fmt.Sprintf("HEAD~%d", i)
		cmd = exec.Command("git", "rev-parse", "--verify", base)
		cmd.Dir = projectRoot
		if err := cmd.Run(); err == nil {
			return base, fmt.Sprintf("最近%d次提交", i), nil
		}
	}
	return "HEAD~1", "最近一次提交", nil
}

// getChangedGoFiles 获取变更的 Go 文件列表（工作区 + 提交范围并集）
func getChangedGoFiles(projectRoot string) ([]string, string, string, error) {
	// 尝试检测基准提交点（用于提交范围）
	baseCommit, strategy, _ := detectBaseCommit(projectRoot)
	log.Printf("使用检测策略: %s，基准提交: %s", strategy, baseCommit)

	changedSet := make(map[string]struct{})
	addLines := func(lines string) {
		for _, raw := range strings.Split(strings.TrimSpace(lines), "\n") {
			line := strings.TrimSpace(raw)
			if line == "" || !strings.HasSuffix(line, ".go") {
				continue
			}
			var absPath string
			if filepath.IsAbs(line) {
				absPath = line
			} else {
				absPath = filepath.Join(projectRoot, line)
			}
			cleanPath := filepath.Clean(absPath)
			finalPath, err := filepath.Abs(cleanPath)
			if err != nil {
				continue
			}
			if _, err := os.Stat(finalPath); err == nil {
				changedSet[finalPath] = struct{}{}
			}
		}
	}

	run := func(args ...string) string {
		cmd := exec.Command("git", args...)
		cmd.Dir = projectRoot
		out, err := cmd.Output()
		if err != nil {
			log.Printf("git %v 失败: %v", args, err)
			return ""
		}
		return string(out)
	}

	// 1) 工作区未暂存
	addLines(run("diff", "--name-only"))
	// 2) 工作区已暂存
	addLines(run("diff", "--name-only", "--cached"))
	// 3) 未跟踪的新文件
	addLines(run("ls-files", "--others", "--exclude-standard"))
	// 4) 提交范围（若存在）
	if baseCommit != "" {
		addLines(run("diff", "--name-only", baseCommit, "HEAD"))
	}

	// 汇总
	var changedGoFiles []string
	for p := range changedSet {
		changedGoFiles = append(changedGoFiles, p)
		log.Printf("收集到变更 Go 文件: %s", p)
	}
	if len(changedGoFiles) == 0 {
		return nil, "", "", fmt.Errorf("未找到任何变更的 Go 文件（工作区与提交范围均为空）")
	}
	log.Printf("总共找到 %d 个变更的 Go 文件（工作区+提交范围）", len(changedGoFiles))
	return changedGoFiles, baseCommit, strategy, nil
}

// runGolangciLintForChangedPackages 对包含变更文件的包进行智能检查
func runGolangciLintForChangedPackages(projectRoot string, changedFiles []string, baseCommit string, vendorMode bool) (*LintResult, error) {
	log.Printf("开始包级智能检查，项目根目录: %s，基准提交: %s，变更文件数: %d", projectRoot, baseCommit, len(changedFiles))

	// 按包分组变更文件
	packageDirs := make(map[string]bool)
	for _, file := range changedFiles {
		packageDir := filepath.Dir(file)
		packageDirs[packageDir] = true
	}

	log.Printf("发现 %d 个包含变更的包", len(packageDirs))

	allIssues := make([]Issue, 0)

	// 对每个包进行检查
	for packageDir := range packageDirs {
		relPackageDir, err := filepath.Rel(projectRoot, packageDir)
		if err != nil {
			log.Printf("警告：无法计算包路径 %s 相对于项目根目录 %s 的路径：%v", packageDir, projectRoot, err)
			continue
		}

		if relPackageDir == "" || relPackageDir == "." {
			relPackageDir = "."
		} else {
			// 确保使用正斜杠（Go模块路径格式）
			relPackageDir = strings.ReplaceAll(relPackageDir, "\\", "/")
			// 如果路径不以./开头，添加它
			if !strings.HasPrefix(relPackageDir, "./") {
				relPackageDir = "./" + relPackageDir
			}
		}

		log.Printf("检查包: %s (目录: %s)", relPackageDir, packageDir)

		args := []string{"run"}
		if vendorMode {
			args = append(args, "--modules-download-mode=vendor")
		}

		// 关键：添加智能检测的基准提交
		if baseCommit != "" {
			args = append(args, "--new-from-rev", baseCommit)
			log.Printf("使用基准提交进行变更检测: %s", baseCommit)
		}

		// 根据版本获取JSON输出参数
		outputArgs, err := getGolangciLintOutputArgs()
		if err != nil {
			log.Printf("包 %s 获取输出参数失败: %v", relPackageDir, err)
			continue
		}
		args = append(args, outputArgs...)
		args = append(args, relPackageDir)

		result, err := runGolangciLintWithArgs(projectRoot, args)
		if err != nil {
			log.Printf("包 %s 检查失败: %v", relPackageDir, err)
			continue
		}

		if result != nil && len(result.Issues) > 0 {
			log.Printf("包 %s 发现 %d 个问题", relPackageDir, len(result.Issues))
			allIssues = append(allIssues, result.Issues...)
		} else {
			log.Printf("包 %s 检查通过，无问题", relPackageDir)
		}
	}

	log.Printf("包级检查完成，总共发现 %d 个问题", len(allIssues))
	return &LintResult{Issues: allIssues}, nil
}

// runGolangciLintWithArgs 以自定义参数运行 golangci-lint 并解析 JSON 结果
func runGolangciLintWithArgs(projectRoot string, args []string) (*LintResult, error) {
	log.Printf("执行命令: golangci-lint %v", args)
	log.Printf("命令执行目录: %s", projectRoot)

	// golangci-lint 可用性检查已在服务启动时完成

	golangciLintPath, err := getGolangciLintPath()
	if err != nil {
		log.Printf("golangci-lint 未找到: %v", err)
		return nil, fmt.Errorf("golangci-lint 未找到: %v", err)
	}
	cmd := exec.Command(golangciLintPath, args...)
	cmd.Dir = projectRoot
	cmd.Env = os.Environ()

	output, cmdErr := cmd.CombinedOutput()
	log.Printf("命令输出长度: %d", len(output))
	log.Printf("命令执行错误: %v", cmdErr)

	// 即使有命令错误，也尝试解析输出（golangci-lint 发现问题时会返回非零退出码）
	if len(output) == 0 {
		if cmdErr != nil {
			log.Printf("命令无输出且有错误: %v", cmdErr)
			return nil, fmt.Errorf("golangci-lint 执行失败: %v", cmdErr)
		}
		return &LintResult{Issues: make([]Issue, 0)}, nil
	}

	jsonOutput := extractJSONFromOutput(string(output))
	if jsonOutput == "" {
		log.Printf("未能从输出中提取有效JSON，原始输出: %s", string(output))
		return &LintResult{Issues: make([]Issue, 0)}, nil
	}

	var golangciOutput GolangciLintOutput
	if err := json.Unmarshal([]byte(jsonOutput), &golangciOutput); err != nil {
		log.Printf("JSON解析失败: %v，JSON内容: %s", err, jsonOutput)
		return &LintResult{Issues: make([]Issue, 0)}, nil
	}

	log.Printf("成功解析到 %d 个问题", len(golangciOutput.Issues))
	return &LintResult{Issues: golangciOutput.Issues}, nil
}

// extractJSONFromOutput 从golangci-lint输出中提取JSON部分
func extractJSONFromOutput(output string) string {
	log.Printf("原始输出内容: %s", output)

	// 如果输出为空，直接返回
	trimmedOutput := strings.TrimSpace(output)
	if trimmedOutput == "" {
		log.Printf("输出为空")
		return ""
	}

	// 如果输出以 { 开始且以 } 结束，可能是完整的JSON
	if strings.HasPrefix(trimmedOutput, "{") && strings.HasSuffix(trimmedOutput, "}") {
		log.Printf("输出似乎已经是完整的JSON")
		// 验证是否为有效的JSON
		var js map[string]interface{}
		if err := json.Unmarshal([]byte(trimmedOutput), &js); err == nil {
			return trimmedOutput
		}
		log.Printf("看似JSON但解析失败，继续尝试提取")
	}

	// 尝试提取JSON部分
	log.Printf("尝试从输出中提取JSON部分")

	// 分行处理，查找可能的JSON行
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "{") && strings.HasSuffix(line, "}") {
			// 验证是否为有效的JSON
			var js map[string]interface{}
			if err := json.Unmarshal([]byte(line), &js); err == nil {
				log.Printf("找到有效的JSON行")
				return line
			}
		}
	}

	// 如果没有找到完整的JSON行，尝试提取最长的JSON片段
	maxLength := 0
	bestJSON := ""

	for i, char := range output {
		if char == '{' {
			// 找到一个开始位置
			currentStart := i
			braceCount := 1

			// 向后查找匹配的结束括号
			for j := i + 1; j < len(output); j++ {
				if output[j] == '{' {
					braceCount++
				} else if output[j] == '}' {
					braceCount--
					if braceCount == 0 {
						// 找到一个完整的JSON片段
						length := j - currentStart + 1
						if length > maxLength {
							// 验证是否为有效的JSON
							candidate := output[currentStart : j+1]
							var js map[string]interface{}
							if err := json.Unmarshal([]byte(candidate), &js); err == nil {
								maxLength = length
								bestJSON = candidate
							}
						}
						break
					}
				}
			}
		}
	}

	if bestJSON != "" {
		log.Printf("找到最长的有效JSON片段，长度: %d", len(bestJSON))
		return bestJSON
	}

	// 如果仍然没有找到有效的JSON，返回空字符串
	log.Printf("未找到有效的JSON")
	return ""
}

// handleCodeLintRequest 处理智能代码检查请求
func handleCodeLintRequest(ctx context.Context, req mcp.CallToolRequest) (result *mcp.CallToolResult, err error) {
	// 捕获任何 panic 并转换为错误返回
	defer func() {
		if r := recover(); r != nil {
			log.Printf("❌ 发生 panic: %v", r)
			result = buildErrorResult(fmt.Sprintf("内部错误: %v", r))
			err = nil // MCP 框架期望错误在结果中，而不是返回错误
		}
	}()

	log.Printf("🚀 收到智能代码检查请求: name=%s", req.Params.Name)
	log.Printf("📋 请求参数: %+v", req.Params.Arguments)

	var lintReq CodeLintRequest
	// 将 arguments 映射解码到结构体
	if req.Params.Arguments == nil {
		req.Params.Arguments = map[string]interface{}{}
	}
	argsBytes, _ := json.Marshal(req.Params.Arguments)
	if err := json.Unmarshal(argsBytes, &lintReq); err != nil {
		return buildErrorResult(fmt.Sprintf("无效的请求参数: %v", err)), nil
	}

	log.Printf("解析后的请求: %+v", lintReq)

	// 如果既没有 projectPath 也没有 files，则直接给出明确指引，避免从可执行目录误扫系统盘
	if strings.TrimSpace(lintReq.ProjectPath) == "" && (len(lintReq.Files) == 0 || strings.TrimSpace(lintReq.Files[0]) == "") {
		msg := "缺少项目起点：请提供 projectPath（项目根目录绝对路径，推荐）或 files（任一项目内文件的绝对路径）。例如：{\"projectPath\":\"/Users/you/path/to/project\"}。"
		return buildErrorResult(msg), nil
	}

	// 计算检测起点目录：优先 projectPath -> files 推断 -> 当前工作目录
	baseDir := ""
	if lintReq.ProjectPath != "" {
		abs, err := filepath.Abs(lintReq.ProjectPath)
		if err != nil {
			return buildErrorResult(fmt.Sprintf("projectPath 解析失败: %v", err)), nil
		}
		if !filepath.IsAbs(abs) {
			return buildErrorResult("projectPath 必须是绝对路径"), nil
		}
		if stat, err := os.Stat(abs); err != nil || !stat.IsDir() {
			return buildErrorResult(fmt.Sprintf("projectPath 无效或不是目录: %s", abs)), nil
		}
		baseDir = abs
	} else if len(lintReq.Files) > 0 && lintReq.Files[0] != "" {
		root, err := getProjectRootFromFile(lintReq.Files[0])
		if err != nil {
			return buildErrorResult(fmt.Sprintf("从 files 推断项目根目录失败: %v", err)), nil
		}
		baseDir = root
	} else {
		cwd, err := os.Getwd()
		if err != nil {
			return buildErrorResult(fmt.Sprintf("获取当前工作目录失败: %v", err)), nil
		}
		abs, err := filepath.Abs(cwd)
		if err != nil {
			return buildErrorResult(fmt.Sprintf("转换绝对路径失败: %v", err)), nil
		}
		baseDir = abs
	}
	log.Printf("检测起点目录: %s", baseDir)

	// 开始智能变更检测和包级检查
	log.Printf("开始智能检测变更文件（起点: %s）", baseDir)

	// 获取最新变更的 Go 文件（工作区+提交范围）
	changedFiles, baseCommit, strategy, err := getChangedGoFiles(baseDir)
	if err != nil {
		log.Printf("Git检测失败（起点: %s），尝试备用策略: %v", baseDir, err)
		fallbackFiles, fallbackErr := findAllGoFiles(baseDir)
		if fallbackErr != nil {
			return buildErrorResult(fmt.Sprintf("Git检测失败（起点: %s）: %v\n备用文件扫描也失败: %v\n\n请提供 projectPath 或 files 以明确项目位置。", baseDir, err, fallbackErr)), nil
		}
		log.Printf("使用备用策略：扫描到 %d 个Go文件（起点: %s）", len(fallbackFiles), baseDir)
		changedFiles = fallbackFiles
		baseCommit = "" // 备用策略时不使用基准提交
		strategy = "备用文件扫描"
	}

	log.Printf("检测策略: %s，基准提交: %s，变更文件: %d个", strategy, baseCommit, len(changedFiles))

	// 按项目分组变更文件，因为变更可能涉及多个项目
	projectFiles := make(map[string][]string)
	for _, file := range changedFiles {
		projectRoot, err := getProjectRootFromFile(file)
		if err != nil {
			log.Printf("警告：无法确定文件 %s 的项目根目录：%v", file, err)
			continue
		}
		projectFiles[projectRoot] = append(projectFiles[projectRoot], file)
	}

	// 对每个项目进行包级智能检查
	allIssues := make([]Issue, 0)
	for projectRoot, files := range projectFiles {
		log.Printf("检查项目 %s 中的 %d 个变更文件", projectRoot, len(files))

		vendorMode := autoDetectVendorMode(projectRoot)
		result, err := runGolangciLintForChangedPackages(projectRoot, files, baseCommit, vendorMode)
		if err != nil {
			log.Printf("项目 %s 检查失败: %v", projectRoot, err)
			return buildErrorResult(fmt.Sprintf("代码检查失败\n项目: %s\n基准提交: %s\nvendorMode: %v\n错误: %v", projectRoot, baseCommit, vendorMode, err)), nil
		}

		if result != nil {
			allIssues = append(allIssues, result.Issues...)
		}
	}

	log.Printf("所有项目检查完成，总共发现 %d 个问题", len(allIssues))
	finalResult := &LintResult{Issues: allIssues}
	resultJSON, _ := json.Marshal(finalResult)
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Type: "text", Text: string(resultJSON)}}}, nil
}

// getActualMainBranch 获取项目实际使用的主分支
func getActualMainBranch(projectRoot string) string {
	log.Printf("🔍 智能检测实际主分支...")

	// 方法1: 检查Git默认分支配置 (最准确)
	cmd := exec.Command("git", "symbolic-ref", "refs/remotes/origin/HEAD")
	cmd.Dir = projectRoot
	if output, err := cmd.Output(); err == nil {
		defaultRef := strings.TrimSpace(string(output))
		if parts := strings.Split(defaultRef, "/"); len(parts) >= 3 {
			branchName := strings.Join(parts[3:], "/")
			remoteBranch := "origin/" + branchName
			// 验证该分支是否真实存在
			if verifyBranchExists(projectRoot, remoteBranch) {
				log.Printf("✅ 检测到Git默认分支: %s", remoteBranch)
				return remoteBranch
			}
			// 尝试本地分支
			if verifyBranchExists(projectRoot, branchName) {
				log.Printf("✅ 检测到Git默认分支(本地): %s", branchName)
				return branchName
			}
		}
	}

	// 方法2: reflog历史检测 (检查当前分支是从哪里checkout出来的)
	currentBranch := getCurrentBranchSmart(projectRoot)
	if currentBranch != "" {
		cmd := exec.Command("git", "reflog", "--oneline", "-n", "15")
		cmd.Dir = projectRoot
		if output, err := cmd.Output(); err == nil {
			lines := strings.Split(strings.TrimSpace(string(output)), "\n")
			for _, line := range lines {
				if strings.Contains(line, "checkout: moving from") &&
					strings.Contains(line, " to "+currentBranch) {
					if idx := strings.Index(line, "moving from "); idx >= 0 {
						remaining := line[idx+len("moving from "):]
						if toIdx := strings.Index(remaining, " to "); toIdx >= 0 {
							sourceBranch := remaining[:toIdx]
							if sourceBranch != currentBranch && sourceBranch != "" {
								// 优先检查origin/分支
								remoteBranch := "origin/" + sourceBranch
								if verifyBranchExists(projectRoot, remoteBranch) {
									log.Printf("✅ 从reflog发现源分支: %s", remoteBranch)
									return remoteBranch
								}
								// 检查本地分支
								if verifyBranchExists(projectRoot, sourceBranch) {
									log.Printf("✅ 从reflog发现源分支(本地): %s", sourceBranch)
									return sourceBranch
								}
							}
						}
					}
				}
			}
		}
	}

	// 方法3: 按优先级检查常见主分支
	candidates := []string{"origin/main", "main", "origin/master", "master", "origin/develop", "develop"}
	for _, branch := range candidates {
		if verifyBranchExists(projectRoot, branch) {
			log.Printf("✅ 找到存在的主分支: %s", branch)
			return branch
		}
	}

	log.Printf("⚠️ 未能找到有效的主分支")
	return ""
}

// verifyBranchExists 验证分支是否存在
func verifyBranchExists(projectRoot, branch string) bool {
	cmd := exec.Command("git", "rev-parse", "--verify", branch)
	cmd.Dir = projectRoot
	return cmd.Run() == nil
}

// getCurrentBranchSmart 获取当前分支名
func getCurrentBranchSmart(projectRoot string) string {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = projectRoot
	if output, err := cmd.Output(); err == nil {
		return strings.TrimSpace(string(output))
	}
	return ""
}

// ==================== AI代码审查功能 ====================

// 支持的资源模式定义
var SupportedResourcePatterns = []ResourcePattern{
	{
		Type:       "transaction",
		Name:       "数据库事务",
		OpenRegex:  `(\w+),\s*err\s*:=\s*\w*\.Begin\w*\(`,
		CloseRegex: `(\w+)\.(?:Commit|Rollback)\(\)`,
		RiskPatterns: []string{
			"事务开启后未正确关闭",
			"错误路径中缺少事务回滚",
			"事务嵌套导致死锁",
			"长时间持有事务连接",
		},
	},
	{
		Type:       "mutex",
		Name:       "互斥锁",
		OpenRegex:  `(\w+)\.(?:Lock|RLock)\(\)`,
		CloseRegex: `(\w+)\.(?:Unlock|RUnlock)\(\)`,
		RiskPatterns: []string{
			"锁获取后未释放",
			"错误路径中缺少锁释放",
			"锁的获取和释放不匹配",
			"可能导致死锁的锁顺序",
		},
	},
	{
		Type:       "file",
		Name:       "文件句柄",
		OpenRegex:  `(\w+),\s*err\s*:=\s*(?:os\.Open|os\.Create|ioutil\.ReadFile)\(`,
		CloseRegex: `(\w+)\.Close\(\)`,
		RiskPatterns: []string{
			"文件打开后未关闭",
			"错误路径中文件句柄泄漏",
			"defer语句缺失",
			"文件句柄重复关闭",
		},
	},
	{
		Type:       "http_response",
		Name:       "HTTP响应",
		OpenRegex:  `(\w+),\s*err\s*:=\s*http\.(?:Get|Post|Do)\(`,
		CloseRegex: `(\w+)\.Body\.Close\(\)`,
		RiskPatterns: []string{
			"HTTP响应体未关闭",
			"连接池资源泄漏",
			"defer语句缺失",
			"响应体重复关闭",
		},
	},
	{
		Type:       "context",
		Name:       "上下文取消",
		OpenRegex:  `(\w+),\s*(\w+)\s*:=\s*context\.WithCancel\(`,
		CloseRegex: `(\w+)\(\)`,
		RiskPatterns: []string{
			"上下文取消函数未调用",
			"goroutine泄漏风险",
			"资源清理不完整",
			"超时控制失效",
		},
	},
	{
		Type:       "sql_rows",
		Name:       "数据库查询结果",
		OpenRegex:  `(\w+),\s*err\s*:=\s*\w*\.Query\w*\(`,
		CloseRegex: `(\w+)\.Close\(\)`,
		RiskPatterns: []string{
			"查询结果未关闭导致连接泄漏",
			"defer rows.Close()缺失",
			"错误路径中结果集未关闭",
			"连接池耗尽风险",
		},
	},
	{
		Type:       "grpc_conn",
		Name:       "gRPC连接",
		OpenRegex:  `(\w+),\s*err\s*:=\s*grpc\.Dial\w*\(`,
		CloseRegex: `(\w+)\.Close\(\)`,
		RiskPatterns: []string{
			"gRPC连接未关闭",
			"连接资源泄漏",
			"defer conn.Close()缺失",
			"服务端连接数过多",
		},
	},
	{
		Type:       "temp_dir",
		Name:       "临时目录",
		OpenRegex:  `(\w+),\s*err\s*:=\s*(?:ioutil\.TempDir|os\.MkdirTemp)\(`,
		CloseRegex: `os\.RemoveAll\((\w+)\)`,
		RiskPatterns: []string{
			"临时目录未清理",
			"磁盘空间泄漏",
			"defer清理语句缺失",
			"系统临时目录堆积",
		},
	},
	{
		Type:       "redis_conn",
		Name:       "Redis连接",
		OpenRegex:  `(\w+)\s*:=\s*\w*\.Get\(\)`,
		CloseRegex: `(\w+)\.Close\(\)`,
		RiskPatterns: []string{
			"Redis连接未归还连接池",
			"连接池资源耗尽",
			"defer conn.Close()缺失",
			"连接超时未处理",
		},
	},
	{
		Type:       "channel",
		Name:       "消息队列通道",
		OpenRegex:  `(\w+),\s*err\s*:=\s*\w*\.Channel\(\)`,
		CloseRegex: `(\w+)\.Close\(\)`,
		RiskPatterns: []string{
			"消息通道未关闭",
			"AMQP连接资源泄漏",
			"defer ch.Close()缺失",
			"消息队列连接数过多",
		},
	},
	{
		Type:       "zip_reader",
		Name:       "压缩文件读取",
		OpenRegex:  `(\w+),\s*err\s*:=\s*zip\.OpenReader\(`,
		CloseRegex: `(\w+)\.Close\(\)`,
		RiskPatterns: []string{
			"压缩文件读取器未关闭",
			"文件描述符泄漏",
			"defer reader.Close()缺失",
			"大文件处理内存泄漏",
		},
	},
	{
		Type:       "ticker",
		Name:       "定时器",
		OpenRegex:  `(\w+)\s*:=\s*time\.NewTicker\(`,
		CloseRegex: `(\w+)\.Stop\(\)`,
		RiskPatterns: []string{
			"定时器未停止",
			"goroutine泄漏风险",
			"defer ticker.Stop()缺失",
			"内存持续增长",
		},
	},
	{
		Type:       "buffer_pool",
		Name:       "缓冲池",
		OpenRegex:  `(\w+)\s*:=\s*\w*\.Get\(\)`,
		CloseRegex: `\w*\.Put\((\w+)\)`,
		RiskPatterns: []string{
			"缓冲区未归还池中",
			"内存池效率降低",
			"defer pool.Put()缺失",
			"内存使用不当",
		},
	},
	{
		Type:       "waitgroup",
		Name:       "等待组",
		OpenRegex:  `(\w+)\.Add\(\d+\)`,
		CloseRegex: `(\w+)\.Done\(\)`,
		RiskPatterns: []string{
			"WaitGroup计数不匹配",
			"goroutine永久阻塞",
			"defer wg.Done()缺失",
			"并发控制失效",
		},
	},
	{
		Type:       "panic_recovery",
		Name:       "Panic恢复机制",
		OpenRegex:  `defer\s+func\(\)\s*\{[^}]*recover\(\)`,
		CloseRegex: `\}\(\)`,
		RiskPatterns: []string{
			"panic未被正确捕获",
			"recover()调用位置不当",
			"错误信息丢失",
			"程序异常终止风险",
		},
	},
	{
		Type:       "slice_bounds",
		Name:       "切片边界检查",
		OpenRegex:  `if\s+len\((\w+)\)\s*[><=]+\s*\d+`,
		CloseRegex: `(\w+)\[.*\]`,
		RiskPatterns: []string{
			"数组/切片越界访问",
			"边界检查缺失",
			"panic: index out of range",
			"运行时崩溃风险",
		},
	},
	{
		Type:       "type_assertion",
		Name:       "类型断言安全",
		OpenRegex:  `(\w+),\s*(\w+)\s*:=\s*.*\.\(.*\)`,
		CloseRegex: `if\s+!(\w+)`,
		RiskPatterns: []string{
			"类型断言失败导致panic",
			"ok检查缺失",
			"类型转换不安全",
			"运行时类型错误",
		},
	},
	{
		Type:       "nil_pointer",
		Name:       "空指针检查",
		OpenRegex:  `if\s+(\w+)\s*!=\s*nil`,
		CloseRegex: `(\w+)\..*`,
		RiskPatterns: []string{
			"空指针解引用",
			"nil检查缺失",
			"panic: runtime error",
			"指针访问不安全",
		},
	},
}

// detectResourceScopes 检测代码中的资源作用域
func detectResourceScopes(fileContent string, changedLines []int) ([]ResourceScope, error) {
	lines := strings.Split(fileContent, "\n")
	var scopes []ResourceScope

	for _, pattern := range SupportedResourcePatterns {
		foundScopes := findResourceScopes(lines, pattern, changedLines)
		scopes = append(scopes, foundScopes...)
	}

	log.Printf("检测到 %d 个资源作用域", len(scopes))
	return scopes, nil
}

// findResourceScopes 查找特定模式的资源作用域
func findResourceScopes(lines []string, pattern ResourcePattern, changedLines []int) []ResourceScope {
	var scopes []ResourceScope
	openRegex := regexp.MustCompile(pattern.OpenRegex)
	closeRegex := regexp.MustCompile(pattern.CloseRegex)

	for i, line := range lines {
		if matches := openRegex.FindStringSubmatch(line); matches != nil {
			variable := ""
			if len(matches) > 1 {
				variable = matches[1] // 提取变量名
			}

			// 寻找对应的关闭语句
			closeLineIdx := findMatchingClose(lines, i, variable, closeRegex)
			if closeLineIdx > i {
				scope := ResourceScope{
					Type:         pattern.Type,
					StartLine:    i + 1,
					EndLine:      closeLineIdx + 1,
					OpenPattern:  strings.TrimSpace(line),
					ClosePattern: strings.TrimSpace(lines[closeLineIdx]),
					Variable:     variable,
				}

				// 检查变更是否在此作用域内
				for _, changedLine := range changedLines {
					if changedLine > scope.StartLine && changedLine < scope.EndLine {
						scope.HasChanges = true
						break
					}
				}

				scopes = append(scopes, scope)
				log.Printf("发现%s作用域: %d-%d行, 变量: %s, 包含变更: %v",
					pattern.Name, scope.StartLine, scope.EndLine, scope.Variable, scope.HasChanges)
			}
		}
	}

	return scopes
}

// findMatchingClose 查找匹配的关闭语句（找到最后一个匹配的）
func findMatchingClose(lines []string, startIdx int, variable string, closeRegex *regexp.Regexp) int {
	lastMatch := -1
	for i := startIdx + 1; i < len(lines); i++ {
		line := lines[i]
		if matches := closeRegex.FindStringSubmatch(line); matches != nil {
			// 如果有变量名，检查是否匹配
			if variable != "" && len(matches) > 1 {
				if matches[1] == variable {
					lastMatch = i
				}
			} else {
				// 没有变量名或无法提取变量名，直接匹配模式
				lastMatch = i
			}
		}
	}
	return lastMatch
}

// extractChangedLines 从Git diff中提取变更行信息
func extractChangedLines(filePath, baseCommit string) ([]ChangedLine, error) {
	var changedLines []ChangedLine

	// 读取当前文件内容
	fileContent, err := ioutil.ReadFile(filePath)
	if err != nil {
		return changedLines, fmt.Errorf("读取文件失败: %v", err)
	}
	fileLines := strings.Split(string(fileContent), "\n")

	// 获取文件的Git diff
	cmd := exec.Command("git", "diff", baseCommit, "HEAD", "--unified=3", filePath)
	cmd.Dir = filepath.Dir(filePath)
	output, err := cmd.Output()
	if err != nil {
		// 如果Git diff失败，尝试检查工作区变更
		cmd = exec.Command("git", "diff", "--unified=3", filePath)
		cmd.Dir = filepath.Dir(filePath)
		output, err = cmd.Output()
		if err != nil {
			return changedLines, fmt.Errorf("无法获取文件diff: %v", err)
		}
	}

	// 解析diff输出，提取变更行
	lines := strings.Split(string(output), "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "@@") {
			// 解析行号范围，如 @@ -10,5 +10,7 @@
			re := regexp.MustCompile(`@@ -\d+(?:,\d+)? \+(\d+)(?:,(\d+))? @@`)
			matches := re.FindStringSubmatch(line)
			if len(matches) >= 2 {
				startLine, _ := strconv.Atoi(matches[1])

				// 解析后续的diff行，提取实际变更内容
				for j := i + 1; j < len(lines); j++ {
					diffLine := lines[j]
					if strings.HasPrefix(diffLine, "@@") {
						break // 遇到下一个diff块
					}

					if strings.HasPrefix(diffLine, "+") && !strings.HasPrefix(diffLine, "+++") {
						// 新增行
						content := strings.TrimPrefix(diffLine, "+")
						lineNum := startLine + len(changedLines)
						changedLines = append(changedLines, ChangedLine{
							LineNumber: lineNum,
							Content:    content,
							Type:       "added",
						})
					} else if strings.HasPrefix(diffLine, "-") && !strings.HasPrefix(diffLine, "---") {
						// 删除行（记录但不计入行号）
						content := strings.TrimPrefix(diffLine, "-")
						changedLines = append(changedLines, ChangedLine{
							LineNumber: startLine,
							Content:    content,
							Type:       "deleted",
						})
					} else if strings.HasPrefix(diffLine, " ") {
						// 上下文行，跳过但调整行号计数
						startLine++
					}
				}
			}
		}
	}

	// 如果diff解析失败，回退到简单的行号范围检测
	if len(changedLines) == 0 {
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "@@") {
				re := regexp.MustCompile(`@@ -\d+(?:,\d+)? \+(\d+)(?:,(\d+))? @@`)
				matches := re.FindStringSubmatch(line)
				if len(matches) >= 2 {
					startLine, _ := strconv.Atoi(matches[1])
					count := 1
					if len(matches) > 2 && matches[2] != "" {
						count, _ = strconv.Atoi(matches[2])
					}

					// 从文件中读取实际内容
					for i := 0; i < count; i++ {
						lineNum := startLine + i
						content := ""
						if lineNum > 0 && lineNum <= len(fileLines) {
							content = fileLines[lineNum-1]
						}
						changedLines = append(changedLines, ChangedLine{
							LineNumber: lineNum,
							Content:    content,
							Type:       "modified",
						})
					}
				}
			}
		}
	}

	log.Printf("文件 %s 检测到 %d 个变更行", filePath, len(changedLines))
	return changedLines, nil
}

// buildEnhancedContext 构建增强的代码上下文
func buildEnhancedContext(filePath, baseCommit, strategy string) (*EnhancedContext, error) {
	// 读取文件内容
	fileContent, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("读取文件失败: %v", err)
	}

	// 提取变更行
	changedLines, err := extractChangedLines(filePath, baseCommit)
	if err != nil {
		return nil, fmt.Errorf("提取变更行失败: %v", err)
	}

	// 提取行号列表
	lineNumbers := make([]int, len(changedLines))
	for i, change := range changedLines {
		lineNumbers[i] = change.LineNumber
	}

	// 检测资源作用域
	scopes, err := detectResourceScopes(string(fileContent), lineNumbers)
	if err != nil {
		return nil, fmt.Errorf("检测资源作用域失败: %v", err)
	}

	// 构建上下文
	context := &EnhancedContext{
		ChangedLines:   changedLines,
		ResourceScopes: scopes,
		ProjectPath:    filepath.Dir(filePath),
		BaseCommit:     baseCommit,
		Strategy:       strategy,
	}

	// 为在资源作用域内的变更构建上下文块
	for _, scope := range scopes {
		if scope.HasChanges {
			scopeCode := extractLines(string(fileContent), scope.StartLine, scope.EndLine)
			context.ContextBlocks = append(context.ContextBlocks, ContextBlock{
				Type:      scope.Type,
				StartLine: scope.StartLine,
				EndLine:   scope.EndLine,
				Code:      scopeCode,
				Reason:    fmt.Sprintf("变更代码位于%s作用域内", scope.Type),
			})
		}
	}

	log.Printf("构建增强上下文完成: %d个变更行, %d个资源作用域, %d个上下文块",
		len(context.ChangedLines), len(context.ResourceScopes), len(context.ContextBlocks))

	return context, nil
}

// extractLines 提取指定行范围的代码
func extractLines(content string, startLine, endLine int) string {
	lines := strings.Split(content, "\n")
	if startLine < 1 || endLine > len(lines) || startLine > endLine {
		return ""
	}

	var result []string
	for i := startLine - 1; i < endLine; i++ {
		result = append(result, fmt.Sprintf("%d: %s", i+1, lines[i]))
	}
	return strings.Join(result, "\n")
}

// ==================== 智能上下文分析 ====================

// assessRiskLevel 评估风险等级
func assessRiskLevel(scopes []ResourceScope, changedLines []ChangedLine) string {
	highRiskCount := 0
	mediumRiskCount := 0
	lowRiskCount := 0

	// 基于资源作用域评估
	for _, scope := range scopes {
		if scope.HasChanges {
			switch scope.Type {
			case "transaction", "mutex", "waitgroup":
				highRiskCount++
			case "sql_rows", "grpc_conn", "redis_conn":
				highRiskCount++
			case "file", "http_response", "channel":
				mediumRiskCount++
			case "context", "ticker", "zip_reader":
				mediumRiskCount++
			case "temp_dir", "buffer_pool":
				lowRiskCount++
			}
		}
	}

	// 基于变更行数量和内容评估
	if len(changedLines) > 50 {
		mediumRiskCount++ // 大量变更增加风险
	}

	// 检查变更内容中的高风险关键词
	for _, line := range changedLines {
		content := strings.ToLower(line.Content)
		// 高风险：panic、数组越界、空指针、并发问题
		if strings.Contains(content, "panic") ||
			strings.Contains(content, "unsafe") ||
			strings.Contains(content, "goroutine") ||
			strings.Contains(content, "go func") ||
			strings.Contains(content, "[") && strings.Contains(content, "]") || // 数组/切片访问
			strings.Contains(content, "*") && strings.Contains(content, ".") || // 指针解引用
			strings.Contains(content, ".(") || // 类型断言
			strings.Contains(content, "must") || // MustXxx函数
			strings.Contains(content, "fatal") {
			highRiskCount++
		} else if strings.Contains(content, "defer") ||
			strings.Contains(content, "close") ||
			strings.Contains(content, "commit") ||
			strings.Contains(content, "rollback") ||
			strings.Contains(content, "recover") || // panic恢复
			strings.Contains(content, "len(") || // 长度检查
			strings.Contains(content, "cap(") || // 容量检查
			strings.Contains(content, "nil") { // nil检查
			mediumRiskCount++
		}
	}

	if highRiskCount > 0 {
		return "high"
	}
	if mediumRiskCount > 0 {
		return "medium"
	}
	if lowRiskCount > 0 || len(changedLines) > 10 {
		return "low"
	}
	return "none"
}

// identifyFocusAreas 识别需要重点关注的领域
func identifyFocusAreas(scopes []ResourceScope) []string {
	focusMap := make(map[string]bool)

	for _, scope := range scopes {
		if scope.HasChanges {
			switch scope.Type {
			case "transaction":
				focusMap["transaction"] = true
			case "mutex", "waitgroup":
				focusMap["concurrency"] = true
			case "file", "http_response", "grpc_conn", "redis_conn", "channel", "zip_reader", "temp_dir", "buffer_pool":
				focusMap["resource"] = true
			case "context", "ticker":
				focusMap["concurrency"] = true
			case "panic_recovery", "slice_bounds", "type_assertion", "nil_pointer":
				focusMap["panic_safety"] = true
			case "sql_rows":
				focusMap["transaction"] = true
				focusMap["resource"] = true
			}
		}
	}

	var areas []string
	for area := range focusMap {
		areas = append(areas, area)
	}

	return areas
}

// generateSimplifiedPrompt 生成精简的AI提示词模板（已废弃）
func generateSimplifiedPrompt(files []FileAnalysis, scopes []ResourceScope, focusAreas []string) string {
	return "代码审查工具已重构，不再生成提示词"
}

// 已删除不再需要的函数

// detectRiskPoints 检测代码风险点
func detectRiskPoints(filePath string, changedLines []ChangedLine, scopes []ResourceScope) []RiskPoint {
	var riskPoints []RiskPoint
	riskID := 1

	// 1. 基于变更行内容检测风险
	for _, line := range changedLines {
		content := strings.TrimSpace(line.Content)
		if content == "" {
			continue
		}

		contentLower := strings.ToLower(content)

		// 检测各种风险模式
		risks := []struct {
			pattern     string
			riskType    string
			category    string
			severity    string
			description string
			suggestion  string
		}{
			// Panic 风险
			{"panic(", "panic_call", "panic_safety", "high", "直接调用panic可能导致程序崩溃", "使用错误返回值替代panic，或添加recover机制"},
			{"must", "panic_risk", "panic_safety", "medium", "MustXxx函数可能触发panic", "检查函数文档，添加错误处理"},
			{"[", "bounds_risk", "panic_safety", "medium", "数组/切片访问可能越界", "添加边界检查: if index < len(arr)"},
			{".(", "type_assertion", "panic_safety", "medium", "类型断言失败可能panic", "使用安全的类型断言: val, ok := x.(Type)"},

			// 事务风险
			{"begin(", "transaction_start", "transaction", "high", "事务开启，需确保正确关闭", "添加defer tx.Rollback()或在成功时调用tx.Commit()"},
			{"commit(", "transaction_commit", "transaction", "medium", "事务提交，需确保错误处理", "检查commit返回的错误"},
			{"rollback(", "transaction_rollback", "transaction", "low", "事务回滚操作", "确保在适当的错误路径中调用"},

			// 资源风险
			{"open(", "resource_open", "resource", "medium", "资源打开，需确保关闭", "添加defer file.Close()"},
			{"close()", "resource_close", "resource", "low", "资源关闭操作", "确保资源正确关闭"},

			// 并发风险
			{"go func", "goroutine_start", "concurrency", "medium", "启动goroutine，注意资源管理", "确保goroutine能正常退出，避免泄漏"},
			{"lock()", "mutex_lock", "concurrency", "medium", "获取锁，需确保释放", "添加defer mu.Unlock()"},
			{"unlock()", "mutex_unlock", "concurrency", "low", "释放锁操作", "确保锁的获取和释放匹配"},
		}

		for _, risk := range risks {
			if strings.Contains(contentLower, risk.pattern) {
				riskPoint := RiskPoint{
					ID:          fmt.Sprintf("%s_%d_%d", risk.riskType, line.LineNumber, riskID),
					Type:        risk.riskType,
					Category:    risk.category,
					Line:        line.LineNumber,
					Severity:    risk.severity,
					Description: risk.description,
					Context:     content,
					Suggestion:  risk.suggestion,
				}
				riskPoints = append(riskPoints, riskPoint)
				riskID++
			}
		}
	}

	// 2. 基于资源作用域检测风险
	for _, scope := range scopes {
		if scope.HasChanges {
			var riskType, description, suggestion string
			severity := "medium"

			switch scope.Type {
			case "transaction":
				riskType = "transaction_scope_change"
				description = "事务作用域内有代码变更，需检查事务完整性"
				suggestion = "确保变更不会影响事务的正确提交或回滚"
			case "mutex":
				riskType = "mutex_scope_change"
				description = "锁作用域内有代码变更，需检查并发安全"
				suggestion = "确保变更不会引入竞态条件或死锁"
				severity = "high"
			case "panic_recovery":
				riskType = "panic_recovery_change"
				description = "panic恢复机制内有变更，需检查错误处理"
				suggestion = "确保panic能被正确捕获和处理"
				severity = "high"
			default:
				riskType = "resource_scope_change"
				description = fmt.Sprintf("%s资源作用域内有变更", scope.Type)
				suggestion = "检查资源的正确获取和释放"
			}

			riskPoint := RiskPoint{
				ID:          fmt.Sprintf("%s_%s_%d", riskType, scope.ID, riskID),
				Type:        riskType,
				Category:    getCategoryByType(scope.Type),
				Line:        scope.StartLine,
				Severity:    severity,
				Description: description,
				Context:     fmt.Sprintf("%s作用域 (第%d-%d行)", scope.Type, scope.StartLine, scope.EndLine),
				Suggestion:  suggestion,
			}
			riskPoints = append(riskPoints, riskPoint)
			riskID++
		}
	}

	return riskPoints
}

// getCategoryByType 根据类型获取分类
func getCategoryByType(scopeType string) string {
	switch scopeType {
	case "transaction", "sql_rows":
		return "transaction"
	case "mutex", "waitgroup", "context", "ticker":
		return "concurrency"
	case "panic_recovery", "slice_bounds", "type_assertion", "nil_pointer":
		return "panic_safety"
	default:
		return "resource"
	}
}

// assessFileRiskLevel 评估单个文件的风险等级
func assessFileRiskLevel(riskPoints []RiskPoint) string {
	if len(riskPoints) == 0 {
		return "none"
	}

	highCount := 0
	mediumCount := 0

	for _, risk := range riskPoints {
		switch risk.Severity {
		case "high":
			highCount++
		case "medium":
			mediumCount++
		}
	}

	if highCount > 0 {
		return "high"
	}
	if mediumCount > 0 {
		return "medium"
	}
	return "low"
}

// 已删除 generateAIInstructions 函数（不再需要）

// ==================== 主处理函数 ====================

// handleCodeReview 处理代码审查请求（重构版 - 专注风险检测）
func handleCodeReview(request *CodeReviewRequest) (*mcp.CallToolResult, error) {
	startTime := time.Now()

	log.Printf("开始代码审查分析，项目路径: %s", request.ProjectPath)

	// 1. 获取变更的Go文件
	changedFiles, baseCommit, _, err := getChangedGoFiles(request.ProjectPath)
	if err != nil {
		return nil, fmt.Errorf("获取变更文件失败: %v", err)
	}

	if len(changedFiles) == 0 {
		log.Printf("未检测到Go文件变更")
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{
				Type: "text",
				Text: `{"summary":{"totalChangedFiles":0,"totalRiskPoints":0,"riskLevel":"none"},"changedFiles":[],"processingTime":"0ms"}`,
			}},
		}, nil
	}

	// 2. 分析每个变更文件，检测风险点
	var allFileAnalyses []FileAnalysis
	totalRiskPoints := 0

	for _, filePath := range changedFiles {
		log.Printf("分析文件: %s", filePath)

		// 获取变更行
		changedLines, err := extractChangedLines(filePath, baseCommit)
		if err != nil {
			log.Printf("提取文件 %s 的变更行失败: %v", filePath, err)
			continue
		}

		// 如果没有变更行，跳过
		if len(changedLines) == 0 {
			log.Printf("文件 %s 无变更行", filePath)
			continue
		}

		// 检测资源作用域
		fileContent, err := ioutil.ReadFile(filePath)
		if err != nil {
			log.Printf("读取文件 %s 失败: %v", filePath, err)
			continue
		}

		changedLineNumbers := make([]int, len(changedLines))
		for i, line := range changedLines {
			changedLineNumbers[i] = line.LineNumber
		}

		scopes, err := detectResourceScopes(string(fileContent), changedLineNumbers)
		if err != nil {
			log.Printf("检测文件 %s 的资源作用域失败: %v", filePath, err)
			scopes = []ResourceScope{} // 继续处理，但不包含作用域
		}

		// 检测风险点
		riskPoints := detectRiskPoints(filePath, changedLines, scopes)

		// 评估文件风险等级
		riskLevel := assessFileRiskLevel(riskPoints)

		// 创建文件分析结果
		fileAnalysis := FileAnalysis{
			File:         filePath,
			ChangedLines: changedLines,
			RiskPoints:   riskPoints,
			RiskLevel:    riskLevel,
		}

		allFileAnalyses = append(allFileAnalyses, fileAnalysis)
		totalRiskPoints += len(riskPoints)

		log.Printf("文件 %s 分析完成，发现 %d 个风险点", filePath, len(riskPoints))
	}

	// 3. 创建概览摘要（专注风险统计）
	riskByCategory := make(map[string]int)
	riskBySeverity := make(map[string]int)
	overallRiskLevel := "none"

	// 统计风险点
	for _, file := range allFileAnalyses {
		for _, risk := range file.RiskPoints {
			riskByCategory[risk.Category]++
			riskBySeverity[risk.Severity]++
		}

		// 更新整体风险等级
		if file.RiskLevel == "high" {
			overallRiskLevel = "high"
		} else if file.RiskLevel == "medium" && overallRiskLevel != "high" {
			overallRiskLevel = "medium"
		} else if file.RiskLevel == "low" && overallRiskLevel == "none" {
			overallRiskLevel = "low"
		}
	}

	summary := ReviewSummary{
		TotalChangedFiles: len(allFileAnalyses),
		TotalRiskPoints:   totalRiskPoints,
		RiskLevel:         overallRiskLevel,
		RiskByCategory:    riskByCategory,
		RiskBySeverity:    riskBySeverity,
		ProcessingTime:    time.Since(startTime).String(),
	}

	// 4. 构建最终结果（精简版）
	result := CodeReviewOutput{
		Summary:        summary,
		ChangedFiles:   allFileAnalyses,
		ProcessingTime: time.Since(startTime).String(),
	}

	log.Printf("代码审查分析完成，处理 %d 个文件，发现 %d 个风险点，处理时间: %s",
		len(allFileAnalyses), totalRiskPoints, result.ProcessingTime)

	// 5. 返回结果
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("序列化结果失败: %v", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Type: "text", Text: string(resultJSON)}},
	}, nil
}

func main() {
	// 设置日志输出到文件
	logFile, err := os.OpenFile("/tmp/lint-mcp-debug.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		// 如果无法创建日志文件，使用stderr
		log.SetOutput(os.Stderr)
	} else {
		log.SetOutput(logFile)
		defer logFile.Close()
	}

	// 设置日志输出格式，包含时间和文件位置
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("🚀 启动 lint-mcp 服务 (兼容版本)...")
	log.Printf("📝 日志文件: /tmp/lint-mcp-debug.log")
	log.Printf("🕐 启动时间: %s", time.Now().Format("2006-01-02 15:04:05"))

	s := server.NewMCPServer(
		"lint-mcp",
		"1.0.16",
	)

	// 注册 code_lint 工具
	tool := mcp.NewTool("code_lint",
		mcp.WithDescription("智能Go代码检查工具。自动检测变更代码并进行精确的包级行级检查，避免历史代码问题干扰。支持多项目处理和智能依赖模式检测。"),
		mcp.WithString("projectPath",
			mcp.Description("项目根目录（可选，优先作为检测起点，建议为Git仓库或包含go.mod的目录）"),
		),
	)

	s.AddTool(tool, handleCodeLintRequest)

	log.Println("工具注册成功: code_lint")

	// 注册 code_review 工具
	reviewTool := mcp.NewTool("code_review",
		mcp.WithDescription("智能代码上下文分析工具。基于变更检测，识别资源作用域，为外部AI工具提供精准的代码上下文和结构化分析数据。专注于并发、事务、资源管理等关键领域。"),
		mcp.WithString("projectPath",
			mcp.Description("项目根目录（可选，优先作为检测起点，建议为Git仓库或包含go.mod的目录）"),
		),
		mcp.WithArray("reviewFocus",
			mcp.Description("关注点列表，可选值：concurrency(并发安全)、transaction(事务管理)、resource(资源管理)、performance(性能问题)。默认全部"),
		),
	)

	s.AddTool(reviewTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var request CodeReviewRequest

		// 手动解析参数
		if projectPath, ok := req.Params.Arguments["projectPath"].(string); ok {
			request.ProjectPath = projectPath
		}
		if reviewFocus, ok := req.Params.Arguments["reviewFocus"].([]interface{}); ok {
			for _, rf := range reviewFocus {
				if rfStr, ok := rf.(string); ok {
					request.ReviewFocus = append(request.ReviewFocus, rfStr)
				}
			}
		}

		return handleCodeReview(&request)
	})

	log.Println("工具注册成功: code_review")

	// 启动时检查golangci-lint版本兼容性
	if err := checkGolangciLintInstalled(); err != nil {
		log.Printf("❌ golangci-lint 版本检查失败: %v", err)
		log.Println("请按照提示升级golangci-lint后重启服务")
		return
	}
	log.Println("✅ golangci-lint 版本检查通过")

	log.Println("服务就绪，等待连接...")

	if err := server.ServeStdio(s); err != nil {
		log.Printf("服务错误: %v\n", err)
	}
}
