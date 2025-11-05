# 028_code_reviewer - AI-Powered Multi-Stage Code Review

## Overview

Demonstrates building a **production-grade code review system** using DSGo's Program module for multi-stage pipeline orchestration. This example shows how to combine multiple analysis modules to perform comprehensive automated code reviews, detecting security vulnerabilities, performance issues, and code quality concerns.

## What it demonstrates

- **Program composition** - Multi-stage pipeline with dependent modules (structure → issues → recommendations)
- **Chain of Thought reasoning** - Deep analysis for complex recommendations
- **Multiple output types** - Strings, floats, JSON, and class/enum outputs
- **Pipeline data flow** - Each stage builds on previous outputs
- **Security analysis** - Detection of SQL injection, XSS, authentication flaws
- **Quality metrics** - Maintainability scores, complexity assessment, quality ratings
- Use cases: automated code review, security scanning, refactoring prioritization, quality gates

## Usage

```bash
cd examples/028_code_reviewer
go run main.go
```

### With Harness Flags

```bash
go run main.go -verbose -format=json
go run main.go -concurrency=1
```

### Environment Variables

```bash
export HARNESS_VERBOSE=true
export HARNESS_OUTPUT_FORMAT=json
go run main.go
```

## Expected Output

```
=== AI Code Reviewer Example ===
Demonstrates: Multi-Stage Code Review Pipeline with Program Composition

--- Example 1: Code Analysis Pipeline ---
Code to Review:

func processData(data []int) []int {
    result := []int{}
    for i := 0; i < len(data); i++ {
        if data[i] > 0 {
            result = append(result, data[i] * 2)
        }
    }
    return result
}

Structure Analysis: Simple function with single loop, no nested structures. 
Linear complexity, basic control flow.

Complexity: Low - O(n) time complexity with single iteration. Simple conditional 
logic. No recursion or nested loops.

Maintainability Score: 0.75

Issues Found:
• Pre-allocation: Result slice should be pre-allocated with cap(len(data))
• Range loop: Should use range instead of index-based iteration
• Magic number: The multiplier '2' should be a named constant
• Nil handling: No check for nil input slice

Severity: low

Recommendations:
[
  {
    "priority": 1,
    "issue": "Pre-allocate result slice",
    "solution": "Use result := make([]int, 0, len(data))",
    "impact": "Reduces memory allocations and improves performance"
  },
  {
    "priority": 2,
    "issue": "Use range loop",
    "solution": "Replace for i := 0; i < len(data); i++ with for _, v := range data",
    "impact": "More idiomatic Go, eliminates index errors"
  },
  {
    "priority": 3,
    "issue": "Extract magic number",
    "solution": "Define const multiplier = 2",
    "impact": "Improves maintainability and readability"
  }
]

Refactoring Priority: Start with pre-allocation for immediate performance gain, 
then convert to range loop for better idioms, finally extract constants.

--- Example 2: Comprehensive Code Review ---
Code Under Review:

function authenticateUser(username, password) {
    var query = "SELECT * FROM users WHERE username='" + username +
                "' AND password='" + password + "'";
    var result = db.execute(query);
    if (result.length > 0) {
        return result[0];
    }
    return null;
}

======================================================================
COMPREHENSIVE CODE REVIEW REPORT
======================================================================

🔒 SECURITY ISSUES:
CRITICAL - SQL Injection Vulnerability:
  • Directly concatenating user input (username, password) into SQL query
  • Allows attackers to inject malicious SQL: e.g., username = "admin' OR '1'='1"
  • Could expose entire user database or allow authentication bypass
  • MUST use parameterized queries or prepared statements

HIGH - Plain Text Password Comparison:
  • Password stored/compared in plain text
  • Should use secure hashing (bcrypt, scrypt, argon2)
  • Storing passwords in plain text is a major security violation

MEDIUM - Information Disclosure:
  • SELECT * returns all user data including potentially sensitive fields
  • Should only select necessary fields for authentication

⚡ PERFORMANCE ISSUES:
• Database query executed synchronously without connection pooling
• No indexing guidance (should index username column)
• Returning entire user object when only validation needed
• No query timeout or connection management

✅ BEST PRACTICES:
• Missing input validation (username/password length, format)
• No error handling for database connection failures
• Using 'var' instead of 'const/let' (outdated JavaScript)
• No logging for security events (failed auth attempts)
• Function should return boolean or token, not user object
• Missing JSDoc documentation
• No rate limiting or brute force protection

👃 CODE SMELLS:
• Magic strings for database column names
• Tight coupling to database layer (should use repository pattern)
• Mixed concerns (authentication + data retrieval)
• No separation of validation logic
• Returning null (prefer explicit error handling)

📊 OVERALL QUALITY: 0.15/1.0

📝 SUMMARY:
This authentication function has CRITICAL security vulnerabilities, particularly 
SQL injection and plain text password handling. It requires immediate refactoring 
before any production use. The function violates fundamental security principles 
and modern JavaScript best practices. Recommended approach: use parameterized 
queries, implement password hashing, add input validation, separate concerns, 
and implement proper error handling.

======================================================================
```

## Key Concepts

### 1. Multi-Stage Pipeline with Program

Build complex analysis workflows by chaining modules:

```go
import (
    "github.com/assagman/dsgo"
    "github.com/assagman/dsgo/module"
)

// Stage 1: Analyze structure
structureSig := dsgo.NewSignature("Analyze code structure and complexity").
    AddInput("code", dsgo.FieldTypeString, "The code to analyze").
    AddOutput("structure_analysis", dsgo.FieldTypeString, "Structure analysis").
    AddOutput("complexity", dsgo.FieldTypeString, "Complexity assessment").
    AddOutput("maintainability_score", dsgo.FieldTypeFloat, "Maintainability score 0-1")

structureModule := module.NewPredict(structureSig, lm)

// Stage 2: Identify issues (uses outputs from stage 1)
issuesSig := dsgo.NewSignature("Identify code issues and improvements").
    AddInput("code", dsgo.FieldTypeString, "The code").
    AddInput("structure_analysis", dsgo.FieldTypeString, "Structure analysis").
    AddInput("complexity", dsgo.FieldTypeString, "Complexity").
    AddOutput("issues", dsgo.FieldTypeString, "List of issues found").
    AddOutput("suggestions", dsgo.FieldTypeString, "Improvement suggestions").
    AddClassOutput("severity", []string{"low", "medium", "high", "critical"}, "Overall severity")

issuesModule := module.NewPredict(issuesSig, lm)

// Stage 3: Generate recommendations (uses outputs from stage 2)
recommendSig := dsgo.NewSignature("Generate actionable recommendations").
    AddInput("issues", dsgo.FieldTypeString, "Issues").
    AddInput("suggestions", dsgo.FieldTypeString, "Suggestions").
    AddInput("severity", dsgo.FieldTypeString, "Severity").
    AddOutput("recommendations", dsgo.FieldTypeJSON, "Prioritized recommendations as JSON array").
    AddOutput("refactoring_priority", dsgo.FieldTypeString, "What to refactor first")

recommendModule := module.NewChainOfThought(recommendSig, lm)

// Compose into pipeline
pipeline := module.NewProgram("Code Review Pipeline").
    AddModule(structureModule).
    AddModule(issuesModule).
    AddModule(recommendModule)

// Execute entire pipeline
result, err := pipeline.Forward(ctx, map[string]any{
    "code": codeToReview,
})
```

**How it works:**
1. **Stage 1** analyzes code and produces `structure_analysis`, `complexity`, `maintainability_score`
2. **Stage 2** receives `structure_analysis` and `complexity` automatically, produces `issues`, `suggestions`, `severity`
3. **Stage 3** receives `issues`, `suggestions`, `severity` automatically, produces final recommendations
4. Each stage builds on previous outputs - no manual plumbing required

**Benefits:**
- **Automatic data flow** - Outputs become inputs for next stage
- **Modular design** - Each stage is independent and testable
- **Progressive refinement** - Each stage adds more detail
- **Error isolation** - Failures isolated to specific stages

**When to use:**
- Multi-step analysis requiring progressive refinement
- Complex workflows with dependent stages
- Modular systems where stages can be reused
- When intermediate results are valuable

### 2. Comprehensive Code Review

Perform deep analysis with multiple dimensions:

```go
import (
    "github.com/assagman/dsgo"
    "github.com/assagman/dsgo/module"
)

// Multi-aspect review signature
reviewSig := dsgo.NewSignature("Perform comprehensive code review").
    AddInput("code", dsgo.FieldTypeString, "Code to review").
    AddInput("language", dsgo.FieldTypeString, "Programming language").
    AddOutput("security_issues", dsgo.FieldTypeString, "Security concerns").
    AddOutput("performance_issues", dsgo.FieldTypeString, "Performance concerns").
    AddOutput("best_practices", dsgo.FieldTypeString, "Best practice violations").
    AddOutput("code_smell", dsgo.FieldTypeString, "Code smells detected").
    AddOutput("overall_quality", dsgo.FieldTypeFloat, "Overall quality score 0-1").
    AddOutput("summary", dsgo.FieldTypeString, "Executive summary")

review := module.NewChainOfThought(reviewSig, lm)

result, err := review.Forward(ctx, map[string]any{
    "code":     codeToReview,
    "language": "JavaScript",
})

// Access multi-dimensional analysis
securityIssues := result.Outputs["security_issues"].(string)
performanceIssues := result.Outputs["performance_issues"].(string)
quality := result.Outputs["overall_quality"].(float64)
```

**Analysis dimensions:**
- **Security** - SQL injection, XSS, authentication flaws, data exposure
- **Performance** - Inefficiencies, N+1 queries, memory leaks
- **Best Practices** - Language idioms, error handling, documentation
- **Code Smells** - Coupling, complexity, duplication, magic values
- **Quality Score** - Numeric assessment (0-1 scale)
- **Summary** - Executive overview with priorities

**Benefits:**
- **Holistic view** - All dimensions analyzed together
- **Prioritization** - Clear severity levels and quality scores
- **Actionable** - Specific issues with concrete recommendations
- **Context-aware** - Language-specific analysis

**When to use:**
- Pre-commit quality gates
- Security audits
- Refactoring prioritization
- Code review automation
- Technical debt assessment

### 3. JSON Output for Structured Recommendations

Generate machine-readable prioritized action items:

```go
import "github.com/assagman/dsgo"

sig := dsgo.NewSignature("Generate actionable recommendations").
    AddInput("issues", dsgo.FieldTypeString, "Issues").
    AddInput("suggestions", dsgo.FieldTypeString, "Suggestions").
    AddInput("severity", dsgo.FieldTypeString, "Severity").
    AddOutput("recommendations", dsgo.FieldTypeJSON, "Prioritized recommendations as JSON array").
    AddOutput("refactoring_priority", dsgo.FieldTypeString, "What to refactor first")

// LM generates structured JSON output
result, _ := module.Forward(ctx, inputs)

// Output like:
// [
//   {
//     "priority": 1,
//     "issue": "SQL Injection",
//     "solution": "Use parameterized queries",
//     "impact": "Critical security fix"
//   },
//   {
//     "priority": 2,
//     "issue": "Password hashing",
//     "solution": "Implement bcrypt",
//     "impact": "Prevents credential theft"
//   }
// ]
```

**Benefits:**
- **Machine-readable** - Easy to parse and process
- **Structured data** - Consistent format across reviews
- **Prioritized** - Clear ordering of actions
- **Trackable** - Can integrate with issue trackers

**When to use:**
- CI/CD integration
- Automated issue creation
- Dashboard reporting
- Trend analysis

### 4. Severity Classification

Use constrained enum outputs for consistent categorization:

```go
import "github.com/assagman/dsgo"

sig := dsgo.NewSignature("Identify code issues").
    AddInput("code", dsgo.FieldTypeString, "Code to analyze").
    AddClassOutput("severity", 
        []string{"low", "medium", "high", "critical"}, 
        "Overall severity of issues found")

result, _ := module.Forward(ctx, inputs)

severity := result.Outputs["severity"].(string)
// Guaranteed to be one of: "low", "medium", "high", "critical"

switch severity {
case "critical":
    // Block merge
case "high":
    // Require immediate fix
case "medium":
    // Schedule for next sprint
case "low":
    // Add to backlog
}
```

**Severity levels:**
- **critical** - Security vulnerabilities, data loss risks, complete failures
- **high** - Major bugs, significant performance issues, wrong behavior
- **medium** - Minor bugs, code smells, non-critical best practice violations
- **low** - Style issues, minor inefficiencies, suggestions

**Benefits:**
- **Consistent** - Standardized severity levels
- **Validated** - Automatic checking of valid values
- **Actionable** - Clear priority for remediation
- **Enforceable** - Can set quality gates based on severity

## Common Patterns

### Pattern 1: Three-Stage Analysis Pipeline

Progressive refinement from structure to recommendations:

```go
// Stage 1: High-level structure
structureModule := module.NewPredict(structureSig, lm)

// Stage 2: Detailed issue detection
issuesModule := module.NewPredict(issuesSig, lm)

// Stage 3: CoT for complex reasoning
recommendModule := module.NewChainOfThought(recommendSig, lm)

// Pipeline automatically connects stages
pipeline := module.NewProgram("Code Review").
    AddModule(structureModule).
    AddModule(issuesModule).
    AddModule(recommendModule)
```

### Pattern 2: Comprehensive Single-Pass Review

Deep analysis with multiple output dimensions:

```go
reviewSig := dsgo.NewSignature("Comprehensive review").
    AddInput("code", dsgo.FieldTypeString, "Code").
    AddInput("language", dsgo.FieldTypeString, "Language").
    AddOutput("security_issues", dsgo.FieldTypeString, "Security").
    AddOutput("performance_issues", dsgo.FieldTypeString, "Performance").
    AddOutput("best_practices", dsgo.FieldTypeString, "Best practices").
    AddOutput("code_smell", dsgo.FieldTypeString, "Code smells").
    AddOutput("overall_quality", dsgo.FieldTypeFloat, "Quality score").
    AddOutput("summary", dsgo.FieldTypeString, "Summary")

review := module.NewChainOfThought(reviewSig, lm)
```

### Pattern 3: Quality Gate Integration

Block merges based on quality thresholds:

```go
result, _ := pipeline.Forward(ctx, inputs)

quality := result.Outputs["overall_quality"].(float64)
severity := result.Outputs["severity"].(string)

if severity == "critical" || quality < 0.5 {
    // Block merge
    return errors.New("Quality gate failed")
}

if severity == "high" || quality < 0.7 {
    // Require approval
    requestReview()
}

// Auto-approve
approveMerge()
```

### Pattern 4: Language-Specific Analysis

Tailor review to programming language:

```go
reviewSig := dsgo.NewSignature("Language-specific review").
    AddInput("code", dsgo.FieldTypeString, "Code").
    AddInput("language", dsgo.FieldTypeString, "Programming language")

// LM adapts analysis based on language
result, _ := review.Forward(ctx, map[string]any{
    "code":     code,
    "language": "Go",  // Go-specific: goroutine safety, defer usage, error handling
})
```

## Troubleshooting

### Pipeline Stage Fails

**Symptom:** Error at stage 2 or 3 of pipeline

**Diagnosis:**
```go
// Previous stage didn't produce required outputs
// Or outputs have wrong types
```

**Solution:**
```go
// Ensure stage signatures match
// Stage 1 outputs = Stage 2 inputs
structureSig.AddOutput("complexity", dsgo.FieldTypeString, "...")
issuesSig.AddInput("complexity", dsgo.FieldTypeString, "...")  // Must match

// Test stages independently first
result1, _ := structureModule.Forward(ctx, inputs)
fmt.Printf("Stage 1 outputs: %+v\n", result1.Outputs)
```

### JSON Output Invalid

**Symptom:** Can't parse `recommendations` JSON

**Diagnosis:**
```go
// LM returned invalid JSON or non-array
```

**Solution:**
```go
// Make prompt more explicit
sig.AddOutput("recommendations", dsgo.FieldTypeJSON, 
    "Prioritized recommendations as valid JSON array with priority, issue, solution fields")

// Or use JSONAdapter for better parsing
```

### Severity Not Recognized

**Symptom:** Error about invalid class value

**Diagnosis:**
```go
// LM returned value not in allowed list
// e.g., "severe" instead of "high"
```

**Solution:**
```go
// Use clear, distinct values
sig.AddClassOutput("severity", 
    []string{"low", "medium", "high", "critical"},  // Common, unambiguous terms
    "Overall severity - use low/medium/high/critical")

// Provide examples in signature instruction
```

### Quality Score Out of Range

**Symptom:** Score > 1.0 or negative

**Diagnosis:**
```go
// LM returned invalid quality score
```

**Solution:**
```go
// Add validation
quality := result.Outputs["overall_quality"].(float64)
if quality < 0 || quality > 1 {
    quality = math.Max(0, math.Min(1, quality))  // Clamp to [0, 1]
}

// Or add constraint in signature description
sig.AddOutput("overall_quality", dsgo.FieldTypeFloat, 
    "Overall quality score between 0.0 and 1.0 inclusive")
```

## Performance Considerations

### Pipeline Execution Time

**Typical timing:**
- 3-stage pipeline: 3 sequential API calls = 3-10 seconds total
- Stage 1: 1-3 seconds (structure analysis)
- Stage 2: 1-3 seconds (issue detection)
- Stage 3: 2-5 seconds (CoT recommendations)

**Optimization:**
- Use faster models for early stages (structure)
- Reserve powerful models for final stage (recommendations)
- Cache results for unchanged code

### Token Usage

**Typical usage per review:**
- Structure analysis: 500-1000 tokens
- Issue detection: 800-1500 tokens
- Recommendations (CoT): 1500-3000 tokens
- **Total pipeline: ~3000-5500 tokens**

**Cost estimate:**
- GPT-4: ~$0.015-0.028 per review
- GPT-3.5: ~$0.002-0.003 per review

**Optimization:**
- Use smaller context for repeated code sections
- Cache common patterns
- Batch similar reviews

### Concurrency

**Pipeline is sequential:**
- Stages must run in order (dependencies)
- Cannot parallelize within single review

**Can parallelize reviews:**
```go
// Review multiple files concurrently
for _, file := range files {
    go reviewFile(file)  // Each gets own pipeline instance
}
```

## Comparison with Alternatives

**vs. Static Analysis Tools (ESLint, golangci-lint):**
- **Code Reviewer**: Semantic understanding, context-aware, explains issues
- **Static Tools**: Faster, cheaper, rule-based, no context

**vs. Human Code Review:**
- **Code Reviewer**: Instant, consistent, catches common issues
- **Human**: Better context, architecture decisions, creativity

**vs. Single-Pass Analysis:**
- **Multi-Stage Pipeline**: More thorough, builds context progressively
- **Single-Pass**: Faster, simpler, good for basic checks

**Best Practice:** Combine all approaches
1. Static tools for syntax/style
2. AI reviewer for semantic issues
3. Human review for architecture/design

## See Also

- [007_program_composition](../007_program_composition/) - Module composition & pipelines
- [002_chain_of_thought](../002_chain_of_thought/) - CoT reasoning for complex analysis
- [001_predict](../001_predict/) - Basic prediction
- [013_sentiment](../013_sentiment/) - Another classification use case
- [027_research_assistant](../027_research_assistant/) - Complex multi-tool workflows
- [QUICKSTART.md](../../QUICKSTART.md) - Getting started guide

## Production Tips

1. **Validate Outputs**: Always validate JSON structure and score ranges
2. **Cache Results**: Cache reviews for unchanged code (hash-based)
3. **Rate Limits**: Implement rate limiting for API protection
4. **Error Handling**: Graceful degradation when stages fail
5. **Logging**: Log all reviews for analysis and improvement
6. **Quality Gates**: Enforce minimum scores in CI/CD
7. **Language Detection**: Auto-detect language from file extension
8. **Incremental Reviews**: Only review changed lines (diff-based)
9. **Severity Escalation**: Alert security team for critical issues
10. **Metrics**: Track quality trends over time

## Architecture Notes

Code review pipeline flow:

```
┌─────────────────────────────────────────────────────────┐
│                   Application Code                       │
│                  inputs: { code }                        │
└────────────────────────┬────────────────────────────────┘
                         │
                         ▼
              ┌──────────────────────┐
              │   Program Pipeline   │
              │  (3 sequential stages)│
              └──────────┬───────────┘
                         │
        ┌────────────────┼────────────────┐
        │                │                │
        ▼                ▼                ▼
┌──────────────┐  ┌──────────────┐  ┌─────────────┐
│   Stage 1    │  │   Stage 2    │  │   Stage 3   │
│  Structure   │─▶│    Issues    │─▶│Recommenda-  │
│   Analysis   │  │  Detection   │  │   tions     │
└──────────────┘  └──────────────┘  └─────────────┘
   Predict           Predict          ChainOfThought
      │                 │                   │
      ▼                 ▼                   ▼
 structure_        issues,              recommendations
 analysis,         suggestions,         (JSON),
 complexity,       severity             refactoring_
 maintainability                        priority
 
                         │
                         ▼
         ┌───────────────────────────────┐
         │    Validated Final Output     │
         │  All stages combined          │
         └───────────────────────────────┘
```

**Design Principles:**
- **Progressive refinement** - Each stage adds detail
- **Dependency chain** - Outputs flow to next inputs
- **Mixed module types** - Predict for fast, CoT for complex
- **Type safety** - All outputs validated
- **Composable** - Stages can be reused/reordered
- **Production-ready** - Error handling, metrics, logging
