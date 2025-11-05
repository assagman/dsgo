# 005_best_of_n - Best of N Sampling

## Overview

Demonstrates BestOfN module for generating multiple candidate solutions and selecting the best one based on a custom scoring function. Supports parallel execution for improved performance.

## What it demonstrates

- Creating a BestOfN module with custom scoring
- Parallel execution for speed (up to Nx faster)
- Early stopping with score thresholds
- Returning all candidates for analysis
- Domain-specific scoring functions

## Usage

```bash
cd examples/005_best_of_n
go run main.go
```

### With Harness Flags

```bash
go run main.go -verbose -format=json
go run main.go -concurrency=10 -timeout=60
```

### Environment Variables

```bash
export HARNESS_VERBOSE=true
export HARNESS_OUTPUT_FORMAT=json
go run main.go
```

## Expected Output

```
Topic: Machine Learning for Beginners
Generated 5 candidates

All Candidates Generated:
1. 10 Essential ML Concepts for Beginners 👑 WINNER
2. Machine Learning Made Simple: A Beginner's Journey
3. Getting Started with Machine Learning Today
4. Your First Steps in Machine Learning
5. Machine Learning Basics: The Complete Guide

🏆 Selected Best Title:
Title: 10 Essential ML Concepts for Beginners
Hook: Learn the fundamental concepts that every ML beginner needs to know
Score: 115.0
```

## Key Concepts

- **BestOfN**: Generate N candidates, pick the best
- **Parallel Execution**: Speed up generation with concurrency
- **Custom Scorer**: Define what "best" means for your use case
- **Early Stopping**: Save API calls when threshold met
- **ReturnAll**: Analyze all candidates, not just the winner

## See Also

- [001_predict](../001_predict/) - Basic single prediction
- [002_chain_of_thought](../002_chain_of_thought/) - Reasoning module
- [QUICKSTART.md](../../QUICKSTART.md) - Getting started guide
