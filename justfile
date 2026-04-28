set shell := ["bash", "-uc"]

default:
    @just --list

# === ingest ===
# Bookkeeping only: moves the source, logs, lints, reindexes. Prints
# `AGENT-PROMPT|...` for the agent (or human) to drive steps 3-9 of the
# WIKI.md §4.1 ingest workflow (read source, write summary, update
# entity/concept pages, update catalog).
ingest path:
    bash scripts/ingest.sh {{path}}

# Same, plus shells the emitted prompt to an agent CLI for non-
# interactive `batch` queue processing. Skips invocation for interactive/
# checkpoint queues (human-in-the-loop). Default CLI: claude. Override
# via AWIKI_AGENT in .awiki/config or pass explicitly.
ingest-with-agent path agent="claude":
    bash scripts/ingest.sh --agent {{agent}} {{path}}

ingest-xlsx path *flags:
    bash scripts/ingest-xlsx.sh {{path}} {{flags}}

ingest-batch-list:
    @find raw/inbox/batch -type f | sort

# Auto-ingest documents landing in raw/inbox/batch/. Foreground daemon —
# Ctrl-C to stop. Uses fswatch (mac) / inotifywait (linux); falls back to
# polling when neither is available. `--catchup` drains existing files at
# startup. Failed ingests are quarantined to raw/inbox/batch/_failed/.
watchdog *args:
    bash scripts/watchdog.sh {{args}}

# === git-docs ingest (phase 20) ===
ingest-git spec *flags:
    bash scripts/ingest-git.sh {{spec}} {{flags}}

ingest-git-list:
    @ls -1 .awiki/git-state/ 2>/dev/null | sed 's/\.json$//'

# === maintenance ===
lint:
    bash scripts/lint.sh

lint-fix:
    bash scripts/lint.sh --fix

reindex:
    bash scripts/qmd-index.sh

search query:
    qmd --index .qmd/index.sqlite search "{{query}}"

log action *message:
    bash scripts/log-append.sh {{action}} {{message}}

rename old new:
    bash scripts/rename.sh {{old}} {{new}}

delete slug:
    bash scripts/delete-page.sh {{slug}}

# === synthesis ===
synth plugin topic *args:
    bash scripts/synth.sh new -- {{plugin}} {{topic}} {{args}}

synth-regen slug *args:
    bash scripts/synth.sh regen -- {{slug}} {{args}}

synth-finalize slug:
    bash scripts/synth.sh finalize -- {{slug}}

synth-accept-stage slug:
    bash scripts/synth.sh accept-stage -- {{slug}}

synth-refine slug *note:
    bash scripts/synth.sh refine -- {{slug}} "{{note}}"

synth-list:
    bash scripts/synth.sh list

synth-resolve slug:
    bash scripts/synth.sh resolve -- {{slug}}

# === hugo ===
serve:
    bash scripts/serve.sh

build:
    bash scripts/build.sh --full

# === bootstrap / setup ===
init:
    @echo "Open agent. Say: 'init wiki'. Agent reads BOOTSTRAP.md."

install-hooks:
    bash scripts/install-hooks.sh

install-qmd:
    bash scripts/install-qmd.sh

encrypt-init:
    bash scripts/encrypt-init.sh

check-deps:
    bash scripts/check-deps.sh

# === git ===
status:
    git status -s

commit message:
    git add -A && git commit -m "{{message}}"

# === tests ===
test:
    bats tests/

# === help ===
help:
    @cat docs/just-help.txt

# === task layer (phases 16-17) ===
task-init:
    bash scripts/task-init.sh

# === data layer (opt-in) ===
# Per-step idempotent enabler for datasets + charts. Mirrors task-init.
data-init:
    bash scripts/data-init.sh

# Scaffold a new dataset page. Use --from=<csv-path> to seed rows.
dataset-new slug *args:
    bash scripts/dataset.sh new {{slug}} {{args}}

# Flip inline <-> file based on threshold (.awiki/config). Idempotent.
dataset-compact slug:
    bash scripts/dataset.sh compact {{slug}}

# Refresh rows + run schema validation if columns: declared.
dataset-validate slug:
    bash scripts/dataset.sh validate {{slug}}

# Show per-recipe data-layer help.
data-help:
    cat docs/data-help.txt

# Scaffold a type:chart page that references an existing dataset.
chart-new slug *args:
    bash scripts/chart.sh new {{slug}} {{args}}

# Walk every vega-lite fence + type:chart page; regen stale SVG sidecars
# under assets/charts/. Uses scripts/lib/vendor-vega.sh if vendored bundle
# absent. Requires the `vl-convert` Rust binary.
charts-render:
    bash scripts/chart.sh render

# Single-chart regen for fast iteration. Argument is the chart-id
# (`<page-slug>-fig<N>` for inline charts, `<slug>` for type:chart pages).
charts-render-one chart_id:
    bash scripts/chart.sh render-one {{chart_id}}

capture *text:
    bash scripts/capture.sh -- {{text}}

scan:
    bash scripts/action-scan.sh

agenda:
    bash scripts/action-scan.sh
    bash scripts/agenda.sh

# === task layer (phase 18a) ===
# Run the triage walker over content/inbox.md and raw/inbox/interactive/.
# For non-interactive single-item application, use:
#   just triage-apply <id> <outcome> [k=v ...]
triage:
    bash scripts/triage.sh --interactive

triage-apply *args:
    bash scripts/triage.sh {{args}}

# Re-emit open copies for any [x] every:... lines whose next-due is missing.
recur:
    bash scripts/action-recur.sh --all

# Dry-run variant — prints unified diff, makes no changes.
recur-dry:
    bash scripts/action-recur.sh --dry-run --all

# === task layer (phase 19) ===
# Run the weekly-review chain: rebuild agenda regions, lint the wiki,
# and emit the structured REVIEW|... report on stdout.
review:
    bash scripts/agenda.sh
    bash scripts/lint.sh
    bash scripts/review-status.sh

# === template ===
template-init:
    bash scripts/template-init.sh --repo "${TEMPLATE_REPO:?TEMPLATE_REPO required}" \
                                   --ref "${TEMPLATE_REF:-main}" \
                                   --version "${TEMPLATE_VERSION:?TEMPLATE_VERSION required}" \
                                   --commit "${TEMPLATE_COMMIT:?TEMPLATE_COMMIT required}"

# === template (added Phase 09) ===
template-update *args:
    bash scripts/template-update.sh {{args}}

template-status:
    bash scripts/template-update.sh --status

template-gc:
    bash scripts/template-update.sh --gc

bootstrap-step id:
    bash scripts/template-step.sh {{id}}

template-retrofit:
    bash scripts/template-retrofit.sh
