#!/usr/bin/env bats

setup() {
  command -v pdftotext >/dev/null 2>&1 || skip "pdftotext not installed"
  WORK="$(mktemp -d)/repo"
  mkdir -p "$WORK/raw/inbox/interactive" "$WORK/raw/processed"
  # Write minimal valid PDF (one-page hello). Uses python's reportlab if available, else skip.
  if ! python3 -c "from reportlab.pdfgen import canvas" 2>/dev/null; then skip "reportlab not installed"; fi
  python3 - "$WORK/raw/inbox/interactive/hello.pdf" <<PY
from reportlab.pdfgen import canvas
import sys
c = canvas.Canvas(sys.argv[1])
c.drawString(72, 720, "Hello PDF")
c.showPage(); c.save()
PY
  cd "$WORK"
}
teardown() { cd - >/dev/null; rm -rf "$WORK"; }

@test "ingest-pdf produces markdown with frontmatter and stashes original" {
  bash "$BATS_TEST_DIRNAME/../scripts/ingest-pdf.sh" raw/inbox/interactive/hello.pdf
  [ -f raw/inbox/interactive/hello.md ]
  [ -f raw/processed/_originals/hello.pdf ]
  [ ! -f raw/inbox/interactive/hello.pdf ]
  run grep -E '^title: "hello"' raw/inbox/interactive/hello.md
  [ "$status" -eq 0 ]
}

@test "ingest-pdf rejects path outside raw/inbox/" {
  cp raw/inbox/interactive/hello.md elsewhere.pdf 2>/dev/null || true
  run bash "$BATS_TEST_DIRNAME/../scripts/ingest-pdf.sh" /tmp/elsewhere.pdf
  [ "$status" -ne 0 ]
}
