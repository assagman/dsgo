package main

import (
	"context"
	"fmt"
	"log"
	"math"
	"sort"
	"strings"

	"github.com/assagman/dsgo"
	"github.com/assagman/dsgo/examples"
	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found, using environment variables")
	}

	fmt.Println("=== AI Data Analyst ===")
	fmt.Println("Demonstrates: ReAct agent with multiple analytical tools")
	fmt.Println()

	dataAnalystAgent()
}

func dataAnalystAgent() {
	ctx := context.Background()
	lm := examples.GetLM("gpt-4o-mini")

	// Define analytical tools
	
	// Tool 1: Calculate statistics
	statsTool := dsgo.NewTool(
		"calculate_statistics",
		"Calculate statistics (mean, median, std dev) for a dataset",
		func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			dataStr := args["data"].(string)
			numbers := parseNumbers(dataStr)
			
			if len(numbers) == 0 {
				return "No valid numbers found", nil
			}

			mean := calculateMean(numbers)
			median := calculateMedian(numbers)
			stdDev := calculateStdDev(numbers, mean)
			
			return fmt.Sprintf(
				"Statistics for dataset:\n"+
				"  Count: %d\n"+
				"  Mean: %.2f\n"+
				"  Median: %.2f\n"+
				"  Std Dev: %.2f\n"+
				"  Min: %.2f\n"+
				"  Max: %.2f",
				len(numbers), mean, median, stdDev, 
				numbers[0], numbers[len(numbers)-1],
			), nil
		},
	).AddParameter("data", "string", "Comma-separated numbers", true)

	// Tool 2: Find outliers
	outlierTool := dsgo.NewTool(
		"find_outliers",
		"Identify outliers in a dataset using IQR method",
		func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			dataStr := args["data"].(string)
			numbers := parseNumbers(dataStr)
			
			if len(numbers) < 4 {
				return "Need at least 4 data points to detect outliers", nil
			}

			q1 := calculatePercentile(numbers, 25)
			q3 := calculatePercentile(numbers, 75)
			iqr := q3 - q1
			lowerBound := q1 - (1.5 * iqr)
			upperBound := q3 + (1.5 * iqr)

			var outliers []float64
			for _, num := range numbers {
				if num < lowerBound || num > upperBound {
					outliers = append(outliers, num)
				}
			}

			if len(outliers) == 0 {
				return "No outliers detected", nil
			}

			return fmt.Sprintf(
				"Outlier Analysis:\n"+
				"  Q1: %.2f\n"+
				"  Q3: %.2f\n"+
				"  IQR: %.2f\n"+
				"  Valid Range: [%.2f, %.2f]\n"+
				"  Outliers Found: %v",
				q1, q3, iqr, lowerBound, upperBound, outliers,
			), nil
		},
	).AddParameter("data", "string", "Comma-separated numbers", true)

	// Tool 3: Compare datasets
	compareTool := dsgo.NewTool(
		"compare_datasets",
		"Compare two datasets and provide insights",
		func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			data1Str := args["dataset1"].(string)
			data2Str := args["dataset2"].(string)
			
			nums1 := parseNumbers(data1Str)
			nums2 := parseNumbers(data2Str)

			if len(nums1) == 0 || len(nums2) == 0 {
				return "One or both datasets are empty", nil
			}

			mean1 := calculateMean(nums1)
			mean2 := calculateMean(nums2)
			diff := ((mean2 - mean1) / mean1) * 100

			return fmt.Sprintf(
				"Dataset Comparison:\n"+
				"  Dataset 1: Mean=%.2f, Count=%d\n"+
				"  Dataset 2: Mean=%.2f, Count=%d\n"+
				"  Mean Difference: %.2f%%\n"+
				"  %s has higher average",
				mean1, len(nums1), mean2, len(nums2), math.Abs(diff),
				map[bool]string{true: "Dataset 2", false: "Dataset 1"}[mean2 > mean1],
			), nil
		},
	).AddParameter("dataset1", "string", "First dataset (comma-separated)", true).
		AddParameter("dataset2", "string", "Second dataset (comma-separated)", true)

	// Tool 4: Trend analysis
	trendTool := dsgo.NewTool(
		"analyze_trend",
		"Analyze trend in time series data",
		func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			dataStr := args["data"].(string)
			numbers := parseNumbers(dataStr)
			
			if len(numbers) < 3 {
				return "Need at least 3 data points for trend analysis", nil
			}

			// Simple linear trend detection
			increases := 0
			decreases := 0
			for i := 1; i < len(numbers); i++ {
				if numbers[i] > numbers[i-1] {
					increases++
				} else if numbers[i] < numbers[i-1] {
					decreases++
				}
			}

			trend := "stable"
			if increases > decreases*2 {
				trend = "increasing"
			} else if decreases > increases*2 {
				trend = "decreasing"
			}

			change := ((numbers[len(numbers)-1] - numbers[0]) / numbers[0]) * 100

			return fmt.Sprintf(
				"Trend Analysis:\n"+
				"  Overall Trend: %s\n"+
				"  Increases: %d\n"+
				"  Decreases: %d\n"+
				"  Total Change: %.2f%%\n"+
				"  Start: %.2f → End: %.2f",
				trend, increases, decreases, change,
				numbers[0], numbers[len(numbers)-1],
			), nil
		},
	).AddParameter("data", "string", "Time series data (comma-separated)", true)

	// Create signature for data analysis
	sig := dsgo.NewSignature("Analyze the given dataset and answer the question").
		AddInput("question", dsgo.FieldTypeString, "The analysis question").
		AddInput("data", dsgo.FieldTypeString, "The dataset").
		AddOutput("analysis", dsgo.FieldTypeString, "Detailed analysis").
		AddOutput("insights", dsgo.FieldTypeString, "Key insights").
		AddOutput("recommendation", dsgo.FieldTypeString, "Recommendations")

	tools := []dsgo.Tool{*statsTool, *outlierTool, *compareTool, *trendTool}

	react := dsgo.NewReAct(sig, lm, tools).
		WithMaxIterations(5).
		WithVerbose(true)

	// Example 1: Basic analysis
	fmt.Println("--- Example 1: Sales data analysis ---")
	inputs1 := map[string]interface{}{
		"question": "Analyze this monthly sales data. Are there any outliers? What's the trend?",
		"data":     "45000, 47000, 46500, 48000, 51000, 49000, 52000, 95000, 53000, 54000, 55000, 56000",
	}

	outputs1, err := react.Forward(ctx, inputs1)
	if err != nil {
		log.Printf("Error: %v\n", err)
		return
	}

	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Println("ANALYSIS REPORT")
	fmt.Println(strings.Repeat("=", 70))
	fmt.Printf("\n📊 ANALYSIS:\n%s\n", outputs1["analysis"])
	fmt.Printf("\n💡 KEY INSIGHTS:\n%s\n", outputs1["insights"])
	fmt.Printf("\n✅ RECOMMENDATIONS:\n%s\n", outputs1["recommendation"])
	fmt.Println(strings.Repeat("=", 70))

	// Example 2: Comparison
	fmt.Println("\n\n--- Example 2: Compare two products' ratings ---")
	inputs2 := map[string]interface{}{
		"question": "Compare Product A and Product B ratings. Which one performs better and by how much?",
		"data":     "Product A: 4.2, 4.5, 4.3, 4.6, 4.4, 4.7 | Product B: 3.8, 3.9, 4.0, 3.7, 3.9, 3.8",
	}

	outputs2, err := react.Forward(ctx, inputs2)
	if err != nil {
		log.Printf("Error: %v\n", err)
		return
	}

	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Println("COMPARISON REPORT")
	fmt.Println(strings.Repeat("=", 70))
	fmt.Printf("\n📊 ANALYSIS:\n%s\n", outputs2["analysis"])
	fmt.Printf("\n💡 KEY INSIGHTS:\n%s\n", outputs2["insights"])
	fmt.Printf("\n✅ RECOMMENDATIONS:\n%s\n", outputs2["recommendation"])
	fmt.Println(strings.Repeat("=", 70))
}

// Helper functions

func parseNumbers(s string) []float64 {
	// Handle both single dataset and multiple datasets separated by |
	s = strings.ReplaceAll(s, "|", ",")
	// Remove labels like "Product A:"
	parts := strings.Split(s, ",")
	
	var numbers []float64
	for _, part := range parts {
		part = strings.TrimSpace(part)
		// Remove non-numeric prefixes
		if idx := strings.Index(part, ":"); idx != -1 {
			part = part[idx+1:]
			part = strings.TrimSpace(part)
		}
		
		var num float64
		if _, err := fmt.Sscanf(part, "%f", &num); err == nil {
			numbers = append(numbers, num)
		}
	}
	
	sort.Float64s(numbers)
	return numbers
}

func calculateMean(numbers []float64) float64 {
	sum := 0.0
	for _, num := range numbers {
		sum += num
	}
	return sum / float64(len(numbers))
}

func calculateMedian(numbers []float64) float64 {
	n := len(numbers)
	if n%2 == 0 {
		return (numbers[n/2-1] + numbers[n/2]) / 2
	}
	return numbers[n/2]
}

func calculateStdDev(numbers []float64, mean float64) float64 {
	sum := 0.0
	for _, num := range numbers {
		diff := num - mean
		sum += diff * diff
	}
	variance := sum / float64(len(numbers))
	return math.Sqrt(variance)
}

func calculatePercentile(numbers []float64, percentile float64) float64 {
	index := (percentile / 100.0) * float64(len(numbers)-1)
	lower := int(math.Floor(index))
	upper := int(math.Ceil(index))
	
	if lower == upper {
		return numbers[lower]
	}
	
	weight := index - float64(lower)
	return numbers[lower]*(1-weight) + numbers[upper]*weight
}
