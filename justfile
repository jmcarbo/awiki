set shell := ["bash", "-uc"]

# Prepend repo-local bin/ to PATH so all recipes find `awiki` without
# the user having to add bin/ to their shell PATH.
export PATH := absolute_path("bin") + ":" + env_var("PATH")

default:
    @just --list

# Build the awiki Go binary into bin/awiki. Recipes prepend bin/ to
# PATH so this binary is what `awiki <verb>` resolves to. Run this
# once after clone, and after each `git pull` that touches Go sources.
build-self:
    @mkdir -p bin
    go build -o bin/awiki ./cmd/awiki

# === ingest ===
# Bookkeeping only: moves the source, logs, lints, reindexes. Prints
# `AGENT-PROMPT|...` for the agent (or human) to drive steps 3-9 of the
# WIKI.md §4.1 ingest workflow (read source, write summary, update
# entity/concept pages, update catalog).
ingest path:
    awiki ingest {{path}}

# Same, plus shells the emitted prompt to an agent CLI for non-
# interactive `batch` queue processing. Skips invocation for interactive/
# checkpoint queues (human-in-the-loop). Default CLI: claude. Override
# via AWIKI_AGENT in .awiki/config or pass explicitly.
ingest-with-agent path agent="claude":
    awiki ingest --agent {{agent}} {{path}}

ingest-xlsx path *flags:
    awiki ingest-xlsx {{path}} {{flags}}

ingest-batch-list:
    @awiki ingest-batch-list

# Auto-ingest documents landing in raw/inbox/batch/. Foreground daemon —
# Ctrl-C to stop. Uses fsnotify; falls back to polling when fsnotify
# init fails. `--catchup` drains existing files at startup. Failed
# ingests are quarantined to raw/inbox/batch/_failed/.
watchdog *args:
    awiki watchdog {{args}}

# === git-docs ingest (phase 20) ===
ingest-git spec *flags:
    awiki ingest-git {{spec}} {{flags}}

ingest-git-list:
    @awiki ingest-git-list

# === maintenance ===
lint:
    awiki lint

lint-fix:
    awiki lint --fix

reindex:
    awiki reindex

search query:
    qmd --index .qmd/index.sqlite search "{{query}}"

log action *message:
    awiki log {{action}} {{message}}

rename old new:
    awiki rename {{old}} {{new}}

delete slug:
    awiki delete {{slug}}

update-catalog:
    awiki update-catalog

# === synthesis ===
synth plugin topic *args:
    awiki synth new -- {{plugin}} {{topic}} {{args}}

synth-regen slug *args:
    awiki synth regen -- {{slug}} {{args}}

synth-finalize slug:
    awiki synth finalize -- {{slug}}

synth-accept-stage slug:
    awiki synth accept-stage -- {{slug}}

synth-refine slug *note:
    awiki synth refine -- {{slug}} "{{note}}"

synth-list:
    awiki synth list

synth-resolve slug:
    awiki synth resolve -- {{slug}}

# === hugo ===
serve:
    awiki serve

build:
    awiki build --full

# === bootstrap / setup ===
init:
    @echo "Open agent. Say: 'init wiki'. Agent reads BOOTSTRAP.md."

# Same as `init` but launches the agent CLI directly with the prompt.
# Example: `just init-agent claude` → `claude -p "init wiki"`.
# Default agent: claude. Override: `just init-agent codex`.
init-agent agent="claude" *flags:
    {{agent}} -p "init wiki" {{flags}}

install-hooks:
    awiki install-hooks

install-qmd:
    awiki install-qmd

encrypt-init:
    bash scripts/encrypt-init.sh

check-deps:
    awiki check-deps

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
    awiki task-init

# === data layer (opt-in) ===
# Per-step idempotent enabler for datasets + charts. Mirrors task-init.
data-init:
    awiki data-init

# Scaffold a new dataset page. Use --from=<csv-path> to seed rows.
dataset-new slug *args:
    awiki dataset new {{slug}} {{args}}

# Flip inline <-> file based on threshold (.awiki/config). Idempotent.
dataset-compact slug:
    awiki dataset compact {{slug}}

# Refresh rows + run schema validation if columns: declared.
dataset-validate slug:
    awiki dataset validate {{slug}}

# Show per-recipe data-layer help.
data-help:
    cat docs/data-help.txt

# Scaffold a type:chart page that references an existing dataset.
chart-new slug *args:
    awiki chart new {{slug}} {{args}}

# Walk every vega-lite fence + type:chart page; regen stale SVG sidecars
# under assets/charts/. Uses scripts/lib/vendor-vega.sh if vendored bundle
# absent. Requires the `vl-convert` Rust binary.
charts-render:
    awiki chart render

# Single-chart regen for fast iteration. Argument is the chart-id
# (`<page-slug>-fig<N>` for inline charts, `<slug>` for type:chart pages).
charts-render-one chart_id:
    awiki chart render-one {{chart_id}}

# === Query layer ===

# Run an ad-hoc SQL query against awiki datasets.
# Use --out=<slug> to materialize the result as a dataset page.
query SQL *args:
    awiki query run "{{SQL}}" {{args}}

# Scaffold a type:query page that materializes to a sibling dataset.
query-new slug *args:
    awiki query new {{slug}} {{args}}

# Re-run every materialized query and refresh inline awiki-query fences.
query-render:
    awiki query render

# Single-query regen for fast iteration.
query-render-one slug:
    awiki query render-one {{slug}}

# Re-run every inline awiki-query fence in content/.
query-fence-render:
    awiki query fence-render

capture *text:
    awiki capture -- {{text}}

scan:
    awiki scan

agenda:
    awiki scan
    awiki agenda

# === task layer (phase 18a) ===
# Run the triage walker over content/inbox.md and raw/inbox/interactive/.
# For non-interactive single-item application, use:
#   just triage-apply <id> <outcome> [k=v ...]
triage:
    awiki triage

triage-apply *args:
    awiki triage-apply {{args}}

# Re-emit open copies for any [x] every:... lines whose next-due is missing.
recur:
    awiki recur

# Dry-run variant — prints unified diff, makes no changes.
recur-dry:
    awiki recur-dry

# === task layer (phase 19) ===
# Run the weekly-review chain: rebuild agenda regions, lint the wiki,
# and emit the structured REVIEW|... report on stdout.
review:
    awiki review

# === template ===
template-init:
    awiki template init --repo "${TEMPLATE_REPO:?TEMPLATE_REPO required}" \
                        --ref "${TEMPLATE_REF:-main}" \
                        --version "${TEMPLATE_VERSION:?TEMPLATE_VERSION required}" \
                        --commit "${TEMPLATE_COMMIT:?TEMPLATE_COMMIT required}"

# === template (added Phase 09) ===
template-update *args:
    awiki template update {{args}}

template-status:
    awiki template status

template-gc:
    awiki template gc

bootstrap-step id:
    awiki bootstrap-step {{id}}

template-retrofit:
    awiki template retrofit
