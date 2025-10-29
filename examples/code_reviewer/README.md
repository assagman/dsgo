# AI Code Reviewer

Demonstrates building a **multi-stage code review pipeline** using the Program module.

## Features Demonstrated

- **Program Module**: Chain multiple analysis modules together
- **Multi-Stage Analysis**: Structure → Issues → Recommendations
- **ChainOfThought**: Deep reasoning about code quality
- **Comprehensive Reviews**: Security, performance, best practices

## Use Cases

1. **Code Analysis Pipeline**: Automated multi-stage review process
2. **Comprehensive Security Review**: Detect SQL injection, XSS, etc.
3. **Quality Assessment**: Maintainability scoring and metrics

## Running the Example

```bash
export OPENAI_API_KEY=your_key_here
cd examples/code_reviewer
go run main.go
```

## What You'll Learn

- How to build analysis pipelines with Program
- How to structure multi-stage code reviews
- How outputs from one stage inform the next
- How to use ChainOfThought for deep analysis

## Review Stages

### Example 1: Three-Stage Pipeline
1. **Structure Analysis**: Complexity, maintainability score
2. **Issue Detection**: Find bugs, code smells, violations
3. **Recommendations**: Prioritized action items

### Example 2: Comprehensive Review
Single-stage deep analysis covering:
- Security issues
- Performance concerns
- Best practice violations
- Code smells
- Overall quality score

## Key Code Patterns

```go
// Multi-stage pipeline
pipeline := dsgo.NewProgram("Code Review").
    AddModule(structureModule).    // Analyze structure
    AddModule(issuesModule).        // Find issues
    AddModule(recommendModule)      // Generate recommendations

// Each stage builds on previous outputs
outputs, err := pipeline.Forward(ctx, inputs)
```

## Example Output

The reviewer detects:
- SQL injection vulnerabilities
- Unhashed passwords
- Missing input validation
- Performance inefficiencies
- Best practice violations

And provides prioritized, actionable recommendations.
