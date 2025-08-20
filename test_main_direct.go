package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// 复制相关结构体和函数
type CodeLintRequest struct {
	Files            []string `json:"files" description:"参考文件列表（可选，用于确定检查起点）。当checkOnlyChanges=true时，将智能检测当前工作目录的所有变更文件。" required:"false"`
	ProjectPath      string   `json:"projectPath" description:"项目根目录（可选，优先作为检测起点，建议为Git仓库或包含go.mod的目录）"`
	CheckOnlyChanges bool     `json:"checkOnlyChanges" description:"是否启用智能变更检测（默认true）。将自动检测Git变更范围：未推送提交、分支分叉点或工作区变更。" default:"true"`
}

type Issue struct {
	FromLinter string `json:"FromLinter"`
	Text       string `json:"Text"`
	Pos        Pos    `json:"Pos"`
}

type Pos struct {
	Filename string `json:"Filename"`
	Line     int    `json:"Line"`
	Column   int    `json:"Column"`
}

type LintResult struct {
	Issues []Issue `json:"Issues"`
}

type GolangciLintOutput struct {
	Issues []Issue `json:"Issues"`
}

// buildErrorResult 统一将错误以 JSON Issues 形式返回
func buildErrorResult(message string) *LintResult {
	return &LintResult{Issues: []Issue{{
		FromLinter: "lint-mcp",
		Text:       message,
		Pos:        Pos{Filename: "system", Line: 0, Column: 0},
	}}}
}

// autoDetectVendorMode 自动检测项目的依赖模式
func autoDetectVendorMode(projectRoot string) bool {
	log.Printf("检测项目 %s 的vendor模式", projectRoot)

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
		clean := strings.TrimPrefix(strings.TrimSuffix(line, "/"), "/")
		if clean == "vendor" {
			log.Printf(".gitignore 含完整 vendor/ 目录忽略（行: %s），使用 modules 模式", line)
			return false
		}
	}

	log.Printf(".gitignore 未包含完整 vendor/ 目录忽略，使用 vendor 模式")
	return true
}

// getChangedGoFiles 获取变更的Go文件
func getChangedGoFiles(projectRoot string) ([]string, error) {
	log.Printf("检测项目 %s 的变更文件", projectRoot)

	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = projectRoot
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git status 失败: %v", err)
	}

	var files []string
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if len(line) < 3 {
			continue
		}
		filename := strings.TrimSpace(line[2:])
		if strings.HasSuffix(filename, ".go") &&
			!strings.Contains(filename, "_test.go") &&
			!strings.HasPrefix(filename, "test_") &&
			!strings.HasPrefix(filename, "debug_") &&
			!strings.HasPrefix(filename, "simple_") {
			absPath := filepath.Join(projectRoot, filename)
			files = append(files, absPath)
		}
	}

	return files, nil
}

// getProjectRootFromFile 从文件路径获取项目根目录
func getProjectRootFromFile(filePath string) (string, error) {
	dir := filepath.Dir(filePath)
	for {
		// 检查是否存在 go.mod
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", fmt.Errorf("未找到 go.mod 文件")
}

// runGolangciLintWithArgs 运行 golangci-lint
func runGolangciLintWithArgs(projectRoot string, args []string) (*LintResult, error) {
	log.Printf("执行命令: golangci-lint %v", args)
	log.Printf("命令执行目录: %s", projectRoot)

	cmd := exec.Command("golangci-lint", args...)
	cmd.Dir = projectRoot
	cmd.Env = os.Environ()

	output, cmdErr := cmd.CombinedOutput()
	log.Printf("命令输出长度: %d", len(output))
	log.Printf("命令执行错误: %v", cmdErr)

	if len(output) == 0 {
		if cmdErr != nil {
			log.Printf("命令无输出且有错误: %v", cmdErr)
			return nil, fmt.Errorf("golangci-lint 执行失败: %v", cmdErr)
		}
		return &LintResult{Issues: make([]Issue, 0)}, nil
	}

	var golangciOutput GolangciLintOutput
	if err := json.Unmarshal(output, &golangciOutput); err != nil {
		log.Printf("JSON解析失败: %v，原始输出: %s", err, string(output))
		return &LintResult{Issues: make([]Issue, 0)}, nil
	}

	log.Printf("成功解析到 %d 个问题", len(golangciOutput.Issues))
	return &LintResult{Issues: golangciOutput.Issues}, nil
}

// 核心处理函数
func handleCodeLintRequest(lintReq CodeLintRequest) *LintResult {
	log.Printf("处理请求: %+v", lintReq)

	// 计算检测起点目录
	baseDir := ""
	if lintReq.ProjectPath != "" {
		abs, err := filepath.Abs(lintReq.ProjectPath)
		if err != nil {
			return buildErrorResult(fmt.Sprintf("projectPath 解析失败: %v", err))
		}
		if stat, err := os.Stat(abs); err != nil || !stat.IsDir() {
			return buildErrorResult(fmt.Sprintf("projectPath 无效或不是目录: %s", abs))
		}
		baseDir = abs
	} else {
		return buildErrorResult("缺少 projectPath 参数")
	}

	log.Printf("检测起点目录: %s", baseDir)

	// 如果 checkOnlyChanges=true，智能检测变更文件
	if lintReq.CheckOnlyChanges {
		log.Printf("checkOnlyChanges=true，智能检测变更文件")

		changedFiles, err := getChangedGoFiles(baseDir)
		if err != nil {
			return buildErrorResult(fmt.Sprintf("获取变更文件失败: %v", err))
		}

		log.Printf("检测到 %d 个变更的 Go 文件", len(changedFiles))
		for _, file := range changedFiles {
			log.Printf("  - %s", file)
		}

		// 按项目分组变更文件
		projectFiles := make(map[string][]string)
		for _, file := range changedFiles {
			projectRoot, err := getProjectRootFromFile(file)
			if err != nil {
				log.Printf("警告：无法确定文件 %s 的项目根目录：%v", file, err)
				continue
			}
			projectFiles[projectRoot] = append(projectFiles[projectRoot], file)
		}

		// 对每个项目的变更文件进行检查
		allIssues := make([]Issue, 0)
		for projectRoot, files := range projectFiles {
			log.Printf("检查项目 %s 中的 %d 个变更文件", projectRoot, len(files))

			vendorMode := autoDetectVendorMode(projectRoot)
			log.Printf("项目 %s 使用 vendor 模式: %v", projectRoot, vendorMode)

			for _, file := range files {
				log.Printf("开始检查文件: %s (项目: %s, vendorMode: %v)", file, projectRoot, vendorMode)

				// 构建参数
				args := []string{"run", "--out-format", "json"}
				if vendorMode {
					args = append(args, "--modules-download-mode=vendor")
				}
				args = append(args, file)

				result, err := runGolangciLintWithArgs(projectRoot, args)
				if err != nil {
					log.Printf("检查文件 %s 失败: %v", file, err)
					continue
				}

				if result != nil && len(result.Issues) > 0 {
					log.Printf("文件 %s 发现 %d 个问题", file, len(result.Issues))
					allIssues = append(allIssues, result.Issues...)
				} else {
					log.Printf("文件 %s 未发现问题", file)
				}
			}
		}

		return &LintResult{Issues: allIssues}
	}

	return buildErrorResult("不支持 checkOnlyChanges=false 的情况")
}

func main() {
	// 测试请求
	testReq := CodeLintRequest{
		ProjectPath:      "/Users/gds/goS/src/scotty",
		CheckOnlyChanges: true,
	}

	log.Printf("=== 直接测试 lint-mcp 核心逻辑 ===")
	log.Printf("测试请求: %+v", testReq)

	result := handleCodeLintRequest(testReq)

	// 输出结果
	resultJSON, _ := json.MarshalIndent(result, "", "  ")
	fmt.Printf("\n=== 最终结果 ===\n%s\n", string(resultJSON))

	log.Printf("=== 测试完成 ===")
	log.Printf("发现问题总数: %d", len(result.Issues))
}
