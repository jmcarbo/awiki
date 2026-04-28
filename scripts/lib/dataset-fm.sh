#!/usr/bin/env bash
# scripts/lib/dataset-fm.sh — frontmatter scalar helpers for awiki datasets.
# Source this file (do not execute). Operates on YAML frontmatter only;
# does NOT support nested mappings (callers go through python for those).

# fm_get <file> <key> -> value to stdout (empty if absent)
fm_get() {
  local file="$1" key="$2"
  awk -v key="$key" '
    BEGIN { in_fm=0; opened=0 }
    /^---[[:space:]]*$/ { if (!opened) { in_fm=1; opened=1; next } else { in_fm=0; exit } }
    in_fm {
      n = index($0, ":")
      if (n == 0) next
      k = substr($0, 1, n-1)
      v = substr($0, n+1)
      sub(/^[[:space:]]+/, "", v)
      sub(/[[:space:]]+$/, "", v)
      gsub(/^"|"$/, "", v)
      if (k == key) { print v; exit }
    }
  ' "$file"
}

# fm_set <file> <key> <value> — update or insert before closing ---
fm_set() {
  local file="$1" key="$2" value="$3"
  awk -v key="$key" -v value="$value" '
    BEGIN { in_fm=0; opened=0; replaced=0 }
    /^---[[:space:]]*$/ {
      if (!opened) { in_fm=1; opened=1; print; next }
      if (in_fm) {
        if (!replaced) { print key ": " value }
        in_fm=0
        print
        next
      }
      print; next
    }
    in_fm {
      n = index($0, ":")
      if (n > 0) {
        k = substr($0, 1, n-1)
        if (k == key) { print key ": " value; replaced=1; next }
      }
    }
    { print }
  ' "$file" > "$file.tmp"
  mv "$file.tmp" "$file"
}

# fm_remove <file> <key>
fm_remove() {
  local file="$1" key="$2"
  awk -v key="$key" '
    BEGIN { in_fm=0; opened=0 }
    /^---[[:space:]]*$/ { if (!opened) { in_fm=1; opened=1; print; next } else { in_fm=0; print; next } }
    in_fm {
      n = index($0, ":")
      if (n > 0) {
        k = substr($0, 1, n-1)
        if (k == key) next
      }
    }
    { print }
  ' "$file" > "$file.tmp"
  mv "$file.tmp" "$file"
}
