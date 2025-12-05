module github.com/assagman/dsgo/examples/security_scan

go 1.25

require (
	github.com/assagman/dsgo v0.0.0
	github.com/assagman/dsgo/examples/shared v0.0.0
)

replace github.com/assagman/dsgo => ../..

replace github.com/assagman/dsgo/examples/shared => ../shared
