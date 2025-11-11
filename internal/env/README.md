# internal/env

This package provides functionality for loading environment variables from `.env` files, serving as an internal replacement for `github.com/joho/godotenv`.

## Features

- Load environment variables from `.env` files
- Support for quoted values (both single and double quotes)
- Support for `export` prefix (bash style)
- Comments with `#` are ignored
- Environment variables are only set if not already set
- File search functionality that walks up the directory tree

## Usage

```go
import "github.com/assagman/dsgo/internal/env"

// Load a specific .env file
err := env.Load(".env")

// Search for and load .env and .env.local files
err := env.LoadFiles()
```

## Precedence

- `.env.local` takes precedence over `.env` files
- Environment variables are only set if not already set, so process environment takes precedence over file values

## Supported Formats

```
# Comment line
SIMPLE=value
QUOTED="quoted value"
SINGLE_QUOTED='single quoted value'
EXPORT_STYLE=export value
VALUE_WITH_SPACES=value with spaces
```
