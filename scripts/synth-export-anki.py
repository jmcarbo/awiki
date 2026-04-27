#!/usr/bin/env python3
"""Convert a Flashcards block to an Anki .apkg file.

Args (positional, after a `--` terminator from the bash wrapper):
  1. deck slug (used as deck name + filename basename)
  2. path to a temp file containing only the Flashcards block
  3. output .apkg path

The temp file is expected to contain Q:/A: blocks separated by --- lines.
We deliberately accept the cards-block as a file path (NOT as an argv string)
so that user-supplied adversarial content (smart quotes, control chars,
shell metacharacters) cannot break out into the parent shell or into Python's
own argv parsing.
"""

import sys
import re
import hashlib

import genanki  # required; the bash wrapper checks before invoking us.


def parse_cards(path):
    with open(path, encoding="utf-8") as f:
        body = f.read()
    blocks = [b.strip() for b in body.split("\n---\n")]
    cards = []
    for blk in blocks:
        if not blk:
            continue
        m_q = re.search(r"^Q:\s*(.+?)$", blk, re.M)
        m_a = re.search(r"^A:\s*(.+?)$", blk, re.M)
        if m_q and m_a:
            cards.append((m_q.group(1).strip(), m_a.group(1).strip()))
    return cards


def stable_id(slug, suffix=""):
    h = hashlib.sha256((slug + suffix).encode("utf-8")).hexdigest()
    # genanki wants positive int < 2^31.
    return int(h[:8], 16) & 0x7FFFFFFF


def main():
    # Drop a leading -- terminator if the wrapper passed one.
    argv = [a for a in sys.argv[1:] if a != "--"]
    if len(argv) != 3:
        print("usage: synth-export-anki.py <slug> <cards-tmp> <out.apkg>", file=sys.stderr)
        sys.exit(1)
    slug, cards_path, out_path = argv

    cards = parse_cards(cards_path)
    if not cards:
        print(f"no cards parsed from {cards_path}; skipping export", file=sys.stderr)
        sys.exit(0)

    model = genanki.Model(
        stable_id(slug, ":model"),
        f"awiki-{slug}-model",
        fields=[{"name": "Question"}, {"name": "Answer"}],
        templates=[
            {
                "name": "Card 1",
                "qfmt": "{{Question}}",
                "afmt": '{{FrontSide}}<hr id="answer">{{Answer}}',
            }
        ],
    )

    deck = genanki.Deck(stable_id(slug, ":deck"), f"awiki :: {slug}")
    for q, a in cards:
        deck.add_note(genanki.Note(model=model, fields=[q, a]))

    genanki.Package(deck).write_to_file(out_path)


if __name__ == "__main__":
    main()
