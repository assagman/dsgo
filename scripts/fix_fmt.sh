#!/bin/bash
# Fix all fmt.Fprintf calls to ignore errors for the test script
perl -i -pe 's/(\s+)fmt\.Fprintf\(f,/$1_, _ = fmt.Fprintf(f,/g' scripts/test_examples.go
