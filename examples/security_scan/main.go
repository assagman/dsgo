package main

import (
	"bufio"
	"context"
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"github.com/assagman/dsgo"
	"github.com/assagman/dsgo/examples/shared/tools"
)

const (
	// ScanModel is the model used for all security scan modules
	ScanModel = "openrouter/google/gemini-2.5-flash-lite-preview-09-2025"

	// MaxFileBytes limits file content to prevent excessive token usage
	MaxFileBytes = 128 * 1024

	// MaxRetries is the maximum number of retry attempts for failed scans
	MaxRetries = 3
)

// GoFile represents a discovered Go source file
type GoFile struct {
	RelativePath string // e.g. "internal/core/lm.go"
	PackageDir   string // e.g. "internal/core"
	PackageName  string // e.g. "core"
}

// PackageInfo represents a Go package with its files
type PackageInfo struct {
	Path        string // same as PackageDir
	Name        string
	FileIndices []int // indices into []GoFile
}

// FileSecurityScan contains the security scan result for a single Go file
type FileSecurityScan struct {
	File            GoFile
	Summary         string
	Vulnerabilities string
	SecurityRisks   string
	Recommendations string
	Severity        string // LOW, MEDIUM, HIGH, CRITICAL
}

// PackageSecurityScan contains the security scan result for a Go package
type PackageSecurityScan struct {
	Package         PackageInfo
	Summary         string
	Vulnerabilities string
	SecurityRisks   string
	Recommendations string
	OverallSeverity string // LOW, MEDIUM, HIGH, CRITICAL
}

// detectPackageName extracts the Go package name from a source file
func detectPackageName(path string) string {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, parser.PackageClauseOnly)
	if err != nil || f == nil || f.Name == nil {
		// Fallback: last dir element
		dir := filepath.Base(filepath.Dir(path))
		if dir == "." || dir == string(os.PathSeparator) {
			return "main"
		}
		return dir
	}
	return f.Name.Name
}

// discoverGoFiles finds all .go files in the project directory
func discoverGoFiles(root string) ([]GoFile, error) {
	ignoreDirs := map[string]bool{
		".git": true, "vendor": true, "node_modules": true,
		".idea": true, ".vscode": true, "dist": true,
	}

	var files []GoFile
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if ignoreDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}

		pkgName := detectPackageName(path)
		pkgDir := filepath.Dir(rel)

		files = append(files, GoFile{
			RelativePath: filepath.ToSlash(rel),
			PackageDir:   filepath.ToSlash(pkgDir),
			PackageName:  pkgName,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}

// groupFilesByPackage groups Go files by their package directory
func groupFilesByPackage(files []GoFile) []PackageInfo {
	byPath := make(map[string]*PackageInfo)

	for idx, f := range files {
		key := f.PackageDir
		if key == "." {
			key = ""
		}
		pkg, ok := byPath[key]
		if !ok {
			pkg = &PackageInfo{
				Path:        key, // e.g. "internal/core"
				Name:        f.PackageName,
				FileIndices: []int{},
			}
			byPath[key] = pkg
		}
		pkg.FileIndices = append(pkg.FileIndices, idx)
	}

	result := make([]PackageInfo, 0, len(byPath))
	for _, pkg := range byPath {
		result = append(result, *pkg)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Path < result[j].Path
	})
	return result
}

// parseModulePath extracts the Go module path from go.mod
func parseModulePath(goModPath string) string {
	file, err := os.Open(goModPath)
	if err != nil {
		return "unknown-module"
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if after, ok := strings.CutPrefix(line, "module "); ok {
			return strings.TrimSpace(after)
		}
	}
	return "unknown-module"
}

// buildFileSecurityScanInputs prepares inputs for file-level parallel security scan
func buildFileSecurityScanInputs(root string, files []GoFile) (map[string]any, error) {
	n := len(files)
	paths := make([]string, n)
	pkgPaths := make([]string, n)
	pkgNames := make([]string, n)
	contents := make([]string, n)

	for i, f := range files {
		full := filepath.Join(root, filepath.FromSlash(f.RelativePath))
		data, err := os.ReadFile(full)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", f.RelativePath, err)
		}
		if len(data) > MaxFileBytes {
			data = append(data[:MaxFileBytes], []byte("\n... [truncated]\n")...)
		}
		paths[i] = f.RelativePath
		pkgPaths[i] = f.PackageDir
		pkgNames[i] = f.PackageName
		contents[i] = string(data)
	}

	return map[string]any{
		"file_path":     paths,
		"package_path":  pkgPaths,
		"package_name":  pkgNames,
		"file_contents": contents,
	}, nil
}

// collectFileSecurityScans extracts file security scans from prediction results
// Returns successful scans and indices of failed files for retry
func collectFileSecurityScans(files []GoFile, pred *dsgo.Prediction) ([]FileSecurityScan, []int) {
	scans := make([]FileSecurityScan, 0, len(files))
	failedIndices := []int{}

	if pred.Completions == nil {
		// All files failed
		for i := range files {
			failedIndices = append(failedIndices, i)
		}
		return scans, failedIndices
	}

	for i, f := range files {
		if i >= len(pred.Completions) {
			failedIndices = append(failedIndices, i)
			continue
		}
		c := pred.Completions[i]
		s := FileSecurityScan{File: f}

		// Check if we got valid outputs
		hasValidOutput := false
		if v, ok := c["summary"].(string); ok && v != "" {
			s.Summary = v
			hasValidOutput = true
		}
		if v, ok := c["vulnerabilities"].(string); ok && v != "" {
			s.Vulnerabilities = v
			hasValidOutput = true
		}
		if v, ok := c["security_risks"].(string); ok && v != "" {
			s.SecurityRisks = v
			hasValidOutput = true
		}
		if v, ok := c["recommendations"].(string); ok && v != "" {
			s.Recommendations = v
			hasValidOutput = true
		}
		if v, ok := c["severity"].(string); ok && v != "" {
			s.Severity = v
			hasValidOutput = true
		}

		if hasValidOutput {
			scans = append(scans, s)
		} else {
			failedIndices = append(failedIndices, i)
		}
	}
	return scans, failedIndices
}

// groupFileSecurityScansByPackage aggregates file security scans by package
func groupFileSecurityScansByPackage(pkgs []PackageInfo, files []GoFile, fileScans []FileSecurityScan) map[string]string {
	byPkg := make(map[string][]FileSecurityScan)

	// index FileSecurityScan by file path for quick lookup
	byPath := make(map[string]FileSecurityScan, len(fileScans))
	for _, fs := range fileScans {
		byPath[fs.File.RelativePath] = fs
	}

	for _, pkg := range pkgs {
		for _, idx := range pkg.FileIndices {
			f := files[idx]
			if fs, ok := byPath[f.RelativePath]; ok {
				byPkg[pkg.Path] = append(byPkg[pkg.Path], fs)
			}
		}
	}

	result := make(map[string]string, len(byPkg))
	for pkgPath, list := range byPkg {
		var b strings.Builder
		fmt.Fprintf(&b, "PACKAGE %s\n", pkgPath)
		for _, fs := range list {
			fmt.Fprintf(&b, "\n### File: %s\n", fs.File.RelativePath)
			fmt.Fprintf(&b, "Severity: %s\n\n", fs.Severity)
			fmt.Fprintf(&b, "Summary:\n%s\n\n", fs.Summary)
			fmt.Fprintf(&b, "Vulnerabilities:\n%s\n\n", fs.Vulnerabilities)
			fmt.Fprintf(&b, "Security Risks:\n%s\n\n", fs.SecurityRisks)
			fmt.Fprintf(&b, "Recommendations:\n%s\n\n", fs.Recommendations)
			b.WriteString("---\n")
		}
		result[pkgPath] = b.String()
	}
	return result
}

// buildPackageSecurityScanInputs prepares inputs for package-level parallel security scan
func buildPackageSecurityScanInputs(pkgs []PackageInfo, fileScanText map[string]string) map[string]any {
	n := len(pkgs)
	paths := make([]string, n)
	names := make([]string, n)
	scans := make([]string, n)

	for i, p := range pkgs {
		paths[i] = p.Path
		names[i] = p.Name
		scans[i] = fileScanText[p.Path]
	}
	return map[string]any{
		"package_path": paths,
		"package_name": names,
		"file_scans":   scans,
	}
}

// collectPackageSecurityScans extracts package security scans from prediction results
// Returns successful scans and indices of failed packages for retry
func collectPackageSecurityScans(pkgs []PackageInfo, pred *dsgo.Prediction) ([]PackageSecurityScan, []int) {
	scans := make([]PackageSecurityScan, 0, len(pkgs))
	failedIndices := []int{}

	if pred.Completions == nil {
		// All packages failed
		for i := range pkgs {
			failedIndices = append(failedIndices, i)
		}
		return scans, failedIndices
	}

	for i, pkg := range pkgs {
		if i >= len(pred.Completions) {
			failedIndices = append(failedIndices, i)
			continue
		}
		c := pred.Completions[i]
		ps := PackageSecurityScan{Package: pkg}

		// Check if we got valid outputs
		hasValidOutput := false
		if v, ok := c["package_summary"].(string); ok && v != "" {
			ps.Summary = v
			hasValidOutput = true
		}
		if v, ok := c["vulnerabilities"].(string); ok && v != "" {
			ps.Vulnerabilities = v
			hasValidOutput = true
		}
		if v, ok := c["security_risks"].(string); ok && v != "" {
			ps.SecurityRisks = v
			hasValidOutput = true
		}
		if v, ok := c["recommendations"].(string); ok && v != "" {
			ps.Recommendations = v
			hasValidOutput = true
		}
		if v, ok := c["overall_severity"].(string); ok && v != "" {
			ps.OverallSeverity = v
			hasValidOutput = true
		}

		if hasValidOutput {
			scans = append(scans, ps)
		} else {
			failedIndices = append(failedIndices, i)
		}
	}
	return scans, failedIndices
}

// buildProjectSecurityScanInput creates the combined input for project-level security scan
func buildProjectSecurityScanInput(root, modulePath string, pkgScans []PackageSecurityScan) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Project root: %s\n", root)
	fmt.Fprintf(&b, "Module path: %s\n\n", modulePath)
	for _, ps := range pkgScans {
		fmt.Fprintf(&b, "=== Package %s (name: %s) ===\n", ps.Package.Path, ps.Package.Name)
		fmt.Fprintf(&b, "Summary:\n%s\n\n", ps.Summary)
		fmt.Fprintf(&b, "Vulnerabilities:\n%s\n\n", ps.Vulnerabilities)
		fmt.Fprintf(&b, "Security Risks:\n%s\n\n", ps.SecurityRisks)
		fmt.Fprintf(&b, "Recommendations:\n%s\n\n", ps.Recommendations)
		fmt.Fprintf(&b, "Overall Severity: %s\n\n", ps.OverallSeverity)
		b.WriteString("========================================\n\n")
	}
	return b.String()
}

// pickBestProjectSecurityScan selects the best completion from multiple candidates
func pickBestProjectSecurityScan(pred *dsgo.Prediction) map[string]any {
	if len(pred.Completions) == 0 {
		return nil
	}
	bestIdx := 0
	bestScore := -1
	for i, c := range pred.Completions {
		exec, _ := c["executive_summary"].(string)
		vulns, _ := c["critical_vulnerabilities"].(string)
		risks, _ := c["security_posture"].(string)
		recs, _ := c["immediate_actions"].(string)
		score := len(exec) + len(vulns) + len(risks) + len(recs)
		if score > bestScore {
			bestScore = score
			bestIdx = i
		}
	}
	return pred.Completions[bestIdx]
}

// getMaxFilesFromEnv returns the maximum number of files to process, or 0 for unlimited
func getMaxFilesFromEnv() int {
	if val := os.Getenv("SECURITY_SCAN_MAX_FILES"); val != "" {
		if n, err := strconv.Atoi(val); err == nil && n > 0 {
			return n
		}
	}
	return 0 // unlimited
}

func main() {
	ctx := context.Background()

	// Initialize LM
	lm, err := dsgo.NewLM(ctx, ScanModel)
	if err != nil {
		log.Fatalf("Failed to initialize LM with model %s: %v", ScanModel, err)
	}

	// Find project root and parse module path
	root, err := tools.FindProjectRoot()
	if err != nil {
		log.Fatalf("Failed to find project root: %v", err)
	}

	goModPath := filepath.Join(root, "go.mod")
	modulePath := parseModulePath(goModPath)

	// Discover Go files and group into packages
	files, err := discoverGoFiles(root)
	if err != nil {
		log.Fatalf("Failed to discover Go files: %v", err)
	}

	// Apply file limit if specified
	if maxFiles := getMaxFilesFromEnv(); maxFiles > 0 && len(files) > maxFiles {
		log.Printf("Limiting to first %d files (out of %d total)", maxFiles, len(files))
		files = files[:maxFiles]
	}

	pkgs := groupFilesByPackage(files)
	log.Printf("Discovered %d Go files in %d packages", len(files), len(pkgs))

	// Stage 1: File-level security scans with retry logic

	fileSig := dsgo.NewSignature("Perform security vulnerability scan on a single Go source file").
		AddInput("file_path", dsgo.FieldTypeString, "Relative path of the Go file").
		AddInput("package_path", dsgo.FieldTypeString, "Relative package path (directory relative to project root)").
		AddInput("package_name", dsgo.FieldTypeString, "Go package name").
		AddInput("file_contents", dsgo.FieldTypeString, "Go source file contents (may be truncated for very large files)").
		AddOutput("summary", dsgo.FieldTypeString, "Brief summary of the file's purpose and security context").
		AddOutput("vulnerabilities", dsgo.FieldTypeString, "Specific security vulnerabilities found (e.g., SQL injection, XSS, path traversal, insecure crypto)").
		AddOutput("security_risks", dsgo.FieldTypeString, "Security risks and anti-patterns (e.g., hardcoded secrets, improper input validation, unsafe deserialization)").
		AddOutput("recommendations", dsgo.FieldTypeString, "Specific security recommendations and remediation steps").
		AddClassOutput("severity", []string{"LOW", "MEDIUM", "HIGH", "CRITICAL"}, "Overall security severity level")

	fileModule := dsgo.NewChainOfThought(fileSig, lm).
		WithOptions(&dsgo.GenerateOptions{
			Temperature: 0.1, // Lower temperature for security analysis
			MaxTokens:   1024 * 64,
		})

	fileParallel := dsgo.NewParallel(fileModule).
		WithVerbose(true).
		WithMaxWorkers(runtime.NumCPU()).
		WithReturnAll(true).
		WithMaxFailures(-1) // allow partial failures; let retry loop handle them
	log.Println("Stage 1: Running file-level security scans...")

	var fileScans []FileSecurityScan
	fileScanMap := make(map[int]FileSecurityScan) // index -> scan
	remainingFiles := make([]int, len(files))
	for i := range files {
		remainingFiles[i] = i
	}

	for attempt := 0; attempt < MaxRetries && len(remainingFiles) > 0; attempt++ {
		if attempt > 0 {
			log.Printf("Retry attempt %d for %d failed file security scans...", attempt+1, len(remainingFiles))
		}

		// Build inputs for remaining files only
		retryFiles := make([]GoFile, len(remainingFiles))
		retryInputs := make(map[string]any)
		paths := make([]string, len(remainingFiles))
		pkgPaths := make([]string, len(remainingFiles))
		pkgNames := make([]string, len(remainingFiles))
		contents := make([]string, len(remainingFiles))

		for i, fileIdx := range remainingFiles {
			f := files[fileIdx]
			retryFiles[i] = f
			full := filepath.Join(root, filepath.FromSlash(f.RelativePath))
			data, err := os.ReadFile(full)
			if err != nil {
				log.Printf("Failed to read file %s: %v", f.RelativePath, err)
				continue
			}
			if len(data) > MaxFileBytes {
				data = append(data[:MaxFileBytes], []byte("\n... [truncated]\n")...)
			}
			paths[i] = f.RelativePath
			pkgPaths[i] = f.PackageDir
			pkgNames[i] = f.PackageName
			contents[i] = string(data)
		}

		retryInputs = map[string]any{
			"file_path":     paths,
			"package_path":  pkgPaths,
			"package_name":  pkgNames,
			"file_contents": contents,
		}

		fileResult, err := fileParallel.Forward(ctx, retryInputs)
		if err != nil {
			log.Printf("Failed to run file-level security scans (attempt %d): %v", attempt+1, err)
			continue
		}

		// Collect successful scans and identify remaining failures
		attemptScans, failedIndices := collectFileSecurityScans(retryFiles, fileResult)
		for i, scan := range attemptScans {
			originalIdx := remainingFiles[i]
			fileScanMap[originalIdx] = scan
		}

		// Update remaining files list
		newRemaining := []int{}
		for _, failedIdx := range failedIndices {
			newRemaining = append(newRemaining, remainingFiles[failedIdx])
		}
		remainingFiles = newRemaining

		log.Printf("Attempt %d: %d successful, %d remaining", attempt+1, len(attemptScans), len(remainingFiles))
		log.Printf("Usage: $%.6f, %d tokens", fileResult.Usage.Cost, fileResult.Usage.TotalTokens)
	}

	// Convert map to slice in original order
	fileScans = make([]FileSecurityScan, len(files))
	for i := range files {
		if scan, ok := fileScanMap[i]; ok {
			fileScans[i] = scan
		} else {
			// Create empty scan for failed files
			fileScans[i] = FileSecurityScan{
				File:            files[i],
				Summary:         "FAILED TO SCAN",
				Vulnerabilities: "",
				SecurityRisks:   "Security scan failed after multiple attempts",
				Recommendations: "",
				Severity:        "UNKNOWN",
			}
		}
	}

	if len(remainingFiles) > 0 {
		log.Printf("Warning: %d file security scans still failed after %d attempts", len(remainingFiles), MaxRetries)
	}

	// Stage 2: Package-level security scans with retry logic

	pkgSig := dsgo.NewSignature("Perform security vulnerability analysis on a Go package from per-file security scans").
		AddInput("package_path", dsgo.FieldTypeString, "Relative path of the Go package directory").
		AddInput("package_name", dsgo.FieldTypeString, "Go package name").
		AddInput("file_scans", dsgo.FieldTypeString, "Concatenated per-file security scans for this package").
		AddOutput("package_summary", dsgo.FieldTypeString, "High-level summary of the package's security posture and attack surface").
		AddOutput("vulnerabilities", dsgo.FieldTypeString, "Aggregated vulnerabilities across the package with cross-file security issues").
		AddOutput("security_risks", dsgo.FieldTypeString, "Package-level security risks and architectural concerns").
		AddOutput("recommendations", dsgo.FieldTypeString, "Prioritized security recommendations for the package").
		AddClassOutput("overall_severity", []string{"LOW", "MEDIUM", "HIGH", "CRITICAL"}, "Overall security severity level for the package")

	pkgModule := dsgo.NewChainOfThought(pkgSig, lm).
		WithOptions(&dsgo.GenerateOptions{
			Temperature: 0.15,
			MaxTokens:   1024 * 64,
		})

	pkgParallel := dsgo.NewParallel(pkgModule).
		WithMaxWorkers(4).
		WithReturnAll(true).
		WithMaxFailures(-1) // allow partial failures; let retry loop handle them

	pkgText := groupFileSecurityScansByPackage(pkgs, files, fileScans)
	log.Println("Stage 2: Running package-level security scans...")

	var pkgScans []PackageSecurityScan
	pkgScanMap := make(map[int]PackageSecurityScan) // index -> scan
	remainingPkgs := make([]int, len(pkgs))
	for i := range pkgs {
		remainingPkgs[i] = i
	}

	for attempt := 0; attempt < MaxRetries && len(remainingPkgs) > 0; attempt++ {
		if attempt > 0 {
			log.Printf("Retry attempt %d for %d failed package security scans...", attempt+1, len(remainingPkgs))
		}

		// Build inputs for remaining packages only
		retryPkgs := make([]PackageInfo, len(remainingPkgs))
		retryInputs := make(map[string]any)
		paths := make([]string, len(remainingPkgs))
		names := make([]string, len(remainingPkgs))
		scans := make([]string, len(remainingPkgs))

		for i, pkgIdx := range remainingPkgs {
			pkg := pkgs[pkgIdx]
			retryPkgs[i] = pkg
			paths[i] = pkg.Path
			names[i] = pkg.Name
			scans[i] = pkgText[pkg.Path]
		}

		retryInputs = map[string]any{
			"package_path": paths,
			"package_name": names,
			"file_scans":   scans,
		}

		pkgResult, err := pkgParallel.Forward(ctx, retryInputs)
		if err != nil {
			log.Printf("Failed to run package-level security scans (attempt %d): %v", attempt+1, err)
			continue
		}

		// Collect successful scans and identify remaining failures
		attemptScans, failedIndices := collectPackageSecurityScans(retryPkgs, pkgResult)
		for i, scan := range attemptScans {
			originalIdx := remainingPkgs[i]
			pkgScanMap[originalIdx] = scan
		}

		// Update remaining packages list
		newRemaining := []int{}
		for _, failedIdx := range failedIndices {
			newRemaining = append(newRemaining, remainingPkgs[failedIdx])
		}
		remainingPkgs = newRemaining

		log.Printf("Attempt %d: %d successful, %d remaining", attempt+1, len(attemptScans), len(remainingPkgs))
		log.Printf("Usage: $%.6f, %d tokens", pkgResult.Usage.Cost, pkgResult.Usage.TotalTokens)
	}

	// Convert map to slice in original order
	pkgScans = make([]PackageSecurityScan, len(pkgs))
	for i := range pkgs {
		if scan, ok := pkgScanMap[i]; ok {
			pkgScans[i] = scan
		} else {
			// Create empty scan for failed packages
			pkgScans[i] = PackageSecurityScan{
				Package:         pkgs[i],
				Summary:         "FAILED TO SCAN",
				Vulnerabilities: "",
				SecurityRisks:   "Security scan failed after multiple attempts",
				Recommendations: "",
				OverallSeverity: "UNKNOWN",
			}
		}
	}

	if len(remainingPkgs) > 0 {
		log.Printf("Warning: %d package security scans still failed after %d attempts", len(remainingPkgs), MaxRetries)
	}

	// Stage 3: Project-level security scan
	log.Println("Stage 3: Running project-level security scan...")

	projectSig := dsgo.NewSignature("Generate a comprehensive project-wide security vulnerability report").
		AddInput("project_root", dsgo.FieldTypeString, "Absolute project root path or project label").
		AddInput("module_path", dsgo.FieldTypeString, "Go module path from go.mod").
		AddInput("package_scans", dsgo.FieldTypeString, "Concatenated package-level security scans for the project").
		AddOutput("executive_summary", dsgo.FieldTypeString, "High-level summary of overall security posture and critical findings").
		AddOutput("critical_vulnerabilities", dsgo.FieldTypeString, "Most critical security vulnerabilities that require immediate attention").
		AddOutput("security_posture", dsgo.FieldTypeString, "Overall security assessment including attack surface analysis").
		AddOutput("immediate_actions", dsgo.FieldTypeString, "Immediate security actions and remediation priorities")

	projectModule := dsgo.NewChainOfThought(projectSig, lm).
		WithOptions(&dsgo.GenerateOptions{
			Temperature: 0.2,
			MaxTokens:   1024 * 64,
		})

	projectParallel := dsgo.NewParallel(projectModule).
		WithRepeat(3). // run three project-level syntheses in parallel
		WithReturnAll(true).
		WithMaxFailures(1)

	projectInput := buildProjectSecurityScanInput(root, modulePath, pkgScans)
	projectPred, err := projectParallel.Forward(ctx, map[string]any{
		"project_root":  root,
		"module_path":   modulePath,
		"package_scans": projectInput,
	})
	if err != nil {
		log.Fatalf("Failed to run project-level security scan: %v", err)
	}

	bestScan := pickBestProjectSecurityScan(projectPred)
	if bestScan == nil {
		log.Fatalf("No project security scan results available")
	}

	log.Printf("Completed project-level security scan: $%.6f, %d tokens", projectPred.Usage.Cost, projectPred.Usage.TotalTokens)

	// Print final security scan report
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("SECURITY VULNERABILITY REPORT")
	fmt.Println(strings.Repeat("=", 60))

	if execSummary, ok := bestScan["executive_summary"].(string); ok && execSummary != "" {
		fmt.Println("\n🔒 EXECUTIVE SUMMARY")
		fmt.Println(strings.Repeat("-", 30))
		fmt.Println(execSummary)
	}

	if criticalVulns, ok := bestScan["critical_vulnerabilities"].(string); ok && criticalVulns != "" {
		fmt.Println("\n🚨 CRITICAL VULNERABILITIES")
		fmt.Println(strings.Repeat("-", 30))
		fmt.Println(criticalVulns)
	}

	if securityPosture, ok := bestScan["security_posture"].(string); ok && securityPosture != "" {
		fmt.Println("\n🛡️  SECURITY POSTURE")
		fmt.Println(strings.Repeat("-", 30))
		fmt.Println(securityPosture)
	}

	if immediateActions, ok := bestScan["immediate_actions"].(string); ok && immediateActions != "" {
		fmt.Println("\n⚡ IMMEDIATE ACTIONS")
		fmt.Println(strings.Repeat("-", 30))
		fmt.Println(immediateActions)
	}

	fmt.Println("\n" + strings.Repeat("=", 60))
}
