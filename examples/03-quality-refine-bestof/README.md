# 03 - Quality Refine BestOf

**Email Drafting with Reasoning, Selection, and Refinement**

## What This Demonstrates

### Modules
- ✓ **ChainOfThought** - Structured reasoning for outlines
- ✓ **BestOfN** - Generate multiple candidates and select best
- ✓ **Refine** - Iterative improvement based on feedback
- ✓ **Predict** - Basic completion

### Adapters
- ✓ **Chat** - Natural conversation format

### Features
- ✓ **Few-shot learning** - Email style examples  
- ✓ **Custom scoring** - Domain-specific quality metrics
- ✓ **Multi-turn refinement** - Progressive enhancement
- ✓ **Verbose mode** - See refinement iterations
- ✓ **Module composition** - Pipeline multiple strategies

### Observability
- ✓ Per-step event tracking
- ✓ Scoring visibility
- ✓ Iteration counts

## Story Flow

### Pipeline Architecture
```
Step A: ChainOfThought + Few-shot → Outline
Step B: BestOfN (N=5) + Custom Scorer → Best Opening
Step C: Refine (2 iterations) → Tone Adjustment
Step D: Refine (2 iterations) → Add CTA
```

### Conversation
1. **Step A**: Create structured outline with reasoning (few-shot guided)
2. **Step B**: Generate 5 opening variants, score by length + directness
3. **Step C**: Refine for formality, deadline, and brevity
4. **Step D**: Add call-to-action for meeting scheduling

## Custom Scorer

```go
scorer := func(inputs map[string]interface{}, pred *dsgo.Prediction) (float64, error) {
    opening := pred.GetString("opening")
    words := len(strings.Fields(opening))
    
    // Length penalty
    lengthScore := 1.0
    if words > 50 {
        lengthScore = 0.5
    }
    
    // Directness reward
    directnessScore := strings.Contains(opening, "review") ? 1.0 : 0.5
    
    return (lengthScore + directnessScore) / 2.0, nil
}
```

## Run

```bash
cd examples/03-quality-refine-bestof
go run main.go
```

### With verbose mode
```bash
DSGO_LOG=pretty go run main.go
```

## Expected Output

```
=== Step A: Create Outline (ChainOfThought + Few-shot) ===
▶ stepA_outline.start module=cot few_shot=1

Outline:
1. Greeting
2. Context (PR #123 for new auth system)
3. Specific ask (review by Friday, focus on security)
4. Thank you

Rationale: Following professional email pattern with clear structure...
✓ stepA_outline.end 1234ms

=== Step B: Generate Best Opening (BestOfN N=5) ===
▶ stepB_bestof.start module=bestofn n=5
• bestofn.candidate score=0.65
• bestofn.candidate score=0.82
• bestofn.candidate score=0.71
• bestofn.candidate score=0.88
• bestofn.candidate score=0.59
• bestofn.select best_score=0.88

Best opening (score: 0.88):
Hi Dr. Chen,

I've completed PR #123 implementing the new authentication system. 
Could you review it by Friday, with particular attention to the 
security implications?

✓ stepB_bestof.end 3421ms candidates=5

=== Step C: Refine for Tone (Refine) ===
▶ stepC_refine.start module=refine iterations=2
• refine.iteration num=1
• refine.iteration num=2

Refined email:
Dear Dr. Chen,

I have completed PR #123 implementing the new authentication system. 
I would appreciate your review by Friday, November 10th, with 
particular attention to security implications.

[Body sections...]

Best regards,
Alex

✓ stepC_refine.end 2156ms iterations=2

=== Step D: Further Refinement (Multi-turn) ===
▶ stepD_refine2.start
• refine.iteration num=1

Final email:
Dear Dr. Chen,

I have completed PR #123 implementing the new authentication system. 
I would appreciate your review by Friday, November 10th, with 
particular attention to security implications.

[Body sections...]

Could we schedule a 30-minute review session next week to discuss 
your feedback?

Best regards,
Alex

✓ stepD_refine2.end 1543ms
```

## Key Patterns

### Few-shot Examples
```go
examples := []dsgo.Example{
    dsgo.NewExample(inputs, outputs),
}
module := module.NewChainOfThought(sig, lm).WithDemos(examples)
```

### BestOfN with Scorer
```go
bestof := module.NewBestOfN(sig, lm, N, scorerFunc).
    WithReturnAll(false)
```

### Refinement Chain
```go
refine := module.NewRefine(sig, lm).
    WithMaxIterations(2).
    WithVerbose(true)
```
