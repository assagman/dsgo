module github.com/assagman/dsgo/examples/codebase_analysis

go 1.25

require (
	github.com/assagman/dsgo v0.0.0
	github.com/assagman/dsgo/examples/shared v0.0.0-00010101000000-000000000000
)

require (
	github.com/openai/openai-go/v3 v3.13.0 // indirect
	github.com/tidwall/gjson v1.18.0 // indirect
	github.com/tidwall/match v1.1.1 // indirect
	github.com/tidwall/pretty v1.2.1 // indirect
	github.com/tidwall/sjson v1.2.5 // indirect
)

replace github.com/assagman/dsgo => ../..

replace github.com/assagman/dsgo/examples/shared => ../shared
