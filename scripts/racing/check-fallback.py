#!/usr/bin/env python3
"""Compatibility entrypoint for the current chase-perspective acceptance."""
import os, sys
from pathlib import Path
script = Path(__file__).with_name('check-remake.py')
os.execv(sys.executable, [sys.executable, '-B', str(script), '--skip-maps', '--fallback', '--tracks', 'metro', *sys.argv[1:]])
