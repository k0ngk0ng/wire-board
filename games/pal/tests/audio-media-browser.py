import http.server
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import threading

ROOT = Path(__file__).resolve().parents[3]
OUT = ROOT / '.local/pal/audio-media-browser'
MEDIA = ROOT / '.local/pal/audio-media-e2e'
ENV = dict(os.environ, AGENT_BROWSER_SOCKET_DIR=str(ROOT / '.local/pal-ab'), AGENT_BROWSER_PROFILE=str(OUT / 'profile'), AGENT_BROWSER_SCREENSHOT_DIR=str(OUT), TMPDIR=str(ROOT / '.local/tmp'), AGENT_BROWSER_NO_VERSION_CHECK='1', AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter')


def browser(session, *args):
    result = subprocess.run(['agent-browser', '--session', session, '--json', *map(str, args)], cwd=ROOT, env=ENV, check=True, capture_output=True, text=True)
    data = json.loads(result.stdout)
    assert data['success'], data
    return data['data']


class Handler(http.server.SimpleHTTPRequestHandler):
    def send_head(self):
        requested = self.headers.get('Range')
        self.remaining = None
        if not requested:
            return super().send_head()
        path = Path(self.translate_path(self.path))
        if not path.is_file():
            self.send_error(404)
            return None
        match = re.fullmatch(r'bytes=(\d*)-(\d*)', requested)
        size = path.stat().st_size
        if not match or not any(match.groups()):
            self.send_error(416)
            return None
        first = int(match[1]) if match[1] else max(0, size - int(match[2]))
        last = min(size - 1, int(match[2])) if match[1] and match[2] else size - 1
        if first > last or first >= size:
            self.send_error(416)
            return None
        file = path.open('rb')
        file.seek(first)
        self.remaining = last - first + 1
        self.send_response(206)
        self.send_header('Content-Type', self.guess_type(str(path)))
        self.send_header('Content-Length', str(self.remaining))
        self.send_header('Content-Range', f'bytes {first}-{last}/{size}')
        self.send_header('Accept-Ranges', 'bytes')
        self.end_headers()
        return file

    def copyfile(self, source, target):
        if self.remaining is None:
            return super().copyfile(source, target)
        remaining = self.remaining
        while remaining:
            data = source.read(min(remaining, 65536))
            if not data:
                break
            target.write(data)
            remaining -= len(data)

    def translate_path(self, path):
        path = path.split('?', 1)[0]
        if path.startswith('/pal/audio-v1/'):
            base, relative = MEDIA, path[len('/pal/audio-v1/'):]
        else:
            base, relative = OUT, path.lstrip('/') or 'index.html'
        target = (base / relative).resolve()
        return str(target if target.is_relative_to(base) else OUT / 'not-found')

    def log_message(self, *args):
        pass

    def handle(self):
        try:
            super().handle()
        except (ConnectionResetError, BrokenPipeError):
            pass


def main():
    assert (MEDIA / 'index.json').is_file(), 'run audio-media-e2e.py first'
    subprocess.run(['node', 'games/pal/tests/audio-media-fixture.mjs'], cwd=ROOT, env=ENV, check=True)
    session = subprocess.check_output(['agent-browser', 'session', 'id', '--scope', 'worktree', '--prefix', 'pal-audio-media'], cwd=ROOT, env=ENV, text=True).strip()
    server = http.server.ThreadingHTTPServer(('127.0.0.1', 0), Handler)
    threading.Thread(target=server.serve_forever, daemon=True).start()
    report = {'passed': False, 'scope': 'all exported PAL sound/music/CD sources through actual platform native GameAudio on Chromium desktop and emulated phone; metadata, playing, duration, pause/resume, mute, seek and stop; not a native phone, listening judgement, mainline audio integration or CDN acceptance', 'cases': []}
    device = None
    url = f'http://127.0.0.1:{server.server_port}'
    try:
        browser(session, 'open', url)
        for width, height in [(1280, 800), (320, 820)]:
            browser(session, 'set', 'viewport', width, height)
            if width == 320:
                device = subprocess.Popen(['node', 'scripts/browser_device.mjs', browser(session, 'get', 'cdp-url')['cdpUrl'], url, str(width), str(height), '--keep'], cwd=ROOT, env=ENV, text=True, stdout=subprocess.PIPE)
                assert device.stdout.readline().strip() == 'ready'
            browser(session, 'open', url)
            browser(session, 'wait', '--fn', 'window.audioMediaReady===true')
            snapshot = browser(session, 'snapshot', '-i')
            assert snapshot
            browser(session, 'find', 'role', 'button', 'click', '--name', '开始验证')
            for unused in range(12):
                try:
                    browser(session, 'wait', '--fn', 'window.audioMediaDone===true', '--timeout', '15000')
                    break
                except subprocess.CalledProcessError:
                    assert browser(session, 'eval', 'window.audioMediaFailure || null')['result'] is None
            result = browser(session, 'eval', 'window.audioMediaResult')['result']
            assert result and result['passed'] and len(result['sources']) == 458, result
            assert all(row['played'] and row['advanced'] for row in result['sources'])
            assert result['touch'] == (width == 320)
            assert result['lifecycle']['passed'] and not result['overflow']
            browser(session, 'screenshot', OUT / f'audio-{width}.png')
            report['cases'].append(result)
        report['passed'] = True
    finally:
        if device:
            device.terminate()
            device.wait(timeout=5)
        try:
            browser(session, 'close')
        except Exception:
            pass
        server.shutdown()
        server.server_close()
        shutil.rmtree(OUT / 'profile', ignore_errors=True)
        (OUT / 'result.json').write_text(json.dumps(report, ensure_ascii=False, indent=2) + '\n')
    print(json.dumps({'passed': report['passed'], 'sources': sum(len(row['sources']) for row in report['cases'])}))


if __name__ == '__main__':
    main()
