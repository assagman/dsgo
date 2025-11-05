# 001_predict - Basic Prediction

## Overview

Basic sentiment classification using the Predict module. Demonstrates the simplest form of LM interaction with structured I/O.

## What it demonstrates

- Creating a Signature with inputs and outputs
- Using class outputs for classification
- Basic Predict module execution
- Structured output access

## Usage

```bash
cd examples/001_predict
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
Input: I absolutely love this product! It exceeded all my expectations.
Sentiment: positive
Confidence: 0.95
```

## Key Concepts

- **Signature**: Defines I/O structure (text → sentiment + confidence)
- **Predict**: Simplest module, direct LM call
- **Class Output**: Constrained to predefined values (positive/negative/neutral)

## See Also

- [013_sentiment](../013_sentiment/) - Chain of Thought reasoning
- [QUICKSTART.md](../../QUICKSTART.md) - Getting started guide
