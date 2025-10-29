# AI Content Generator

Demonstrates **BestOfN with sophisticated custom scoring** for content generation tasks.

## Features Demonstrated

- **BestOfN Module**: Generate N candidates and select the best
- **Custom Scoring Functions**: Multi-dimensional quality assessment
- **Parallel Generation**: Optional parallel execution for speed
- **Domain-Specific Metrics**: Different scoring for different content types

## Use Cases

1. **Blog Titles**: Balance hook strength, SEO, and creativity
2. **Product Descriptions**: Optimize persuasiveness, clarity, and length
3. **Social Media Posts**: Platform-specific scoring (Twitter, LinkedIn, Instagram)

## Running the Example

```bash
export OPENAI_API_KEY=your_key_here
cd examples/content_generator
go run main.go
```

## What You'll Learn

- How to create sophisticated scoring functions
- How to balance multiple quality dimensions
- How to implement platform-specific rules
- How to use length and engagement metrics

## Scoring Strategies

### Blog Titles
```go
score = (hook * 0.4) + (seo * 0.3) + (creativity * 0.3)
```

### Product Descriptions
```go
// Prefers 50-150 word descriptions
score = (persuasiveness * 0.4) + (clarity * 0.4) + (length * 0.2)
```

### Social Media Posts
```go
// Platform-specific length penalties
score = (engagement * 0.7) + (length_appropriateness * 0.3)
```

## Key Code Patterns

```go
// Multi-dimensional custom scorer
customScorer := func(inputs, outputs map[string]interface{}) (float64, error) {
    hook := outputs["hook_strength"].(float64)
    seo := outputs["seo_score"].(float64)
    creativity := outputs["creativity"].(float64)
    
    // Weighted combination
    return (hook * 0.4) + (seo * 0.3) + (creativity * 0.3), nil
}

bestOf := dsgo.NewBestOfN(predict, 4).
    WithScorer(customScorer).
    WithReturnAll(true).  // See all candidates
    WithParallel(false)   // Sequential generation
```

## Example Results

The generator creates content optimized for:
- **Engagement**: Click-worthy titles and posts
- **SEO**: Search engine optimization
- **Platform Guidelines**: Character limits, hashtag counts
- **Brand Voice**: Tone and style consistency
