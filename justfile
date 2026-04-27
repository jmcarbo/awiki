set shell := ["bash", "-uc"]

default:
    @just --list

# === ingest ===
ingest path:
    bash scripts/ingest.sh {{path}}

ingest-batch-list:
    @find raw/inbox/batch -type f | sort

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

# === task layer (phase 16) ===
task-init:
    bash scripts/task-init.sh

capture *text:
    bash scripts/capture.sh -- {{text}}

# Stubs land in later phases:
# scan:                                  # phase 17
#     bash scripts/action-scan.sh
# agenda:                                # phase 17
#     bash scripts/action-scan.sh
#     bash scripts/agenda.sh
# triage:                                # phase 18
#     bash scripts/triage.sh --interactive
# review:                                # phase 19
#     bash scripts/agenda.sh
#     bash scripts/lint.sh
#     bash scripts/review-status.sh
