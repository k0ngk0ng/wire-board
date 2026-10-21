"""Verify native application embedding excludes media/Wasm and pins verified OSS bytes.
Browser execution and load timing are covered by accept_loading.py separately.
"""
from pathlib import Path
from urllib.request import urlopen, Request
from urllib.error import HTTPError, URLError
import hashlib, json, os, socket, subprocess, time, uuid, shutil

ROOT = Path(__file__).resolve().parents[2]
OUT = ROOT / '.local/basketball/runtime-package'
OUT.mkdir(parents=True, exist_ok=True)
env = dict(os.environ)
checks = []
public_origin = os.environ.get("BASKETBALL_PUBLIC_ORIGIN", "https://bga.ichenj.com")
app = None
data = OUT / ('data-' + uuid.uuid4().hex[:8])

def check(name, value):
    assert value, name
    checks.append(name)

try:
    for command, log in [
        (['npm', '--prefix', 'web', 'run', 'build'], 'web-build.log'),
        (['node', 'games/basketball/build/verify-runtime.mjs', 'web/dist'], 'verify.log'),
        (['node', 'games/basketball/build/verify-assets.mjs', 'web/dist'], 'media-exclusion.log'),
    ]:
        with (OUT / log).open('w') as output:
            subprocess.run(command, cwd=ROOT, env=env, stdout=output, stderr=subprocess.STDOUT, check=True)
    subprocess.run(['go', 'build', '-trimpath', '-buildvcs=false', '-o', str(OUT / 'wire-board'), '.'], cwd=ROOT, env=env, check=True)
    with socket.socket() as sock:
        sock.bind(('127.0.0.1', 0))
        port = sock.getsockname()[1]
    base = f'http://127.0.0.1:{port}'
    data.mkdir()
    with (OUT / 'server.log').open('w') as output:
        app = subprocess.Popen([str(OUT / 'wire-board')], cwd=ROOT, env=dict(env, ADDR=f'127.0.0.1:{port}', DATA_DIR=str(data), INVITE_CODE='runtime-package-test', ADMIN_USERNAME='构建验收', ADMIN_PASSWORD='runtime-package-test-only'), stdout=output, stderr=subprocess.STDOUT)
        for _ in range(100):
            if app.poll() is not None:
                raise RuntimeError('Packaged server stopped')
            try:
                with urlopen(base + '/healthz', timeout=1) as response:
                    if response.status == 200:
                        break
            except (URLError, TimeoutError):
                time.sleep(.05)
        else:
            raise RuntimeError('Packaged server did not become healthy')
        runtime = ROOT / 'web/dist/basketball/v1/runtime'
        manifest = json.loads((runtime / 'build.json').read_text())
        check('runtime directory contains only descriptor and JS loader', sorted(p.name for p in runtime.iterdir()) == ['build.json', 'wasm_exec.js'])
        for name in ['build.json', 'wasm_exec.js']:
            with urlopen(base + '/basketball/v1/runtime/' + name) as response:
                check('embedded bytes match verified build: ' + name, response.read() == (runtime / name).read_bytes())
                policy = response.headers.get('Content-Security-Policy', '')
                check('production CSP allows Wasm and immutable OSS: ' + name, "'wasm-unsafe-eval'" in policy and "connect-src 'self' https://cdn.ichenj.com;" in policy and "'unsafe-eval'" not in policy)
        for route in ['/basketball/v1/runtime/basketball.wasm', '/basketball/v1/audio/music-066-loop.mp3', '/basketball/v1/graphics/raster/manifest.json', '/basketball/v1/selection/manifest.json', '/games/basketball/engine/runtime.go', '/.local/basketball/rom.nes']:
            try:
                with urlopen(base + route, timeout=3) as response:
                    status = response.status
            except HTTPError as error:
                status = error.code
            check('application does not serve excluded file: ' + route, status == 404)
        pin = manifest['wasm']
        with urlopen(Request(pin['url'], headers={'Origin': public_origin}), timeout=30) as response:
            content = response.read()
            check('OSS Wasm matches source-bound hash and length', hashlib.sha256(content).hexdigest() == pin['sha256'] and len(content) == pin['bytes'])
            check('OSS MIME and production CORS permit Wasm fetch', response.headers.get_content_type() == 'application/wasm' and response.headers.get('Access-Control-Allow-Origin') in ['*', public_origin])
        (OUT / 'result.json').write_text(json.dumps(dict(status='passed', checks=checks, runtime=pin, applicationBytes=(OUT / 'wire-board').stat().st_size, scope='native application embedding and remote object integrity/MIME/CORS; browser execution is covered separately'), ensure_ascii=False, indent=2) + '\n')
        print(json.dumps(dict(status='passed', checks=len(checks))))
except Exception as error:
    (OUT / 'result.json').write_text(json.dumps(dict(status='failed', checks=checks, error=str(error)), ensure_ascii=False, indent=2) + '\n')
    raise
finally:
    if app:
        app.terminate()
        try:
            app.wait(timeout=10)
        except subprocess.TimeoutExpired:
            app.kill()
            app.wait()
    if data.exists():
        shutil.rmtree(data)
    (OUT / 'wire-board').unlink(missing_ok=True)
