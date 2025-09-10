package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
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

	// 检查是否支持JSON输出（v2.4.0+支持--output.json.path）
	supportsOutputJson := strings.Contains(helpStr, "--output.json.path")

	if !supportsOutputJson {
		return fmt.Errorf(`golangci-lint 版本不兼容

当前版本: %s
要求版本: v2.4.0 或更高版本

请升级到兼容版本：
go install github.com/golangci/golangci-lint/cmd/golangci-lint@v2.4.0

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
