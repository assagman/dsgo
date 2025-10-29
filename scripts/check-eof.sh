#!/bin/bash

set -e

FIX_MODE=false
EXIT_CODE=0

if [[ "$1" == "--fix" ]]; then
    FIX_MODE=true
fi

while IFS= read -r -d '' file; do
    if [[ ! -f "$file" ]]; then
        continue
    fi
    
    # Skip binary files - check for common text extensions or file type
    if [[ "$file" =~ \.(md|txt|go|sh|yml|yaml|json|toml|mod|sum)$ ]] || file "$file" | grep -qE "(text|ASCII|UTF-8|empty)"; then
        if [[ -s "$file" ]]; then
            last_char=$(tail -c 1 "$file")
            if [[ -n "$last_char" ]]; then
                echo "Missing EOF newline: $file"
                EXIT_CODE=1
                
                if [[ "$FIX_MODE" == true ]]; then
                    echo "" >> "$file"
                    echo "  ✓ Fixed: $file"
                fi
            fi
        fi
    fi
done < <(git ls-files -z)

if [[ $EXIT_CODE -eq 0 ]]; then
    echo "✓ All tracked text files end with newline"
fi

exit $EXIT_CODE
