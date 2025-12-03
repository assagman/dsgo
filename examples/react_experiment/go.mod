module github.com/assagman/dsgo/examples/react_experiment

go 1.25.0

replace github.com/assagman/dsgo => ../../

replace github.com/assagman/dsgo/examples/shared => ../shared

require (
	github.com/assagman/dsgo v0.0.0
	github.com/assagman/dsgo/examples/shared v0.0.0-00010101000000-000000000000
)
