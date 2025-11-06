package harness

import (
	"flag"
	"os"
	"strconv"
	"time"
)

type FlagConfig struct {
	Concurrency  *int
	Timeout      *int
	ErrorDumpDir *string
	OutputFormat *string
	Verbose      *bool
}

func ParseFlags() (Config, *flag.FlagSet) {
	fs := flag.NewFlagSet("harness", flag.ContinueOnError)

	fc := FlagConfig{
		Concurrency:  fs.Int("concurrency", 50, "Number of concurrent executions"),
		Timeout:      fs.Int("timeout", 300, "Timeout in seconds per execution"),
		ErrorDumpDir: fs.String("error-dir", "examples/errors", "Directory for error dumps"),
		OutputFormat: fs.String("format", "summary", "Output format: json, ndjson, or summary"),
		Verbose:      fs.Bool("verbose", false, "Verbose output"),
	}

	fs.Parse(os.Args[1:])

	config := Config{
		Concurrency:  getIntFromEnv("HARNESS_CONCURRENCY", *fc.Concurrency),
		Timeout:      time.Duration(getIntFromEnv("HARNESS_TIMEOUT", *fc.Timeout)) * time.Second,
		ErrorDumpDir: getStringFromEnv("HARNESS_ERROR_DIR", *fc.ErrorDumpDir),
		OutputFormat: getStringFromEnv("HARNESS_OUTPUT_FORMAT", *fc.OutputFormat),
		Verbose:      getBoolFromEnv("HARNESS_VERBOSE", *fc.Verbose),
	}

	return config, fs
}

func getIntFromEnv(key string, defaultValue int) int {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return defaultValue
}

func getStringFromEnv(key string, defaultValue string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultValue
}

func getBoolFromEnv(key string, defaultValue bool) bool {
	if val := os.Getenv(key); val != "" {
		if b, err := strconv.ParseBool(val); err == nil {
			return b
		}
	}
	return defaultValue
}
