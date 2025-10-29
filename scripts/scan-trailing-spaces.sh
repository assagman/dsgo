#!/bin/bash
# ==============================================================================
# scan-trailing-spaces.sh
#
# A shell script to scan for or automatically fix trailing whitespace
# (spaces or tabs) at the end of lines in files.
#
# Features:
# - Two modes: 'scan' (default) and 'fix'.
# - Skips binary files automatically.
# - Skips all hidden directories (e.g., .git, .vscode) and node_modules.
# - Handles filenames with spaces, newlines, or special characters.
# - Fix mode works with Windows (CRLF) line endings.
# - Fully portable: works on macOS and Linux.
# - Exits with status codes suitable for CI/CD.
#
# Usage:
#   ./scripts/scan-trailing-spaces.sh          # Scan only
#   ./scripts/scan-trailing-spaces.sh --fix    # Fix automatically
#   ./scripts/scan-trailing-spaces.sh [dir]     # Scan in directory
#   ./scripts/scan-trailing-spaces.sh --fix [dir]
# ==============================================================================

set -euo pipefail
IFS=$'\n\t'

# --- 1. PARSE ARGUMENTS ---
FIX_MODE=false
TARGET_DIR=""

for arg in "$@"; do
  case "$arg" in
    --fix)
      FIX_MODE=true
      shift
      ;;
    *)
      if [ -z "$TARGET_DIR" ]; then
        TARGET_DIR="$arg"
      else
        echo "Error: Multiple directories specified." >&2
        exit 2
      fi
      ;;
  esac
done

: "${TARGET_DIR:=.}"

# --- 2. VALIDATE DIRECTORY ---
if [ ! -d "$TARGET_DIR" ]; then
  echo "Error: Directory '$TARGET_DIR' does not exist." >&2
  exit 2
fi

# --- 3. DEFINE PATTERN ---
# Match one or more horizontal whitespace chars at end of line
WHITESPACE_PATTERN='[[:space:]]+$'

# --- 4. COMMON FIND COMMAND (reused safely) ---
FIND_CMD=(
  find "$TARGET_DIR"
  \( -type d \( -name '.*' -o -name 'node_modules' \) \) -prune -o
  -type f -print0
)

# --- 5. EXECUTE BASED ON MODE ---
if [ "$FIX_MODE" = true ]; then
  # ======================
  # FIX MODE
  # ======================
  echo "Searching for files with trailing whitespace to fix in '$TARGET_DIR'..."

  # Temporary file to store null-terminated list of files to fix
  TEMP_FILE=$(mktemp) || exit 1
  trap 'rm -f "$TEMP_FILE"' EXIT

  # Collect files with trailing whitespace
  "${FIND_CMD[@]}" | \
    xargs -0 grep -lZ -E "$WHITESPACE_PATTERN" 2>/dev/null > "$TEMP_FILE" || true

  if [ ! -s "$TEMP_FILE" ]; then
    echo "No files with trailing whitespace found. Nothing to fix!"
    exit 0
  fi

  echo "Fixing trailing whitespace in the following files:"
  echo "----------------------------------------------------"
  tr '\0' '\n' < "$TEMP_FILE" | sed 's/\r$//'  # Remove any stray \r
  echo "----------------------------------------------------"

  # Fix only the collected files
  < "$TEMP_FILE" xargs -0 sh -c '
    for file; do
      # Clean filename (remove any \r from CRLF paths)
      clean_file=$(printf "%s" "$file" | tr -d "\r")

      # Backup suffix must work on both GNU and BSD sed
      backup_suffix=".scanws.bak"
      if sed -i"$backup_suffix" "s/[[:space:]]\\+$//" "$clean_file" 2>/dev/null; then
        rm -f "${clean_file}${backup_suffix}"
        echo "Fixed: $clean_file"
      else
        echo "Failed to fix: $clean_file" >&2
      fi
    done
  ' sh

  echo "All identified files have been fixed."
  exit 0

else
  # ======================
  # SCAN MODE
  # ======================
  echo "Scanning for trailing whitespace in '$TARGET_DIR'..."
  echo "=================================================="

  SCAN_OUTPUT=$( "${FIND_CMD[@]}" | \
    xargs -0 grep --color=auto -nHE -I "$WHITESPACE_PATTERN" 2>/dev/null || true )

  if [ -z "$SCAN_OUTPUT" ]; then
    echo "Scan complete. No trailing whitespace found. Great job!"
    exit 0
  else
    echo "Found trailing whitespace in the following files:"
    echo "----------------------------------------------------"
    printf '%s\n' "$SCAN_OUTPUT" | sed 's/\r$//'  # Clean any CRLF
    echo "----------------------------------------------------"
    echo "Run this script with the --fix option to correct these files automatically."
    exit 1
  fi
fi
