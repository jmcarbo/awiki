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
    qmd search "{{query}}"

log action *message:
    bash scripts/log-append.sh {{action}} {{message}}

rename old new:
    bash scripts/rename.sh {{old}} {{new}}

delete slug:
    bash scripts/delete-page.sh {{slug}}

# === hugo ===
serve:
    bash scripts/serve.sh

build:
    bash scripts/build.sh --full

# === bootstrap / setup ===
init:
    @echo "Open agent. Say: 'init wiki'. Agent reads BOOTSTRAP.md."

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
