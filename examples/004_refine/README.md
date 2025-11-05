# 004_refine - Iterative Refinement

## Overview

Demonstrates the Refine module for iteratively improving outputs. The module can take an initial result and repeatedly refine it based on feedback.

## What it demonstrates

- Creating a Refine module for iterative improvement
- Configuring maximum iterations
- Providing feedback for refinement
- Tracking changes across iterations
- Multi-output results (improved text + change summary)

## Usage

```bash
cd examples/004_refine
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
📝 Original text:
The product is good. It works fine. The price is okay.

✨ Refined text:
This exceptional product delivers outstanding performance while offering excellent value...

📊 Changes made:
Enhanced tone to be more engaging and professional, added specific benefits...
```

## Key Concepts

- **Refine**: Iteratively improves output based on feedback
- **Max Iterations**: Control how many refinement passes to make
- **Feedback Field**: Specify what aspect to focus on for improvement
- **Progressive Enhancement**: Each iteration builds on the previous result
- **Quality Control**: Can set constraints and validate improvements

## See Also

- [001_predict](../001_predict/) - Basic prediction without refinement
- [002_chain_of_thought](../002_chain_of_thought/) - Reasoning module
- [QUICKSTART.md](../../QUICKSTART.md) - Getting started guide
