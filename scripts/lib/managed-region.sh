#!/usr/bin/env bash
# scripts/lib/managed-region.sh — read/write BEGIN/END managed regions in
# markdown pages. Marker style: <!-- BEGIN <kind>:<id> --> ... <!-- END <kind>:<id> -->
#
# Functions (sourced):
#   managed_region_replace <page> <kind> <id> <body-file>
#     Replace the body inside the named region. Insert a fresh region appended
#     to EOF if absent. Idempotent: byte-identical when body file unchanged.
#
#   managed_region_extract <page> <kind> <id>
#     Print body on stdout. Empty if region absent.

managed_region_replace() {
  local page="$1" kind="$2" id="$3" body_file="$4"
  python3 - "$page" "$kind" "$id" "$body_file" <<'PY'
import re, sys, pathlib
page, kind, ident, body_file = sys.argv[1:5]
text = pathlib.Path(page).read_text(encoding="utf-8")
body = pathlib.Path(body_file).read_text(encoding="utf-8")
if not body.endswith("\n"):
    body += "\n"
begin = f"<!-- BEGIN {kind}:{ident} -->"
end = f"<!-- END {kind}:{ident} -->"
new_block = f"{begin}\n{body}{end}"
pat = re.compile(
    rf"<!-- BEGIN {re.escape(kind)}:{re.escape(ident)} -->\n.*?\n<!-- END {re.escape(kind)}:{re.escape(ident)} -->",
    re.S,
)
if pat.search(text):
    out = pat.sub(new_block, text)
else:
    sep = "" if text.endswith("\n") else "\n"
    out = text + sep + "\n" + new_block + "\n"
pathlib.Path(page).write_text(out, encoding="utf-8")
PY
}

managed_region_extract() {
  local page="$1" kind="$2" id="$3"
  python3 - "$page" "$kind" "$id" <<'PY'
import re, sys, pathlib
page, kind, ident = sys.argv[1:4]
text = pathlib.Path(page).read_text(encoding="utf-8")
pat = re.compile(
    rf"<!-- BEGIN {re.escape(kind)}:{re.escape(ident)} -->\n(.*?)\n<!-- END {re.escape(kind)}:{re.escape(ident)} -->",
    re.S,
)
m = pat.search(text)
print(m.group(1) if m else "", end="")
PY
}
