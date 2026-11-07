"""Repeat the continuous world and presentation-boundary acceptance scope.

Uses a normal Go package build (including GOARCH filename constraints), keeps
all caches in this workspace, and binds the retained result to source hashes.
This is original-execution differential acceptance, not browser/campaign E2E.
"""
import hashlib, json, os, subprocess
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]

def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()

def source_files():
    files = sorted(p for p in (ROOT / 'games/contra/engine').rglob('*')
                   if p.is_file() and p.suffix in ('.go', '.json', '.gz'))
    files += [ROOT / 'docs/research/nes-contra/evidence' / name for name in
              ['world-complete-original.json.gz', 'world-paused-original.json.gz',
               'world-loss-original.json.gz', 'world-boss-original.json.gz',
               'world-start-original.json.gz', 'world-phase-original.json.gz',
               'world-continue-original.json.gz','world-quit-original.json.gz',
               'world-interlude-original.json.gz','terrain-work-stage8-full-v1.json.gz','terrain-work-guardian-world-v1.json.gz',
               'terrain-work-stage8-v2.json.gz','terrain-work-stage6-v2.json.gz',
               'terrain-work-guardian-cases-v5.json.gz','terrain-work-vertical-cases-v1.json.gz','terrain-work-indoor-cases-v2.json.gz']]
    files += [ROOT / 'scripts/contra' / name for name in
              ['world_verify.py', 'world_capture_complete.py', 'world_capture_pause.py', 'world_capture_campaign.py', 'world_capture_start.py','world_interlude_capture.py','world_interlude_verify.py']]
    return files

def main():
    output = ROOT / '.local/contra-world/checks'
    temporary = output / 'tmp'
    temporary.mkdir(parents=True, exist_ok=True)
    env = {**os.environ, 'GOCACHE': str(ROOT / '.local/go-build'),
           'GOMODCACHE': str(ROOT / '.local/go-mod'), 'GOTMPDIR': str(temporary),
           'TMPDIR': str(temporary), 'GOTOOLCHAIN': 'local'}
    files = source_files()
    before = {str(p.relative_to(ROOT)): digest(p) for p in files}
    command = ['go', 'test', './games/contra/engine', '-race', '-run',
               'TestOriginalContinuous.*World|TestWorldPresentation|TestPresentationMemory|TestNewSession|TestSession|TestDiscardedGraphics|TestGraphicsPrefix|TestTerrainWork|TestOriginalTerrainWorkActual|TestOriginalWeaponTerrainSupportContract', '-count=1', '-v']
    result = subprocess.run(command, cwd=ROOT, env=env, text=True,
                            stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
    log = output / 'active-world-latest.log'
    log.write_text(result.stdout)
    after = {str(p.relative_to(ROOT)): digest(p) for p in source_files()}
    changed = sorted(name for name in before.keys() | after.keys() if before.get(name) != after.get(name))
    manifest = {'command': command, 'exitCode': result.returncode,
                'sourceChangedDuringCheck': changed,
                'log': str(log.relative_to(ROOT)), 'logSHA256': digest(log), 'files': before,
                'scope': 'Continuous original differential: 600 ordinary, 180 pause/resume, 373 zero-reserve loss, 6472 initial-reserve30 Boss/exit, 1077 startup, 898 continue and 386 quit frames; startup/menu PPU compared. No forced Boss HP/death/outcome. Another 1460 independent standard-session frames exercise input, RNG, audio and restore. Terminal flush/retries and bounded graphics-prefix effects checked. TerrainWork contracts cover 2677 native prefixes/24310 operations and 50 Guardian consumers; they do not claim actual whole-world producer wiring. 800 stage-six combat phases verify solid-backed platform bullet collision. Not full eight-stage campaign or browser/network acceptance.'}
    target = output / 'active-world-latest.json'
    target.write_text(json.dumps(manifest, indent=2) + '\n')
    if result.returncode == 0 and not changed:
        retained = output / 'active-world-green.log'
        retained.write_text(result.stdout)
        manifest['log'] = str(retained.relative_to(ROOT))
        manifest['logSHA256'] = digest(retained)
        (ROOT / 'docs/research/nes-contra/evidence/active-world-verification.json').write_text(json.dumps(manifest, indent=2) + '\n')
    print(result.stdout, end='')
    if changed:
        print('Sources changed during check:', changed)
    raise SystemExit(result.returncode or bool(changed))

if __name__ == '__main__':
    main()
