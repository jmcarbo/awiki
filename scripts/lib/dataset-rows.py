#!/usr/bin/env python3
"""Row counter / validator / sampler for awiki dataset files.

Subcommands:
  count    --format=<fmt> --file=<path>            -> row count to stdout
  validate --format=<fmt> --file=<path> --schema=<json-file>
                                                    -> exit 0 / non-zero
  sample   --format=<fmt> --file=<path> --n=<int>  -> JSON array to stdout
"""
import argparse
import csv
import json
import sys
from pathlib import Path


_BAD = object()


def _open_csv(path, delimiter):
    with open(path, newline="", encoding="utf-8") as f:
        reader = csv.DictReader(f, delimiter=delimiter)
        for row in reader:
            yield row


def _load(path, fmt):
    if fmt == "csv":
        return list(_open_csv(path, ","))
    if fmt == "tsv":
        return list(_open_csv(path, "\t"))
    if fmt == "dsv":
        # Heuristic: try first-line delimiter detect (',', ';', '|', '\t').
        first = Path(path).read_text(encoding="utf-8").splitlines()[0]
        for d in (",", ";", "|", "\t"):
            if d in first:
                return list(_open_csv(path, d))
        return list(_open_csv(path, ","))
    if fmt == "json":
        data = json.loads(Path(path).read_text(encoding="utf-8"))
        if isinstance(data, list):
            return data
        # Object form: treat as 1 row.
        return [data]
    if fmt == "topojson":
        # topojson is a single GeoJSON-shaped object; row count is 1.
        return [json.loads(Path(path).read_text(encoding="utf-8"))]
    raise SystemExit(f"unknown format: {fmt}")


def _coerce(value, ty):
    if value is None or value == "":
        return None  # null is always valid for type-check purposes.
    if ty == "string":
        return str(value)
    if ty == "integer":
        if isinstance(value, bool):
            return _BAD  # bool subclasses int; reject.
        if isinstance(value, int):
            return value
        s = str(value).strip()
        if s.startswith("-"):
            digits = s[1:]
        else:
            digits = s
        if not digits.isdigit():
            return _BAD
        return int(s)
    if ty == "number":
        try:
            return float(value)
        except (TypeError, ValueError):
            return _BAD
    if ty == "boolean":
        s = str(value).strip().lower()
        if s in ("true", "1"):
            return True
        if s in ("false", "0"):
            return False
        return _BAD
    return _BAD


def _validate(rows, schema):
    errs = []
    for i, row in enumerate(rows, start=1):
        for col in schema:
            name = col["name"]
            ty = col["type"]
            if name not in row:
                errs.append(f"row={i} col={name} missing")
                continue
            coerced = _coerce(row[name], ty)
            if coerced is _BAD:
                errs.append(f"row={i} col={name} want={ty} got={row[name]!r}")
    return errs


def main():
    p = argparse.ArgumentParser()
    sub = p.add_subparsers(dest="cmd", required=True)
    for name in ("count", "validate", "sample"):
        sp = sub.add_parser(name)
        sp.add_argument("--format", required=True)
        sp.add_argument("--file", required=True)
        if name == "validate":
            sp.add_argument("--schema", required=True)
        if name == "sample":
            sp.add_argument("--n", type=int, default=20)
    args = p.parse_args()

    rows = _load(args.file, args.format)

    if args.cmd == "count":
        print(len(rows))
        return 0
    if args.cmd == "validate":
        schema = json.loads(Path(args.schema).read_text(encoding="utf-8"))
        errs = _validate(rows, schema)
        if errs:
            for e in errs:
                print(e)
            return 1
        return 0
    if args.cmd == "sample":
        out = rows[: args.n]
        # Convert non-serializable to string fallback.
        print(json.dumps(out, default=str))
        return 0


if __name__ == "__main__":
    sys.exit(main() or 0)
