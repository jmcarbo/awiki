#!/usr/bin/env python3
"""Read sorted slug list from stdin; print first 6 hex chars of sha256."""
import hashlib
import sys

sys.stdout.write(hashlib.sha256(sys.stdin.read().encode("utf-8")).hexdigest()[:6])
