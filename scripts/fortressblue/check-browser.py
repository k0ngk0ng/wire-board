#!/usr/bin/env python3
"""Compatibility entry point for the refactored real-browser acceptance."""
from pathlib import Path
import runpy
runpy.run_path(str(Path(__file__).with_name('check-refactor.py')), run_name='__main__')
