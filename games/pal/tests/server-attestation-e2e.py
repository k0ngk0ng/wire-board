import json
import os
from pathlib import Path
import subprocess
import tempfile

ROOT = Path(__file__).resolve().parents[3]
runtime = ROOT / '.local/pal/runtime-candidates/audio/original'
saved = {name: (runtime / name).read_bytes() for name in ['rules.wasm', 'build.json']}
release = ROOT / 'games/pal/assets/server-release.json'
published = release.read_bytes()
with tempfile.TemporaryDirectory(prefix='source-failure-', dir=ROOT / '.local/pal') as temporary:
    folder = Path(temporary)
    source = folder / 'main.go'
    source.write_text('package main\nimport "os"\nfunc main(){p,_:=os.Executable();os.WriteFile(p+".used",[]byte("called"),0600);for i,a:=range os.Args{if a=="-o"&&i+1<len(os.Args){os.WriteFile(os.Args[i+1],[]byte("partial failed optimization"),0600)}};os.Exit(23)}\n')
    tool = folder / 'wasm-opt'
    subprocess.run(['go', 'build', '-o', str(tool), str(source)], cwd=ROOT, check=True)
    restored = False
    try:
        result = subprocess.run(['python3', '-B', 'games/pal/assets/publish-server.py'], cwd=ROOT, env=dict(os.environ, PAL_WASM_OPT=str(tool)), capture_output=True, text=True)
        assert result.returncode != 0 and Path(str(tool) + '.used').exists(), 'source compiler failure was not exercised'
        restored = all((runtime / name).read_bytes() == data for name, data in saved.items())
        assert restored, 'failed source rebuild did not restore the accepted core and build receipt'
        assert release.read_bytes() == published, 'failed source reproduction changed publication metadata'
    finally:
        for name, data in saved.items():
            (runtime / name).write_bytes(data)
report = {'passed': True, 'scope': 'actual publication CLI with a failing external optimizer that partially overwrites the accepted original core; accepted program/build receipt and publication metadata remain exact; no remote writes requested', 'partialOptimizerFailure': True, 'acceptedCoreRestored': restored, 'remoteWrites': 0}
(ROOT / '.local/pal/verification/server-attestation-e2e.json').write_text(json.dumps(report, indent=2) + '\n')
print(json.dumps(report))
