#!/usr/bin/env python3
"""Compatibility entrypoint for the map-first Conquest browser acceptance.

The previous form-based UI was removed. The canonical acceptance now lives in
accept_map_ux.py, including native gestures, shortcut orders and shared controls.
"""
import os
from pathlib import Path
import runpy
import sys

if os.environ.get("CONQUEST_SKIP_BUILD") == "1" and "--skip-build" not in sys.argv:
    sys.argv.append("--skip-build")
if os.environ.get("CONQUEST_BASE_URL") and "--base-url" not in sys.argv:
    sys.argv.extend(["--base-url", os.environ["CONQUEST_BASE_URL"]])
runpy.run_path(str(Path(__file__).with_name("accept_map_ux.py")), run_name="__main__")
