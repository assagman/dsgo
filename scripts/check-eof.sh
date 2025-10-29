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
    
    if file "$file" | grep -q "text"; then
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
