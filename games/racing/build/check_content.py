import argparse
import importlib.util
import json
from pathlib import Path
import traceback

ROOT = Path(__file__).resolve().parents[3]
spec = importlib.util.spec_from_file_location('racing_browser_acceptance', ROOT / 'scripts/racing/check-nfs.py')
browser_module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(browser_module)


def preview(job, kind, ident):
    value = job.js('''(() => {
      const image = document.querySelector('img[data-racing-preview="''' + kind + '''"]');
      if (!image || !image.complete || image.naturalWidth < 256) return null;
      const rect=image.getBoundingClientRect();
      return {source:new URL(image.currentSrc).pathname,alt:image.alt,width:image.naturalWidth,height:image.naturalHeight,
        box:{left:rect.left,right:rect.right,top:rect.top,bottom:rect.bottom,width:rect.width,height:rect.height},viewport:innerWidth,scroll:document.documentElement.scrollWidth};
    })()''')
    if value is None or not value['source'].endswith('/' + ident + '.webp'):
        return None
    browser_module.require(value['box']['width'] > 100 and value['box']['height'] > 40, 'preview is not visibly sized', value)
    browser_module.require(value['box']['left'] >= 0 and value['box']['right'] <= value['viewport'] + 1 and value['scroll'] <= value['viewport'], 'preview or selection overflows the viewport', value)
    return value


def choices(job, width):
    job.browser('set', 'viewport', str(width), '1000' if width > 320 else '844')
    job.browser('click', '.lobby-create')
    job.browser('wait', '[role=dialog]')
    job.browser('fill', 'input[aria-label="搜索可选游戏"]', '极品飞车')
    job.browser('click', '.choose-games button')
    job.browser('wait', 'select[aria-label="赛道"]')
    tracks = ['france', 'coastal', 'uk', 'park', 'germany']
    for ident in tracks:
        job.browser('select', 'select[aria-label="赛道"]', ident)
        image = job.wait(lambda: preview(job, 'track', ident), description='track preview ' + ident)
        job.check('native track preview selection', width=width, track=ident, image=image)
    job.browser('scrollintoview', '.racing-track-choice')
    job.screenshot('track-choices-' + str(width))
    job.browser('select', 'select[aria-label="座位上限"]', '2')
    job.browser('fill', 'input[name=name]', '极品飞车扩充验收 ' + str(width))
    job.browser('scrollintoview', '[role=dialog] button[type=submit]')
    job.browser('snapshot', '-i')
    job.browser('click', '[role=dialog] button[type=submit]')
    job.record('submitted-' + str(width), job.js('({text:document.body.innerText,dialog:!!document.querySelector("[role=dialog]")})'))
    job.browser('wait', '.waiting-layout')
    for ident in ['p911', 'fm50', 'ldia', 'mcf1', 'ccor', 'bmw5', 'jxkr', 'mclk']:
        job.browser('select', '.racing-preparation select[aria-label="车型"]', ident)
        job.wait(lambda: job.room()['options']['vehicles'][0] == ident, description='prepared original vehicle')
        image = job.wait(lambda: preview(job, 'vehicle', ident), description='vehicle preview ' + ident)
        job.check('native vehicle preview selection', width=width, vehicle=ident, image=image)
    job.browser('scrollintoview', '.racing-preparation')
    job.screenshot('vehicle-choices-' + str(width))
    requests = job.js("performance.getEntriesByType('resource').filter(r=>/\\/racing\\/v3\\/.*\\.glb/.test(r.name)).map(r=>r.name)")
    browser_module.require(not requests, 'room thumbnails eagerly loaded 3D scenes', requests)
    job.check('room thumbnails do not load game models', width=width)
    job.command('leave')
    job.browser('wait', '.room-list')


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--out', required=True)
    parser.add_argument('--dist', default='.local/racing-release/web/dist')
    parser.add_argument('--base', required=True)
    args = parser.parse_args()
    args.phase = 'content'
    args.build_identity = None
    job = browser_module.Acceptance(args)
    try:
        job.start_fixture()
        choices(job, 1280)
        choices(job, 320)
        job.browser('set', 'viewport', '1280', '1000')
        for ident in ['bmw5', 'jxkr', 'mclk']:
            job.create_ui('germany', vehicle=ident, players=2, bots=1)
            job.enter('germany', vehicle=ident)
            job.browser('focus', '.racing-chase')
            result = job.native('new-vehicle-driving-' + ident, [{'key': 'w', 'down': True}, {'wait': 700}, {'key': 'w', 'down': False}, {'key': 'c', 'down': True}, {'key': 'c', 'down': False}, {'wait': 180}, {'key': 'c', 'down': True}, {'key': 'c', 'down': False}, {'wait': 180}, {'key': 'c', 'down': True}, {'key': 'c', 'down': False}, {'wait': 250}, {'read': 'cockpit'}, {'shot': ident + '-cockpit'}])
            browser_module.require(not result.get('exceptions'), 'new original car raised a browser exception', result)
            rows = {row['name']: row['data'] for row in result['samples']}
            canvas = rows['cockpit']['canvas']
            parts = json.loads(canvas.get('cockpitDynamics', '[]'))
            expected = {'steering', 'rpm'} | ({'speed'} if ident != 'mclk' else set())
            browser_module.require(canvas.get('cameraMode') == '3' and {part['kind'] for part in parts} == expected, 'new original cockpit did not mount all native moving parts', canvas)
            job.check('new original vehicle native driving and cockpit', vehicle=ident, parts=parts)
            job.close_room()
        job.create_ui('germany', night=True, vehicle='mclk', players=2, bots=1)
        job.enter('germany', night=True, vehicle='mclk')
        job.screenshot('germany-night')
        job.check('new original course night scene')
        job.close_room()
        job.report['passed'] = True
    except Exception as error:
        job.report['error'] = str(error)
        job.report['traceback'] = traceback.format_exc()
        print(job.report['traceback'], flush=True)
        if job.sessions.get('main', {}).get('opened'):
            job.screenshot('failure')
    finally:
        job.finish()
    return 0 if job.report['passed'] else 1


if __name__ == '__main__':
    raise SystemExit(main())
