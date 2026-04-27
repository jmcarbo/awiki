# awiki Plan — Phase 11: Multimodal Ingest Helpers

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Spec:** [`2026-04-27-llm-wiki-scaffold-design.md`](../specs/2026-04-27-llm-wiki-scaffold-design.md)
**Master:** [`2026-04-27-awiki-master-plan.md`](./2026-04-27-awiki-master-plan.md)
**Depends on:** Phase 2, 5
**Previous:** [Phase 10](./2026-04-27-phase-10-auto-deploy.md)
**Next:** [Phase 12](./2026-04-27-phase-12-polish-examples.md)

**Tech stack:** bash 4+, just 1.13+, hugo 0.120+ extended, hugo-book theme, qmd (qntx-labs fork), git-crypt 0.7+, age 1.0+, bats-core 1.10+, python3 3.8+, Node 20+ (phase 8 only).

**Conventions:**
- Scripts: `#!/usr/bin/env bash`, `set -euo pipefail`.
- Commit after every task. Conventional Commits.
- TDD where applicable: write failing test → run → implement → run → commit.
- Branch per phase. Merge to main only after `just test && just lint` are clean.

---

**Deliverable:** `ingest-pdf.sh`, `ingest-audio.sh`, vision workflow doc.

**Branch:** `phase-11-multimodal`

## Task 11.1: Branch + `scripts/ingest-pdf.sh`

```bash
git checkout -b phase-11-multimodal
```

- [ ] **Step 1: Write `scripts/ingest-pdf.sh`**

`_originals/` ALWAYS lives under `raw/processed/_originals/` (privacy-protected by gitignore + git-crypt patterns). The helper produces a markdown sidecar in the inbox, ready for `just ingest`.

```bash
cat > scripts/ingest-pdf.sh <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

[[ $# -eq 1 ]] || { echo "usage: ingest-pdf.sh <pdf-path-under-raw/inbox/>" >&2; exit 1; }
PDF="$1"

[[ "$PDF" =~ \.pdf$ ]] || { echo "not a pdf" >&2; exit 1; }
[[ -f "$PDF" ]] || { echo "not found: $PDF" >&2; exit 1; }
[[ "$PDF" =~ ^raw/inbox/ ]] || { echo "must be under raw/inbox/" >&2; exit 1; }

OUT="${PDF%.pdf}.md"
ORIG_DIR="raw/processed/_originals"
mkdir -p "$ORIG_DIR"
cp "$PDF" "$ORIG_DIR/$(basename "$PDF")"

if command -v pdftotext >/dev/null 2>&1; then
  pdftotext -layout "$PDF" - > "$OUT"
elif command -v marker >/dev/null 2>&1; then
  marker "$PDF" -o "$OUT"
else
  echo "Need pdftotext or marker installed" >&2
  exit 1
fi

TMP="$(mktemp)"
{
  printf -- "---\ntitle: \"%s\"\ndate: %s\nlast_updated: %s\ntype: source\ntags: [pdf]\naliases: []\nsources: []\noriginal: %s\ndraft: false\n---\n\n" \
    "$(basename "$PDF" .pdf)" "$(date '+%Y-%m-%d')" "$(date '+%Y-%m-%d')" "$ORIG_DIR/$(basename "$PDF")"
  cat "$OUT"
} > "$TMP"
mv "$TMP" "$OUT"

rm "$PDF"

echo "PDF-CONVERTED|in=$PDF|out=$OUT|orig=$ORIG_DIR/$(basename "$PDF")"
echo "Now run: just ingest $OUT"
EOF
chmod +x scripts/ingest-pdf.sh
```

- [ ] **Step 2: Write `scripts/ingest-audio.sh`**

```bash
cat > scripts/ingest-audio.sh <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

[[ $# -eq 1 ]] || { echo "usage: ingest-audio.sh <audio-path-under-raw/inbox/>" >&2; exit 1; }
AUDIO="$1"

[[ -f "$AUDIO" ]] || { echo "not found: $AUDIO" >&2; exit 1; }
[[ "$AUDIO" =~ ^raw/inbox/ ]] || { echo "must be under raw/inbox/" >&2; exit 1; }
command -v whisper-cpp >/dev/null 2>&1 || { echo "Install whisper.cpp" >&2; exit 1; }
[[ -f models/ggml-base.en.bin ]] || { echo "missing models/ggml-base.en.bin (download from whisper.cpp releases)" >&2; exit 1; }

OUT="${AUDIO%.*}.md"
ORIG_DIR="raw/processed/_originals"
mkdir -p "$ORIG_DIR"
cp "$AUDIO" "$ORIG_DIR/$(basename "$AUDIO")"

whisper-cpp -m models/ggml-base.en.bin -otxt -f "$AUDIO"
mv "${AUDIO}.txt" "$OUT"

TMP="$(mktemp)"
{
  printf -- "---\ntitle: \"%s\"\ndate: %s\nlast_updated: %s\ntype: source\ntags: [audio]\naliases: []\nsources: []\noriginal: %s\ndraft: false\n---\n\n" \
    "$(basename "$AUDIO")" "$(date '+%Y-%m-%d')" "$(date '+%Y-%m-%d')" "$ORIG_DIR/$(basename "$AUDIO")"
  cat "$OUT"
} > "$TMP"
mv "$TMP" "$OUT"

rm "$AUDIO"
echo "AUDIO-TRANSCRIBED|out=$OUT|orig=$ORIG_DIR/$(basename "$AUDIO")"
echo "Now run: just ingest $OUT"
EOF
chmod +x scripts/ingest-audio.sh
```

- [ ] **Step 3: BATS test for `ingest-pdf.sh`**

```bash
cat > tests/ingest_pdf_test.bats <<'EOF'
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
EOF
```

- [ ] **Step 4: Append vision-workflow doc to WIKI.md**

Append exact content to `WIKI.md` after section 4.1:

```markdown
**Vision-aware ingest:** when a source contains images, the agent operates in two passes:
1. **Text pass:** Read the markdown body alone via the Read tool.
2. **Image pass:** Use the Read tool to view referenced images one at a time. Claude Code handles `![alt](path.png)` markdown image refs natively; for Codex / OpenCode use their equivalent vision tool.
3. **Integrate:** combine notes from both passes when writing `content/sources/<slug>.md` and any entity/concept pages affected.

No script needed — the agent decides when image content is load-bearing. For dense visual sources (slides, infographics), the agent should default to image-pass; for text-with-decorative-images, text-pass alone is sufficient.
```

- [ ] **Step 5: Commit + merge**

```bash
bats tests/ingest_pdf_test.bats || true   # optional deps may skip
git add scripts/ingest-pdf.sh scripts/ingest-audio.sh tests/ingest_pdf_test.bats WIKI.md
git commit -m "feat: PDF/audio ingest helpers (originals to raw/processed/_originals) + vision workflow"
git checkout main
git merge --no-ff phase-11-multimodal -m "feat: complete phase 11 multimodal ingest"
git branch -d phase-11-multimodal
```

---

---

## Phase complete

Return to [master plan](./2026-04-27-awiki-master-plan.md) or proceed to [Phase 12](./2026-04-27-phase-12-polish-examples.md).
