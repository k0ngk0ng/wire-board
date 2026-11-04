#!/usr/bin/env python3
"""Reproduce the observed battle RAM IRQ without altering reference state."""
import argparse
import json
import os
from pathlib import Path
import subprocess
import sys

sys.dont_write_bytecode = True
from reference import ROOT, ReferenceSession, buttons_at, sha


FRAME_COUNT = 2400
CORE_SHA256 = '1d08b724cf975de17af146f9ba4292946a609b26d03da3d0d9414c90983095da'
IRQ_SHA256 = '5b0cea90c181d6ff10b647f718b7447be6930d907925d3301b0b40344afbeb25'
CANONICAL = ROOT/'games/yuyuhakusho/engine/testdata/world-frame-irq.json'


def inside(path):
    result = path.resolve()
    if not result.is_relative_to(ROOT):
        raise ValueError('output must remain inside worktree')
    return result


def disassemble(out, irq, ghidra, java_home):
    """Use a private Ghidra project; no existing analysis database is touched."""
    tool = ghidra.resolve()/'support/analyzeHeadless'
    if not tool.is_file() or not (java_home.resolve()/'bin/java').is_file():
        raise ValueError('Ghidra and JDK must already be installed')
    # Ghidra rejects a project locator with any dot-prefixed path element,
    # even when that component is merely the worktree's .local directory.
    project = inside(ROOT/'docs/research/md-yuyuhakusho/static-work'/('frame-irq-'+sha(str(out).encode())[:12]))
    temporary = inside(out/'ghidra-tmp')
    for path in [project, temporary, temporary/'java-home', temporary/'config', temporary/'cache']:
        path.mkdir(parents=True, exist_ok=True)
    env = dict(os.environ)
    env['JAVA_HOME'] = str(java_home.resolve())
    env['PATH'] = str(java_home.resolve()/'bin')+os.pathsep+env.get('PATH', '')
    env['TMPDIR'] = str(temporary)
    # analyzeHeadless's launcher and Java both receive local scratch paths.
    options = {'user.home': temporary/'java-home', 'java.io.tmpdir': temporary,
               'application.settingsdir': temporary/'config', 'application.cachedir': temporary/'cache',
               'application.tempdir': temporary}
    env['JAVA_TOOL_OPTIONS'] = ' '.join('"-D'+name+'='+str(path)+'"' for name, path in options.items())
    output = inside(out/'world-frame-irq.txt')
    arguments = [str(tool), str(project), 'world-frame-irq-replay',
                 '-import', str(ROOT/'.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin'),
                 '-loader', 'BinaryLoader', '-loader-baseAddr', '0', '-processor', '68000:BE:32:default',
                 '-noanalysis', '-scriptPath', str(ROOT/'scripts/yuyuhakusho/static'),
                 '-postScript', 'MdRamFrameProbe.java', str(ROOT), str(irq), str(output),
                 '-deleteProject', '-log', str(out/'ghidra.log'), '-scriptlog', str(out/'ghidra-script.log')]
    with (out/'ghidra-console.log').open('w') as console:
        subprocess.run(arguments, env=env, stdout=console, stderr=subprocess.STDOUT, check=True)
    if not any(project.iterdir()):
        project.rmdir()
    if not output.is_file():
        raise RuntimeError('Ghidra did not produce the requested listing; inspect local logs')
    canonical = ROOT/'docs/research/md-yuyuhakusho/static/world-frame-irq.txt'
    listing = output.read_bytes()
    if listing != canonical.read_bytes():
        raise ValueError('bounded Ghidra listing differs from the saved instruction evidence')
    return {'route': 'direct-ghidra-headless; REA raw 68000 unsupported',
            'language': '68000:BE:32:default', 'listing_sha256': sha(listing),
            'matches_saved_listing': True,
            'probe_sha256': sha((ROOT/'scripts/yuyuhakusho/static/MdRamFrameProbe.java').read_bytes())}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--out', type=Path, required=True)
    parser.add_argument('--disassemble', action='store_true', help='also run the installed Ghidra in a private project')
    parser.add_argument('--ghidra-install', type=Path, default=Path('/Users/jason/Sources/github.com/k0ngk0ng/wire-board/.local/nes-basket/tools/ghidra_12.1.4_PUBLIC'))
    parser.add_argument('--java-home', type=Path, default=Path('/opt/homebrew/Cellar/openjdk/26.0.1/libexec/openjdk.jdk/Contents/Home'))
    args = parser.parse_args()
    out = inside(args.out)
    original = ROOT/'.local/md-yuyuhakusho/world-frame-irq'
    if out == original or out.is_relative_to(original):
        raise ValueError('preserve the original IRQ evidence; select a new output directory')
    out.mkdir(parents=True, exist_ok=True)
    input_path = ROOT/'docs/research/md-yuyuhakusho/inputs.json'
    baseline_path = ROOT/'docs/research/md-yuyuhakusho/frame-hashes.json'
    events = json.loads(input_path.read_text())
    expected = json.loads(baseline_path.read_text())
    if len(expected) < FRAME_COUNT:
        raise ValueError('pinned baseline does not contain 2400 video steps')
    hashes = []
    with ReferenceSession(ROOT, out/'reference') as ref:
        if ref.identity['core_sha256'] != CORE_SHA256:
            raise ValueError('reference core differs from the pinned original')
        for frame in range(1, FRAME_COUNT+1):
            observed = ref.step(frame, buttons_at(events, frame))['reference']
            if observed != expected[frame-1]:
                raise ValueError('pinned RAM/video mismatch at video step '+str(frame))
            hashes.append(observed)
        ram = ref.ram_cpu()
        irq = ram[0xf000:0xf140]
        identity = ref.identity
    if len(irq) != 320 or sha(irq) != IRQ_SHA256:
        raise ValueError('observed CPU-order IRQ differs from the original evidence')
    if (original/'irq.bin').is_file() and irq != (original/'irq.bin').read_bytes():
        raise ValueError('the preserved original IRQ bytes disagree')
    word = lambda offset: int.from_bytes(ram[offset:offset+2], 'big')
    fixture = {'frame': FRAME_COUNT, 'address': 'FFF000', 'bytes': len(irq),
               'byte_order': '68000 CPU big-endian', 'sha256': sha(irq), 'hex': irq.hex(),
               'context': {'SceneMode_F580': word(0xf580), 'SceneRequest_F582': word(0xf582),
                           'InterruptMode_F510': word(0xf510), 'VideoMode_F512': word(0xf512)}}
    checked_canonical = CANONICAL.exists()
    if checked_canonical and fixture != json.loads(CANONICAL.read_text()):
        raise ValueError('IRQ/context differs from the canonical fixture')
    raw = (json.dumps(fixture, indent=2)+'\n').encode()
    (out/'irq.bin').write_bytes(irq)
    (out/'world-frame-irq.json').write_bytes(raw)
    analysis = disassemble(out, out/'irq.bin', args.ghidra_install, args.java_home) if args.disassemble else None
    report = {'pass': True, 'scope': 'Cold-start observed RAM IRQ bytes with pinned per-video-step RAM/video comparison',
              'frames': FRAME_COUNT, 'all_match_pinned': True, 'identity': identity,
              'input_sha256': sha(input_path.read_bytes()), 'baseline_sha256': sha(baseline_path.read_bytes()),
              'capture_sha256': sha(Path(__file__).read_bytes()), 'fixture_sha256': sha(raw),
              'irq_bytes': len(irq), 'irq_sha256': sha(irq), 'canonical_fixture_checked': checked_canonical,
              'observed_frames_sha256': sha((json.dumps(hashes, separators=(',', ':'))+'\n').encode()),
              'context': fixture['context'], 'static_analysis': analysis,
              'supporting_static_sha256': {name: sha((ROOT/'docs/research/md-yuyuhakusho/static'/name).read_bytes())
                                           for name in ['world-frame-battle.txt', 'world-frame-scene.txt',
                                                        'world-frame-driver.txt', 'first-control.txt', 'input-and-tick.txt']},
              'limits': ['RAM bytes were observed at video step 2400; not every listed instruction is claimed executed',
                         'A video step can take the IRQ busy return without one logical tick',
                         'Static instruction listing uses direct Ghidra68000, not REA decompilation',
                         'No game RAM, rule state or input edges were injected']}
    (out/'world-frame-irq-validation.json').write_text(json.dumps(report, indent=2)+'\n')
    print(json.dumps(report, indent=2))


if __name__ == '__main__':
    main()
