"""Actual shared Preparation, original portraits and native pointer selection."""
from pathlib import Path
import argparse, hashlib, json, os, subprocess

ROOT = Path(__file__).resolve().parents[2]
p = argparse.ArgumentParser(description=__doc__)
p.add_argument('--url', required=True)
p.add_argument('--label', required=True)
p.add_argument('--red', action='store_true')
p.add_argument('--arenas', action='store_true')
args = p.parse_args()
assert args.url.startswith('http://127.0.0.1:')
OUT = ROOT/'games/yuyuhakusho/verification/review-polish'/args.label
OUT.mkdir(parents=True, exist_ok=False)
socket = (ROOT.parents[1] if ROOT.parent.name == 'worktrees' else ROOT)/'.local/au'
env = dict(os.environ, AGENT_BROWSER_SOCKET_DIR=str(socket), AGENT_BROWSER_SESSION='pic')
def b(*items):
    r = subprocess.run(['agent-browser', *items, '--json'], cwd=ROOT, env=env, text=True, capture_output=True, timeout=50)
    assert r.returncode == 0, r.stdout+r.stderr
    return json.loads(r.stdout)['data']
def js(code): return b('eval', code)['result']
report = {'pass':False}
try:
    b('--profile', str(ROOT/'.local/md-yuyuhakusho/review-polish-preview/picker-browser'), 'open', args.url)
    b('set', 'viewport', '880', '1150')
    b('wait', '--fn', '!!document.querySelector("[aria-label=比赛准备]")')
    if args.arenas:
        count = js('document.querySelectorAll("[data-arena-picker]").length')
        assert count == 1, f'missing arena image picker: {count}'
        b('click', '[data-arena-picker] summary')
        b('wait', '--fn', 'document.querySelectorAll("[data-arena-option]").length===8')
        b('wait', '--fn', '[...document.querySelectorAll("[data-arena-option] img")].every(i=>i.complete&&i.naturalWidth===1280&&i.naturalHeight===88)')
        b('screenshot', str(OUT/'arena-desktop.png'))
        b('click', '[data-arena-option="10"]')
        assert js('mdPolishEvents.at(-1)') == {'type':'arena','arena':10}
        assert js('document.querySelector("[data-arena-picker]").dataset.arena') == '10'
        align = js('(()=>{const a=document.querySelector("[data-arena-field]>span").getBoundingClientRect(),b=document.querySelector(".yuyu-role>span").getBoundingClientRect(),c=document.querySelector("[data-arena-picker] summary").getBoundingClientRect(),d=document.querySelector("[data-character-seat] summary").getBoundingClientRect();return {captions:Math.abs(a.left-b.left),controls:Math.abs(c.left-d.left)}})()')
        assert align['captions']<=1 and align['controls']<=1, align
        report['arena_selection'] = {'count':8,'selected':10,'alignment':align}
    count = js('document.querySelectorAll("details[data-character-seat]").length')
    assert count == 2, f'missing portrait pickers: {count}'
    b('click', '[data-character-seat="0"] summary')
    b('wait', '--fn', 'document.querySelectorAll("[data-character-option]").length===11')
    b('wait', '--fn', '[...document.querySelectorAll("[data-character-option] img")].every(i=>i.complete&&i.naturalWidth===352&&i.naturalHeight===32)')
    names = js('[...document.querySelectorAll("[data-character-option]")].map(b=>b.textContent.trim())')
    assert names == ['幻海','藏马','桑原','浦饭','飞影','仙水','阵','树','户愚吕兄','酎','户愚吕弟'], names
    desktop = js('(()=>{const r=document.querySelector(".yuyu-character-grid").getBoundingClientRect();return {top:r.top,bottom:r.bottom,height:innerHeight}})()')
    assert desktop['top'] >= 0 and desktop['bottom'] <= desktop['height'], desktop
    b('screenshot', str(OUT/'desktop-open.png'))
    b('click', '[data-character-option="8"]')
    assert js('mdPolishEvents.at(-1)') == {'type':'character','seat':0,'character':8}
    assert js('document.querySelectorAll("[data-character-seat]")[0].dataset.character') == '8'
    assert not js('document.querySelectorAll("[data-character-seat]")[0].open')
    b('click', '[data-character-seat="1"] summary')
    b('wait', '--fn', 'document.querySelectorAll("[data-character-option]").length===11')
    b('click', '[data-character-option="10"]')
    assert js('mdPolishEvents.at(-1)') == {'type':'character','seat':1,'character':10}
    b('set', 'viewport', '320', '740')
    b('click', '[data-character-seat="0"] summary')
    b('wait', '--fn', 'document.querySelectorAll("[data-character-option]").length===11')
    geometry = js('(()=>{const r=document.querySelector(".yuyu-character-grid").getBoundingClientRect();return {left:r.left,right:r.right,top:r.top,bottom:r.bottom,width:innerWidth,height:innerHeight,overflow:document.documentElement.scrollWidth>innerWidth}})()')
    assert geometry['left'] >= 0 and geometry['right'] <= geometry['width'] and not geometry['overflow'], geometry
    assert geometry['top'] >= 0 and geometry['bottom'] <= geometry['height'], geometry
    b('screenshot', str(OUT/'mobile-open.png'))
    b('press', 'Escape')
    assert not js('document.querySelectorAll("[data-character-seat]")[0].open')
    b('click', '[data-character-seat="1"] summary')
    b('wait', '--fn', 'document.querySelectorAll("[data-character-option]").length===11')
    b('click', '[data-character-option="3"]')
    assert js('mdPolishEvents.at(-1)') == {'type':'character','seat':1,'character':3}
    b('screenshot', str(OUT/'mobile-selected.png'))
    report.update(pass_=True, names=names, geometry=geometry, events=js('mdPolishEvents'))
    report['pass'] = report.pop('pass_')
except Exception as error:
    report['error'] = repr(error)
    report['diagnostic'] = js('[...document.querySelectorAll("details[data-character-seat]")].map(d=>({seat:d.dataset.characterSeat,open:d.open,options:d.querySelectorAll("[data-character-option]").length,summary:d.querySelector("summary").outerHTML}))')
    if not args.red: raise
    assert 'missing portrait pickers' in str(error) or 'missing arena image picker' in str(error), error
    report['expected_missing_picker_red'] = True
finally:
    report['checker_sha256'] = hashlib.sha256(Path(__file__).read_bytes()).hexdigest()
    (OUT/'result.json').write_text(json.dumps(report, ensure_ascii=False, indent=2)+'\n')
    b('close')
print(json.dumps({'pass':report['pass'], 'result':str(OUT/'result.json')}))
