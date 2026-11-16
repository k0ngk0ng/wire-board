import argparse
import json
import os
import re
from pathlib import Path
import shutil
import subprocess
import time
import uuid

parser = argparse.ArgumentParser()
parser.add_argument('--url')
parser.add_argument('--out', required=True)
args = parser.parse_args()
root = Path(__file__).resolve().parents[1]
out = (root / args.out).resolve()
assert out.is_relative_to(root) and not out.exists()
out.mkdir(parents=True)
temporary = out / 'temporary'
temporary.mkdir()
common = Path(subprocess.check_output(['git', 'rev-parse', '--path-format=absolute', '--git-common-dir'], cwd=root, text=True).strip()).parent
sockets = common / '.s' / ('r' + uuid.uuid4().hex[:5])
sockets.mkdir(parents=True)
env = dict(os.environ, TMPDIR=str(temporary), AGENT_BROWSER_SESSION=sockets.name,
           AGENT_BROWSER_SOCKET_DIR=str(sockets), AGENT_BROWSER_PROFILE=str(temporary / 'profile'),
           AGENT_BROWSER_ARGS='--disable-breakpad,--disable-crash-reporter,--disable-crashpad,--crash-dumps-dir=' + str(temporary / 'crashpad'),
           GOCACHE=str(root / '.local/go-build'), GOMODCACHE=str(root / '.local/go-mod'),
           GOTMPDIR=str(root / '.local/tmp'), GOTELEMETRY='off')
report = {'passed': False, 'checks': []}
fixture = None
opened = False

def browser(*arguments):
    result = subprocess.run(['agent-browser', *arguments, '--json'], cwd=root, env=env,
                            capture_output=True, text=True, timeout=45)
    assert result.returncode == 0, result.stdout + result.stderr
    value = json.loads(result.stdout)
    assert value['success'], value
    return value['data']

def js(code):
    return browser('eval', code)['result']

def wait(condition, timeout=30):
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        value = condition()
        if value:
            return value
        time.sleep(.2)
    raise AssertionError('condition timed out')

def check(name, **measurements):
    report['checks'].append({'name': name, **measurements})
    print('PASS', name, flush=True)

def screenshot(name):
    browser('screenshot', str(out / (name + '.png')))

def fit():
    result = js('({width:innerWidth,scroll:document.documentElement.scrollWidth})')
    assert result['scroll'] <= result['width'], result
    return result

try:
    if not args.url:
        log = (out / 'fixture.log').open('w')
        fixture = subprocess.Popen(['go', 'test', './internal/server', '-run', '^TestReleaseHistoryBrowser$', '-count=1', '-timeout=10m', '-v'],
                                   cwd=root, env=dict(env, WIRE_BOARD_RELEASE_HISTORY_BROWSER=str(temporary)), stdout=log, stderr=log)
        wait(lambda: (temporary / 'url').exists() or fixture.poll() is not None, 120)
        assert fixture.poll() is None, (out / 'fixture.log').read_text()
        args.url = (temporary / 'url').read_text()
    base = args.url.rstrip('/')
    browser('open', base + '/releases')
    opened = True
    browser('wait', '[data-release-list]')
    history = json.loads((root / 'web/src/releases/history.json').read_text())
    newest, oldest = history[0], history[-1]
    assert js('document.querySelector("h1").textContent') == '更新日志'
    count = js('document.querySelectorAll("[data-release-list] a").length')
    assert 0 < count < len(history)
    browser('click', 'button[data-release-more]')
    assert js('document.querySelectorAll("[data-release-list] a").length') > count
    check('public version list and pagination', archivedVersions=len(history))
    browser('fill', 'input[type=search]', oldest['version'])
    browser('click', '[data-release-list] a[href="/releases/' + oldest['version'] + '"]')
    browser('wait', '[data-release-detail]')
    assert js('document.querySelector("[data-release-detail]").dataset.releaseDetail') == oldest['version']
    assert js('document.querySelector(".release-body").textContent.trim().length') > 0
    browser('reload')
    browser('wait', '[data-release-detail]')
    assert js('document.querySelector("[data-release-detail]").dataset.releaseDetail') == oldest['version']
    check('oldest version detail supports direct links and reload')
    browser('back')
    browser('wait', '[data-release-list]')
    browser('forward')
    browser('wait', '[data-release-detail]')
    check('native browser history preserves version navigation')
    browser('open', base + '/releases')
    browser('wait', '[data-release-list]')
    browser('fill', 'input[type=search]', 'no-such-release-999')
    browser('wait', '[data-release-empty]')
    browser('click', '[data-release-clear]')
    browser('wait', '[data-release-list] a')
    check('search empty state can be cleared')
    browser('open', base + '/releases/v999.999.999')
    browser('wait', '[data-release-missing]')
    assert not js('!!document.querySelector("[data-release-detail]")')
    browser('click', 'a[href="/releases"]')
    browser('wait', '[data-release-list]')
    check('unknown versions show an explicit recoverable state')
    browser('click', '[data-release-list] a')
    browser('wait', '[data-release-detail]')
    assert js('document.querySelector("[data-release-detail]").dataset.releaseDetail') == newest['version']
    for width in (1280, 320):
        browser('set', 'viewport', str(width), '900')
        measurements = fit()
        styles = js('(()=>{const e=document.querySelector(".release-body"),s=getComputedStyle(e);return {color:s.color,background:getComputedStyle(document.querySelector(".release-detail")).backgroundColor,fontSize:s.fontSize}})()')
        contrast = js(r"""(()=>{const luminance=c=>{const v=c.match(/[\d.]+/g).slice(0,3).map(Number).map(x=>{x/=255;return x<=.04045?x/12.92:((x+.055)/1.055)**2.4});return v[0]*.2126+v[1]*.7152+v[2]*.0722};return [...document.querySelectorAll('.release-body li,.release-detail time,.release-title,.release-detail h1,.release-header a')].map(e=>{let p=e,bg;while(p){bg=getComputedStyle(p).backgroundColor;if(bg!=='rgba(0, 0, 0, 0)')break;p=p.parentElement}const foreground=luminance(getComputedStyle(e).color),background=luminance(bg);return {text:e.textContent.slice(0,30),ratio:(Math.max(foreground,background)+.05)/(Math.min(foreground,background)+.05)}})})()""")
        assert contrast and all(value['ratio'] >= 4.5 for value in contrast), contrast
        screenshot('detail-' + str(width))
        check('version detail layout ' + str(width), **measurements, styles=styles, contrast=contrast)
    browser('open', base + '/releases/v1.0.1')
    browser('wait', '[data-release-detail]')
    assert js('!!document.querySelector(' + json.dumps('.release-body a[href^="https://github.com/"]') + ')')
    fit()
    screenshot('historical-320')
    check('historical Markdown and long compare links render at 320px')
    browser('open', base + '/releases')
    browser('wait', '[data-release-list]')
    placeholder = js('(()=>{const e=document.querySelector(".release-search input");return {color:getComputedStyle(e,"::placeholder").color,background:getComputedStyle(e).backgroundColor,opacity:getComputedStyle(e,"::placeholder").opacity}})()')
    def luminance(color):
        values = [float(v) / 255 for v in re.findall(r'[\d.]+', color)[:3]]
        values = [v / 12.92 if v <= .04045 else ((v + .055) / 1.055) ** 2.4 for v in values]
        return sum(v * weight for v, weight in zip(values, (.2126, .7152, .0722)))
    foreground, background = luminance(placeholder['color']), luminance(placeholder['background'])
    ratio = (max(foreground, background) + .05) / (min(foreground, background) + .05)
    assert ratio >= 4.5 and float(placeholder['opacity']) == 1, placeholder
    screenshot('list-320')
    fit()
    check('version list fits 320px with readable search hint', placeholderContrast=ratio, **placeholder)
    browser('open', base + '/')
    browser('wait', 'a[data-release-entry]')
    assert js('document.querySelector("a[data-release-entry]").getAttribute("href")') == '/releases'
    check('logged-out website exposes update history')
    if fixture:
        browser('find', 'role', 'button', 'click', '--name', '创建账号', '--exact')
        browser('fill', 'input[name=name]', '版本验收玩家')
        browser('fill', 'input[name=password]', 'release-history-test-password')
        browser('fill', 'input[name=invite]', 'release-history-test')
        browser('find', 'role', 'button', 'click', '--name', '创建账号，入座')
        browser('wait', '.lobby-version a')
        for selector in ('.app-version', '.lobby-version a'):
            assert js('(()=>{const a=document.querySelector(' + json.dumps(selector) + ');return a.href.endsWith("/releases/v1.59.0")&&a.target==="_blank"&&a.rel.includes("noopener")})()')
        screenshot('lobby-320')
        check('header and lobby versions open details in a separate tab without leaving play')
    report['passed'] = True
finally:
    if opened:
        if not report['passed']:
            try:
                screenshot('failed-page')
                report['page'] = browser('snapshot')
                report['errors'] = browser('errors')
            except Exception:
                pass
        try:
            browser('close')
        except Exception:
            pass
    if fixture:
        (temporary / 'done').write_text('passed' if report['passed'] else 'failed')
        try:
            fixture.wait(timeout=15)
        except subprocess.TimeoutExpired:
            fixture.terminate()
            fixture.wait(timeout=10)
        report['fixtureExitCode'] = fixture.returncode
        report['passed'] = report['passed'] and fixture.returncode == 0
    shutil.rmtree(temporary)
    shutil.rmtree(sockets, ignore_errors=True)
    (out / 'report.json').write_text(json.dumps(report, ensure_ascii=False, indent=2) + '\n')
assert report['passed'], report
