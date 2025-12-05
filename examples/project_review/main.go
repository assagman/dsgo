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
	// ReviewModel is the model used for all review modules
	ReviewModel = "openrouter/google/gemini-2.5-flash-lite-preview-09-2025"

	// MaxFileBytes limits file content to prevent excessive token usage
	MaxFileBytes = 128 * 1024

	// MaxRetries is the maximum number of retry attempts for failed reviews
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

// FileReview contains the review result for a single Go file
type FileReview struct {
	File        GoFile
	Summary     string
	Strengths   string
	Risks       string
	Suggestions string
}

// PackageReview contains the review result for a Go package
type PackageReview struct {
	Package         PackageInfo
	Summary         string
	Strengths       string
	Risks           string
	Recommendations string
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
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module "))
		}
	}
	return "unknown-module"
}

// buildFileReviewInputs prepares inputs for file-level parallel review
func buildFileReviewInputs(root string, files []GoFile) (map[string]any, error) {
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

// collectFileReviews extracts file reviews from prediction results
// Returns successful reviews and indices of failed files for retry
func collectFileReviews(files []GoFile, pred *dsgo.Prediction) ([]FileReview, []int) {
	reviews := make([]FileReview, 0, len(files))
	failedIndices := []int{}

	if pred.Completions == nil {
		// All files failed
		for i := range files {
			failedIndices = append(failedIndices, i)
		}
		return reviews, failedIndices
	}

	for i, f := range files {
		if i >= len(pred.Completions) {
			failedIndices = append(failedIndices, i)
			continue
		}
		c := pred.Completions[i]
		r := FileReview{File: f}

		// Check if we got valid outputs
		hasValidOutput := false
		if v, ok := c["summary"].(string); ok && v != "" {
			r.Summary = v
			hasValidOutput = true
		}
		if v, ok := c["strengths"].(string); ok && v != "" {
			r.Strengths = v
			hasValidOutput = true
		}
		if v, ok := c["risks"].(string); ok && v != "" {
			r.Risks = v
			hasValidOutput = true
		}
		if v, ok := c["suggestions"].(string); ok && v != "" {
			r.Suggestions = v
			hasValidOutput = true
		}

		if hasValidOutput {
			reviews = append(reviews, r)
		} else {
			failedIndices = append(failedIndices, i)
		}
	}
	return reviews, failedIndices
}

// groupFileReviewsByPackage aggregates file reviews by package
func groupFileReviewsByPackage(pkgs []PackageInfo, files []GoFile, fileReviews []FileReview) map[string]string {
	byPkg := make(map[string][]FileReview)

	// index FileReview by file path for quick lookup
	byPath := make(map[string]FileReview, len(fileReviews))
	for _, fr := range fileReviews {
		byPath[fr.File.RelativePath] = fr
	}

	for _, pkg := range pkgs {
		for _, idx := range pkg.FileIndices {
			f := files[idx]
			if fr, ok := byPath[f.RelativePath]; ok {
				byPkg[pkg.Path] = append(byPkg[pkg.Path], fr)
			}
		}
	}

	result := make(map[string]string, len(byPkg))
	for pkgPath, list := range byPkg {
		var b strings.Builder
		fmt.Fprintf(&b, "PACKAGE %s\n", pkgPath)
		for _, fr := range list {
			fmt.Fprintf(&b, "\n### File: %s\n", fr.File.RelativePath)
			fmt.Fprintf(&b, "Summary:\n%s\n\n", fr.Summary)
			fmt.Fprintf(&b, "Strengths:\n%s\n\n", fr.Strengths)
			fmt.Fprintf(&b, "Risks:\n%s\n\n", fr.Risks)
			fmt.Fprintf(&b, "Suggestions:\n%s\n\n", fr.Suggestions)
			b.WriteString("---\n")
		}
		result[pkgPath] = b.String()
	}
	return result
}

// buildPackageReviewInputs prepares inputs for package-level parallel review
func buildPackageReviewInputs(pkgs []PackageInfo, fileReviewText map[string]string) map[string]any {
	n := len(pkgs)
	paths := make([]string, n)
	names := make([]string, n)
	reviews := make([]string, n)

	for i, p := range pkgs {
		paths[i] = p.Path
		names[i] = p.Name
		reviews[i] = fileReviewText[p.Path]
	}
	return map[string]any{
		"package_path": paths,
		"package_name": names,
		"file_reviews": reviews,
	}
}

// collectPackageReviews extracts package reviews from prediction results
// Returns successful reviews and indices of failed packages for retry
func collectPackageReviews(pkgs []PackageInfo, pred *dsgo.Prediction) ([]PackageReview, []int) {
	reviews := make([]PackageReview, 0, len(pkgs))
	failedIndices := []int{}

	if pred.Completions == nil {
		// All packages failed
		for i := range pkgs {
			failedIndices = append(failedIndices, i)
		}
		return reviews, failedIndices
	}

	for i, pkg := range pkgs {
		if i >= len(pred.Completions) {
			failedIndices = append(failedIndices, i)
			continue
		}
		c := pred.Completions[i]
		pr := PackageReview{Package: pkg}

		// Check if we got valid outputs
		hasValidOutput := false
		if v, ok := c["package_summary"].(string); ok && v != "" {
			pr.Summary = v
			hasValidOutput = true
		}
		if v, ok := c["package_strengths"].(string); ok && v != "" {
			pr.Strengths = v
			hasValidOutput = true
		}
		if v, ok := c["package_risks"].(string); ok && v != "" {
			pr.Risks = v
			hasValidOutput = true
		}
		if v, ok := c["package_recommendations"].(string); ok && v != "" {
			pr.Recommendations = v
			hasValidOutput = true
		}

		if hasValidOutput {
			reviews = append(reviews, pr)
		} else {
			failedIndices = append(failedIndices, i)
		}
	}
	return reviews, failedIndices
}

// buildProjectReviewInput creates the combined input for project-level review
func buildProjectReviewInput(root, modulePath string, pkgReviews []PackageReview) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Project root: %s\n", root)
	fmt.Fprintf(&b, "Module path: %s\n\n", modulePath)
	for _, pr := range pkgReviews {
		fmt.Fprintf(&b, "=== Package %s (name: %s) ===\n", pr.Package.Path, pr.Package.Name)
		fmt.Fprintf(&b, "Summary:\n%s\n\n", pr.Summary)
		fmt.Fprintf(&b, "Strengths:\n%s\n\n", pr.Strengths)
		fmt.Fprintf(&b, "Risks:\n%s\n\n", pr.Risks)
		fmt.Fprintf(&b, "Recommendations:\n%s\n\n", pr.Recommendations)
		b.WriteString("========================================\n\n")
	}
	return b.String()
}

// pickBestProjectReview selects the best completion from multiple candidates
func pickBestProjectReview(pred *dsgo.Prediction) map[string]any {
	if pred.Completions == nil || len(pred.Completions) == 0 {
		return nil
	}
	bestIdx := 0
	bestScore := -1
	for i, c := range pred.Completions {
		es, _ := c["executive_summary"].(string)
		risks, _ := c["project_risks"].(string)
		recs, _ := c["top_recommendations"].(string)
		score := len(es) + len(risks) + len(recs)
		if score > bestScore {
			bestScore = score
			bestIdx = i
		}
	}
	return pred.Completions[bestIdx]
}

// getMaxFilesFromEnv returns the maximum number of files to process, or 0 for unlimited
func getMaxFilesFromEnv() int {
	if val := os.Getenv("PROJECT_REVIEW_MAX_FILES"); val != "" {
		if n, err := strconv.Atoi(val); err == nil && n > 0 {
			return n
		}
	}
	return 0 // unlimited
}

func main() {
	ctx := context.Background()

	// Initialize LM
	lm, err := dsgo.NewLM(ctx, ReviewModel)
	if err != nil {
		log.Fatalf("Failed to initialize LM with model %s: %v", ReviewModel, err)
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

	// Stage 1: File-level reviews with retry logic

	fileSig := dsgo.NewSignature("Review a single Go source file").
		AddInput("file_path", dsgo.FieldTypeString, "Relative path of the Go file").
		AddInput("package_path", dsgo.FieldTypeString, "Relative package path (directory relative to project root)").
		AddInput("package_name", dsgo.FieldTypeString, "Go package name").
		AddInput("file_contents", dsgo.FieldTypeString, "Go source file contents (may be truncated for very large files)").
		AddOutput("summary", dsgo.FieldTypeString, "Short summary of what this file does and its role").
		AddOutput("strengths", dsgo.FieldTypeString, "Notable strengths and good practices").
		AddOutput("risks", dsgo.FieldTypeString, "Potential issues, smells, or risks").
		AddOutput("suggestions", dsgo.FieldTypeString, "Concrete suggestions for improvement")

	fileModule := dsgo.NewChainOfThought(fileSig, lm).
		WithOptions(&dsgo.GenerateOptions{
			Temperature: 0.2,
			MaxTokens:   1024 * 64,
		})

	fileParallel := dsgo.NewParallel(fileModule).
		WithVerbose(true).
		WithMaxWorkers(runtime.NumCPU()).
		WithReturnAll(true).
		WithMaxFailures(-1) // allow partial failures; let retry loop handle them
	log.Println("Stage 1: Running file-level reviews...")

	var fileReviews []FileReview
	fileReviewMap := make(map[int]FileReview) // index -> review
	remainingFiles := make([]int, len(files))
	for i := range files {
		remainingFiles[i] = i
	}

	for attempt := 0; attempt < MaxRetries && len(remainingFiles) > 0; attempt++ {
		if attempt > 0 {
			log.Printf("Retry attempt %d for %d failed file reviews...", attempt+1, len(remainingFiles))
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
			log.Printf("Failed to run file-level reviews (attempt %d): %v", attempt+1, err)
			continue
		}

		// Collect successful reviews and identify remaining failures
		attemptReviews, failedIndices := collectFileReviews(retryFiles, fileResult)
		for i, review := range attemptReviews {
			originalIdx := remainingFiles[i]
			fileReviewMap[originalIdx] = review
		}

		// Update remaining files list
		newRemaining := []int{}
		for _, failedIdx := range failedIndices {
			newRemaining = append(newRemaining, remainingFiles[failedIdx])
		}
		remainingFiles = newRemaining

		log.Printf("Attempt %d: %d successful, %d remaining", attempt+1, len(attemptReviews), len(remainingFiles))
		log.Printf("Usage: %s, %d tokens", fileResult.Usage.Cost, fileResult.Usage.TotalTokens)
	}

	// Convert map to slice in original order
	fileReviews = make([]FileReview, len(files))
	for i := range files {
		if review, ok := fileReviewMap[i]; ok {
			fileReviews[i] = review
		} else {
			// Create empty review for failed files
			fileReviews[i] = FileReview{
				File:        files[i],
				Summary:     "FAILED TO REVIEW",
				Strengths:   "",
				Risks:       "Review failed after multiple attempts",
				Suggestions: "",
			}
		}
	}

	if len(remainingFiles) > 0 {
		log.Printf("Warning: %d file reviews still failed after %d attempts", len(remainingFiles), MaxRetries)
	}

	// Stage 2: Package-level reviews with retry logic

	pkgSig := dsgo.NewSignature("Summarize a Go package from per-file reviews").
		AddInput("package_path", dsgo.FieldTypeString, "Relative path of the Go package directory").
		AddInput("package_name", dsgo.FieldTypeString, "Go package name").
		AddInput("file_reviews", dsgo.FieldTypeString, "Concatenated per-file reviews for this package").
		AddOutput("package_summary", dsgo.FieldTypeString, "High-level summary of what the package does and its role").
		AddOutput("package_strengths", dsgo.FieldTypeString, "Key strengths across the package").
		AddOutput("package_risks", dsgo.FieldTypeString, "Key risks or design concerns").
		AddOutput("package_recommendations", dsgo.FieldTypeString, "Concrete, prioritized improvements for this package")

	pkgModule := dsgo.NewChainOfThought(pkgSig, lm).
		WithOptions(&dsgo.GenerateOptions{
			Temperature: 0.25,
			MaxTokens:   1024 * 64,
		})

	pkgParallel := dsgo.NewParallel(pkgModule).
		WithVerbose(true).
		WithMaxWorkers(4).
		WithReturnAll(true).
		WithMaxFailures(-1) // allow partial failures; let retry loop handle them

	pkgText := groupFileReviewsByPackage(pkgs, files, fileReviews)
	log.Println("Stage 2: Running package-level reviews...")

	var pkgReviews []PackageReview
	pkgReviewMap := make(map[int]PackageReview) // index -> review
	remainingPkgs := make([]int, len(pkgs))
	for i := range pkgs {
		remainingPkgs[i] = i
	}

	for attempt := 0; attempt < MaxRetries && len(remainingPkgs) > 0; attempt++ {
		if attempt > 0 {
			log.Printf("Retry attempt %d for %d failed package reviews...", attempt+1, len(remainingPkgs))
		}

		// Build inputs for remaining packages only
		retryPkgs := make([]PackageInfo, len(remainingPkgs))
		retryInputs := make(map[string]any)
		paths := make([]string, len(remainingPkgs))
		names := make([]string, len(remainingPkgs))
		reviews := make([]string, len(remainingPkgs))

		for i, pkgIdx := range remainingPkgs {
			pkg := pkgs[pkgIdx]
			retryPkgs[i] = pkg
			paths[i] = pkg.Path
			names[i] = pkg.Name
			reviews[i] = pkgText[pkg.Path]
		}

		retryInputs = map[string]any{
			"package_path": paths,
			"package_name": names,
			"file_reviews": reviews,
		}

		pkgResult, err := pkgParallel.Forward(ctx, retryInputs)
		if err != nil {
			log.Printf("Failed to run package-level reviews (attempt %d): %v", attempt+1, err)
			continue
		}

		// Collect successful reviews and identify remaining failures
		attemptReviews, failedIndices := collectPackageReviews(retryPkgs, pkgResult)
		for i, review := range attemptReviews {
			originalIdx := remainingPkgs[i]
			pkgReviewMap[originalIdx] = review
		}

		// Update remaining packages list
		newRemaining := []int{}
		for _, failedIdx := range failedIndices {
			newRemaining = append(newRemaining, remainingPkgs[failedIdx])
		}
		remainingPkgs = newRemaining

		log.Printf("Attempt %d: %d successful, %d remaining", attempt+1, len(attemptReviews), len(remainingPkgs))
		log.Printf("Usage: %s, %d tokens", pkgResult.Usage.Cost, pkgResult.Usage.TotalTokens)
	}

	// Convert map to slice in original order
	pkgReviews = make([]PackageReview, len(pkgs))
	for i := range pkgs {
		if review, ok := pkgReviewMap[i]; ok {
			pkgReviews[i] = review
		} else {
			// Create empty review for failed packages
			pkgReviews[i] = PackageReview{
				Package:         pkgs[i],
				Summary:         "FAILED TO REVIEW",
				Strengths:       "",
				Risks:           "Review failed after multiple attempts",
				Recommendations: "",
			}
		}
	}

	if len(remainingPkgs) > 0 {
		log.Printf("Warning: %d package reviews still failed after %d attempts", len(remainingPkgs), MaxRetries)
	}

	// Stage 3: Project-level review
	log.Println("Stage 3: Running project-level review...")

	projectSig := dsgo.NewSignature("Generate a project-wide Go codebase review").
		AddInput("project_root", dsgo.FieldTypeString, "Absolute project root path or project label").
		AddInput("module_path", dsgo.FieldTypeString, "Go module path from go.mod").
		AddInput("package_reviews", dsgo.FieldTypeString, "Concatenated package-level reviews for the project").
		AddOutput("executive_summary", dsgo.FieldTypeString, "High-level summary of overall architecture and quality").
		AddOutput("architecture_overview", dsgo.FieldTypeString, "Description of main layers, key packages, and their relationships").
		AddOutput("project_strengths", dsgo.FieldTypeString, "Key strengths across the project").
		AddOutput("project_risks", dsgo.FieldTypeString, "Key risks, bottlenecks, or fragilities").
		AddOutput("top_recommendations", dsgo.FieldTypeString, "Top prioritized recommendations for improving the codebase")

	projectModule := dsgo.NewChainOfThought(projectSig, lm).
		WithOptions(&dsgo.GenerateOptions{
			Temperature: 0.3,
			MaxTokens:   1024 * 64,
		})

	projectParallel := dsgo.NewParallel(projectModule).
		WithVerbose(true).
		WithRepeat(3). // run three project-level syntheses in parallel
		WithReturnAll(true).
		WithMaxFailures(1)

	projectInput := buildProjectReviewInput(root, modulePath, pkgReviews)
	projectPred, err := projectParallel.Forward(ctx, map[string]any{
		"project_root":    root,
		"module_path":     modulePath,
		"package_reviews": projectInput,
	})
	if err != nil {
		log.Fatalf("Failed to run project-level review: %v", err)
	}

	bestReview := pickBestProjectReview(projectPred)
	if bestReview == nil {
		log.Fatalf("No project review results available")
	}

	log.Printf("Completed project-level review: %s, %d tokens", projectPred.Usage.Cost, projectPred.Usage.TotalTokens)

	// Print final project review
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("PROJECT REVIEW")
	fmt.Println(strings.Repeat("=", 60))

	if execSummary, ok := bestReview["executive_summary"].(string); ok && execSummary != "" {
		fmt.Println("\n📋 EXECUTIVE SUMMARY")
		fmt.Println(strings.Repeat("-", 30))
		fmt.Println(execSummary)
	}

	if archOverview, ok := bestReview["architecture_overview"].(string); ok && archOverview != "" {
		fmt.Println("\n🏗️  ARCHITECTURE OVERVIEW")
		fmt.Println(strings.Repeat("-", 30))
		fmt.Println(archOverview)
	}

	if strengths, ok := bestReview["project_strengths"].(string); ok && strengths != "" {
		fmt.Println("\n💪 PROJECT STRENGTHS")
		fmt.Println(strings.Repeat("-", 30))
		fmt.Println(strengths)
	}

	if risks, ok := bestReview["project_risks"].(string); ok && risks != "" {
		fmt.Println("\n⚠️  PROJECT RISKS")
		fmt.Println(strings.Repeat("-", 30))
		fmt.Println(risks)
	}

	if recommendations, ok := bestReview["top_recommendations"].(string); ok && recommendations != "" {
		fmt.Println("\n🎯 TOP RECOMMENDATIONS")
		fmt.Println(strings.Repeat("-", 30))
		fmt.Println(recommendations)
	}

	fmt.Println("\n" + strings.Repeat("=", 60))
}
