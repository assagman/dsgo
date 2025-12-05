# Security Vulnerability Scanner

This example demonstrates a comprehensive security vulnerability scanning system for Go codebases using DSGo. It performs deep security analysis across three levels: individual files, packages, and the entire project.

## Features

- **Multi-level Security Analysis**: File → Package → Project hierarchy
- **Vulnerability Detection**: Identifies common security issues (SQL injection, XSS, path traversal, etc.)
- **Security Risk Assessment**: Evaluates security anti-patterns and architectural concerns
- **Severity Classification**: Categorizes findings as LOW, MEDIUM, HIGH, or CRITICAL
- **Retry Logic**: Automatically retries failed scans up to 3 times
- **Parallel Processing**: Efficient concurrent scanning of multiple files and packages
- **Comprehensive Reporting**: Generates detailed security vulnerability reports

## How It Works

### Stage 1: File-level Security Scanning
- Scans each Go source file individually
- Identifies file-specific vulnerabilities and security risks
- Classifies severity level for each file
- Provides specific security recommendations

### Stage 2: Package-level Security Analysis
- Aggregates findings from individual files
- Identifies cross-file security issues
- Evaluates package-level security posture
- Provides prioritized recommendations

### Stage 3: Project-level Security Assessment
- Synthesizes all package-level findings
- Identifies critical vulnerabilities requiring immediate attention
- Provides overall security posture assessment
- Generates immediate action plan

## Usage

```bash
# Run from project root
cd examples/security_scan
go run main.go

# Limit number of files processed (useful for large projects)
export SECURITY_SCAN_MAX_FILES=50
go run main.go

# Use custom model
export SCAN_MODEL="gpt-4o"
go run main.go
```

## Security Categories Analyzed

### Common Vulnerabilities
- SQL Injection
- Cross-Site Scripting (XSS)
- Path Traversal
- Command Injection
- Insecure Deserialization
- Buffer Overflows
- Race Conditions

### Security Anti-patterns
- Hardcoded secrets/credentials
- Improper input validation
- Weak cryptographic usage
- Insecure random number generation
- Unsafe file operations
- Insufficient error handling

### Architectural Concerns
- Authentication and authorization flaws
- Insecure communication protocols
- Inadequate logging and monitoring
- Improper dependency management
- Insecure configuration management

## Output

The scanner generates a comprehensive security report with:

- **Executive Summary**: High-level security posture overview
- **Critical Vulnerabilities**: Most severe issues requiring immediate attention
- **Security Posture**: Overall security assessment and attack surface analysis
- **Immediate Actions**: Prioritized remediation steps

## Configuration

### Environment Variables
- `SECURITY_SCAN_MAX_FILES`: Limit number of files to scan (default: unlimited)
- `SCAN_MODEL`: Custom model to use for security analysis

### Retry Configuration
- `MaxRetries`: Maximum retry attempts (default: 3)
- Automatic retry for failed scans with exponential backoff

## Example Output

```
============================================================
SECURITY VULNERABILITY REPORT
============================================================

🔒 EXECUTIVE SUMMARY
------------------------------
The codebase shows moderate security posture with several critical vulnerabilities requiring immediate attention...

🚨 CRITICAL VULNERABILITIES
------------------------------
1. Hardcoded API keys in internal/config/config.go
2. SQL injection vulnerability in internal/database/query.go
3. Path traversal in internal/handlers/file_handler.go

🛡️  SECURITY POSTURE
------------------------------
Overall security risk level: HIGH
Attack surface: Web APIs, database access, file operations...

⚡ IMMEDIATE ACTIONS
------------------------------
1. Remove hardcoded credentials and use environment variables
2. Implement parameterized queries for all database operations
3. Add input validation and sanitization for file operations...
============================================================
```

## Architecture

The scanner follows DSGo's three-layer architecture:

1. **Core Layer**: Security-focused signatures and adapters
2. **Module Layer**: ChainOfThought modules for security analysis
3. **Provider Layer**: LM integration for intelligent security scanning

## Performance

- **Parallel Processing**: Scans multiple files concurrently
- **Retry Logic**: Ensures comprehensive coverage
- **Token Optimization**: Efficient content truncation for large files
- **Cost Tracking**: Monitors API usage and costs

## Best Practices

1. **Regular Scanning**: Run security scans regularly in CI/CD pipelines
2. **Immediate Remediation**: Address CRITICAL and HIGH severity issues promptly
3. **Dependency Updates**: Keep dependencies updated to patch known vulnerabilities
4. **Security Training**: Ensure team understands common security pitfalls
5. **Code Review**: Incorporate security-focused code reviews

## Integration

This scanner can be integrated into:
- CI/CD pipelines for automated security checks
- Pre-commit hooks for immediate feedback
- Security audit workflows
- Compliance monitoring systems

## Limitations

- Static analysis only (no runtime vulnerability detection)
- Dependent on LM model's security knowledge
- May produce false positives requiring human validation
- Large codebases may require file limits for cost management

## Contributing

When contributing to the security scanner:
- Focus on improving vulnerability detection accuracy
- Add new security categories as needed
- Optimize for performance and cost
- Update documentation with new security patterns
