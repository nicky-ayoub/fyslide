#!/bin/bash
# check-threading.sh: Detect potential Fyne threading violations in Go code
# Usage: ./check-threading.sh <file.go> [file2.go ...]

set -euo pipefail

if [[ $# -eq 0 ]]; then
    echo "Usage: $0 <file.go> [file2.go ...]"
    echo ""
    echo "Detects Go code that uses 'go func()' without wrapping UI calls in fyne.Do()."
    echo ""
    echo "CAUTION: This is a heuristic—it may have false positives/negatives."
    exit 1
fi

RED='\033[0;31m'
YELLOW='\033[1;33m'
GREEN='\033[0;32m'
NC='\033[0m' # No Color

check_file() {
    local file="$1"
    local found=0
    
    # Look for goroutines
    local in_goroutine=0
    local brace_depth=0
    local line_no=0
    
    while IFS= read -r line; do
        ((line_no++))
        
        # Simple heuristic: look for 'go func()' and check if UI operations appear before next fyne.Do
        if [[ "$line" =~ go[[:space:]]*func\( ]]; then
            # Entering goroutine — scan next lines until we find either fyne.Do or end of function
            local func_start=$line_no
            local has_fyne_do=0
            local ui_ops=()
            local inner_brace=0
            
            # Check following lines
            local remaining_lines=$(($(wc -l < "$file") - line_no))
            for ((i=1; i<=remaining_lines && i<=100; i++)); do
                read -r next_line || break
                ((line_no++))
                
                # Track braces to know when goroutine ends
                ((inner_brace += $(grep -o '{' <<< "$next_line" | wc -l)))
                ((inner_brace -= $(grep -o '}' <<< "$next_line" | wc -l)))
                
                # Check for UI operations that need wrapping
                if [[ "$next_line" =~ (\.SetImage|\.Refresh|\.SetIcon|\.SetTitle|updateStatusBar|RefreshTags|UpdateInfoText|MainMenu|Toolbar|canvas\.) ]]; then
                    ui_ops+=("$line_no: $next_line")
                fi
                
                # Check for fyne.Do or fyne.DoAndWait
                if [[ "$next_line" =~ fyne\.(Do|DoAndWait) ]]; then
                    has_fyne_do=1
                    break
                fi
                
                # End of goroutine scope
                if [[ $inner_brace -lt 0 ]]; then
                    break
                fi
            done
            
            # Report findings
            if [[ ${#ui_ops[@]} -gt 0 ]] && [[ $has_fyne_do -eq 0 ]]; then
                if [[ $found -eq 0 ]]; then
                    echo -e "${RED}Threading Violations in $file:${NC}"
                    found=1
                fi
                echo -e "  ${YELLOW}Line $func_start: goroutine without fyne.Do() wrapping${NC}"
                for op in "${ui_ops[@]}"; do
                    echo -e "    - $op"
                done
                echo ""
            fi
        fi
    done < "$file"
    
    if [[ $found -eq 0 ]]; then
        echo -e "${GREEN}✓ $file: No obvious threading violations detected${NC}"
    fi
}

for file in "$@"; do
    if [[ ! -f "$file" ]]; then
        echo -e "${RED}Error: File not found: $file${NC}" >&2
        exit 1
    fi
    check_file "$file"
done
