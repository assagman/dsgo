# cost

Cost calculation helpers built on top of the model catalog.

## Overview

- `Pricing` is an alias of `modelcatalog.Pricing` (USD per 1M tokens).
- `Calculator` computes costs using catalog pricing with optional overrides.
- `DefaultCalculator` is the package-level instance used by `cost.Calculate`.

## Usage

```go
calc := cost.NewCalculator()
calc.SetModelPricing("openai/gpt-4o", cost.Pricing{
    PromptPrice:     1.0,
    CompletionPrice: 2.0,
})

usd := calc.Calculate("openai/gpt-4o", promptTokens, completionTokens)
```

## Notes

- `Calculate` returns 0 when pricing is unavailable.
- The calculator is safe for concurrent use through its methods.
