#!/usr/bin/env python3
"""Rebuild racing media from retained licensed source files (Pillow, ffmpeg, resvg)."""
import argparse, array, colorsys, hashlib, json, math, re, shutil, subprocess, wave
from io import BytesIO
from pathlib import Path
from zipfile import ZipFile
from PIL import Image, ImageDraw

ROOT = Path(__file__).resolve().parents[2]
ASSETS = ROOT / 'games/racing/assets'; SOURCE = ASSETS / 'source'
PUBLIC = ASSETS / 'public/racing/v1'; TMP = ROOT / '.local/racing/assets'
for d in (SOURCE, PUBLIC, TMP): d.mkdir(parents=True, exist_ok=True)
p = argparse.ArgumentParser(description=__doc__)
p.add_argument('--resvg', default=str(ROOT / '.local/racing/rasterizer/node_modules/@resvg/resvg-js'))
args = p.parse_args()
sha = lambda b: hashlib.sha256(b).hexdigest()
relative = lambda p: p.relative_to(ROOT).as_posix()
sources = []; entries = []
def record(path, author, license, url, **extra):
    sources.append(dict(path=relative(path), sha256=sha(path.read_bytes()), author=author, license=license, sourceURL=url, **extra))
    return path
def retain(input, name, author, license, url, **extra):
    target = SOURCE / name; target.parent.mkdir(parents=True, exist_ok=True)
    if input is not None and input.exists(): shutil.copyfile(input, target)
    assert target.exists(), f'Missing retained source: {target}'
    return record(target, author, license, url, **extra)
pack = 'https://kenney.nl/assets/racing-pack'
zip_path = ROOT / '.local/racing-research/images/downloads/kenney-racing-pack.zip'
def archive(member, name):
    target = SOURCE / name; target.parent.mkdir(parents=True, exist_ok=True)
    if not target.exists():
        assert sha(zip_path.read_bytes()) == 'a5e5252082e2bcbac177402d3bde3aa161fd81aeb246a36a1ec39255d1f30480'
        with ZipFile(zip_path) as z: target.write_bytes(z.read(member))
    return record(target, 'Kenney', 'CC0-1.0', pack, archiveMember=member)
def emit(name, type, extension, used, **extra):
    target = PUBLIC / (name + extension)
    entries.append(dict(name=name, type=type, output=relative(target), urlPath='/racing/v1/' + target.name,
                        bytes=target.stat().st_size, sha256=sha(target.read_bytes()),
                        mime={'.webp': 'image/webp', '.mp3': 'audio/mpeg', '.json': 'application/json'}[extension],
                        sources=[{'path': relative(s), 'sha256': sha(s.read_bytes())} for s in used],
                        generator='scripts/racing/prepare-assets.py', **extra))
def render(svg, name):
    f = TMP / (name + '.svg'); f.write_text(svg)
    target = TMP / (name + '.png')
    script = "const fs=require('fs'),{Resvg}=require(process.argv[1]);fs.writeFileSync(process.argv[3],new Resvg(fs.readFileSync(process.argv[2],'utf8')).render().asPng())"
    subprocess.run(['node', '-e', script, args.resvg, str(f), str(target)], check=True)
    return Image.open(target).convert('RGBA')
def webp(im, name, used):
    im.save(SOURCE / (name + '.png'), compress_level=9)
    im.save(PUBLIC / (name + '.webp'), lossless=True, method=6)
    emit(name, 'image', '.webp', used, width=im.width, height=im.height)

vector = archive('Vector/vector_objects.svg', 'kenney/vector_objects.svg')
license_paths = [archive('License.txt', 'kenney/License.txt')]
svg_original = vector.read_text()
seat_colors = ['#ce6657', '#548faf', '#4d9373', '#d0a945', '#9473ae', '#586b7c']
body_names = ['流线跑车', '经典轿跑', '运动轿车', '轻型皮卡', '紧凑掀背']
cars = Image.new('RGBA', (1152, 1600)); sprites = []
rows = [(8, 70), (79, 62), (144, 70), (218, 70), (294, 64)]
for body, (y, height) in enumerate(rows):
    for seat, color in enumerate(seat_colors):
        th, ts, tv = colorsys.rgb_to_hsv(*(int(color[i:i+2], 16)/255 for i in (1, 3, 5)))
        def recolor(match):
            raw = match.group(); h, s, v = colorsys.rgb_to_hsv(*(int(raw[i:i+2], 16)/255 for i in (1, 3, 5)))
            if .51 < h < .64 and s > .35:
                rgb = colorsys.hsv_to_rgb(th, min(.72, ts * s / .77), min(1, tv * v / .82))
                return '#' + ''.join(f'{round(c*255):02x}' for c in rgb)
            return raw
        svg = re.sub(r'#[0-9a-f]{6}', recolor, svg_original)
        svg = svg.replace('<svg ', f'<svg viewBox="88 {y} 40 {height}" width="160" height="{height*4}" ')
        im = render(svg, f'car-{seat}-{body}')
        x0, y0 = seat*192, body*320
        cars.alpha_composite(im, (x0 + (192-im.width)//2, y0 + (320-im.height)//2))
        sprites.append({'name': f'car-{seat}-{body}', 'seat': seat, 'body': body, 'rect': [x0, y0, 192, 320], 'anchor': [.5, .5]})
webp(cars, 'cars', [vector])

scenery = Image.new('RGBA', (1024, 1280)); scenery_sprites = []; scenery_sources = []
objects = [('tree', 'Objects/tree_large.png'), ('rock', 'Objects/rock1.png'), ('cone', 'Objects/cone_straight.png'),
 ('barrier', 'Objects/barrier_white.png'), ('lights', 'Objects/lights.png'), ('grandstand', 'Objects/tribune_full.png'),
 ('skid', 'Objects/skidmark_long_1.png'), ('grass', 'Tiles/Grass/land_grass11.png'), ('dirt', 'Tiles/Dirt/land_dirt12.png'),
 ('sand', 'Tiles/Sand/land_sand12.png'), ('asphalt', 'Tiles/Asphalt road/road_asphalt22.png'),
 ('road-dirt', 'Tiles/Dirt road/road_dirt22.png'), ('road-sand', 'Tiles/Sand road/road_sand22.png'),
 ('arrow', 'Objects/arrow_white.png'), ('tires', 'Objects/tires_white.png'), ('tent', 'Objects/tent_blue.png')]
for name, member in objects:
    scenery_sources.append(archive('PNG/' + member, 'kenney/' + member))
evidence = ROOT / 'docs/research/racing/evidence/images'
for name, pack_name, member in [('smoke', 'smoke-particles', 'PNG/White puff/whitePuff00.png'),
                             ('dust', 'smoke-particles', 'PNG/White puff/whitePuff01.png'),
                             ('spark', 'particle-pack', 'PNG (Transparent)/Rotated/spark_05_rotated.png')]:
    scenery_sources.append(retain(evidence / pack_name / member, f'kenney/{name}.png', 'Kenney', 'CC0-1.0', 'https://kenney.nl/assets/' + pack_name))
    objects.append((name, member))
    if not (SOURCE / f'kenney/{pack_name}-License.txt').exists():
        license_paths.append(retain(evidence / pack_name / 'License.txt', f'kenney/{pack_name}-License.txt', 'Kenney', 'CC0-1.0', 'https://kenney.nl/assets/' + pack_name))
    else: license_paths.append(record(SOURCE / f'kenney/{pack_name}-License.txt', 'Kenney', 'CC0-1.0', 'https://kenney.nl/assets/' + pack_name))
for i, ((name, _), source) in enumerate(zip(objects, scenery_sources)):
    im = Image.open(source).convert('RGBA')
    im.thumbnail((240, 240), Image.Resampling.LANCZOS)
    # Mildly desaturate natural/painted surfaces while preserving transparency.
    if name not in {'smoke', 'dust', 'spark', 'skid', 'lights'}:
        from PIL import ImageEnhance
        im = ImageEnhance.Color(im).enhance(.76)
    if name == 'spark':
        tint = Image.new('RGBA', im.size, '#f9d778'); tint.putalpha(im.getchannel('A')); im = tint
    x, y = (i % 4)*256 + (256-im.width)//2, (i // 4)*256 + (256-im.height)//2
    scenery.alpha_composite(im, (x, y))
    scenery_sprites.append({'name': name, 'rect': [x, y, im.width, im.height], 'anchor': [.5, .5]})
webp(scenery, 'scenery', scenery_sources)
atlas = {'version': 1, 'cars': {'file': 'cars.webp', 'cell': [192, 320], 'size': list(cars.size), 'columns': 6, 'rows': 5,
 'heading': 'up', 'seatColors': seat_colors, 'bodyNames': body_names, 'sprites': sprites},
 'scenery': {'file': 'scenery.webp', 'size': list(scenery.size), 'sprites': scenery_sprites}}
(PUBLIC / 'atlas.json').write_text(json.dumps(atlas, ensure_ascii=False, indent=2) + '\n')
emit('atlas', 'metadata', '.json', [vector] + scenery_sources)
cover = record(SOURCE / 'cover.svg', 'wire-board contributors', 'Project original', 'https://github.com/k0ngk0ng/wire-board/tree/main/games/racing/assets')
webp(render(cover.read_text(), 'cover'), 'cover', [cover])

# Mono 44.1 kHz source masters, deterministic fades, conservative peak levels.
audio_evidence = ROOT / 'docs/research/racing/evidence/audio'
audio_meta = [
 ('engine', 'engine-loop-0.wav', 'domasx2', 'CC0-1.0', 'https://opengameart.org/content/racing-car-engine-sound-loops', .54, True),
 ('tires', 'tire-loop.wav', 'audible-edge (Tom Haigh); loop edit by qubodup', 'CC-BY-3.0', 'https://opengameart.org/content/car-tire-squeal-skid-loop', .44, True),
 ('collision-light', 'impact-soft-heavy.ogg', 'Kenney', 'CC0-1.0', 'https://kenney.nl/assets/impact-sounds', .48, False),
 ('collision-heavy', 'impact-metal-medium.ogg', 'Kenney', 'CC0-1.0', 'https://kenney.nl/assets/impact-sounds', .61, False)]
def write_wav(path, samples):
    pcm = array.array('h', (round(max(-.99, min(.99, x))*32767) for x in samples))
    with wave.open(str(path), 'wb') as w: w.setnchannels(1); w.setsampwidth(2); w.setframerate(44100); w.writeframes(pcm.tobytes())
def export_audio(name, samples, used, peak, loop=False):
    mean = sum(samples)/len(samples); samples = [v-mean for v in samples]
    level = max(abs(v) for v in samples); samples = [v*peak/max(level, .0001) for v in samples]
    # 3 ms fades meet at zero in a loop; avoid codec-edge clicks without a long audible gap.
    fade = min(132 if loop else 330, len(samples)//3)
    for i in range(fade):
        gain = math.sin(math.pi/2*i/fade); samples[i] *= gain; samples[-1-i] *= gain
    master = SOURCE / 'audio' / (name + '-master.wav'); master.parent.mkdir(exist_ok=True)
    write_wav(master, samples)
    subprocess.run(['ffmpeg', '-nostdin', '-v', 'error', '-y', '-i', str(master), '-codec:a', 'libmp3lame', '-b:a', '160k', '-map_metadata', '-1', str(PUBLIC / (name+'.mp3'))], check=True)
    emit(name, 'audio', '.mp3', used + [master], loop=loop, sampleRate=44100, channels=1)
for name, filename, author, license, url, peak, loop in audio_meta:
    source = retain(audio_evidence / filename, 'audio/original/' + filename, author, license, url)
    raw = subprocess.run(['ffmpeg', '-nostdin', '-v', 'error', '-i', str(source), '-f', 'f32le', '-ac', '1', '-ar', '44100', '-'], check=True, capture_output=True).stdout
    values = array.array('f'); values.frombytes(raw)
    export_audio(name, list(values), [source], peak, loop)
def tone_sequence(notes, duration, width=.18):
    out = [0.] * round(duration*44100)
    for start, frequency, length in notes:
        for i in range(round(length*44100)):
            n = round(start*44100)+i
            if n >= len(out): break
            t = i/44100; env = min(1, t/.009) * max(0, 1-t/length)**1.4
            out[n] += env*(math.sin(2*math.pi*frequency*t) + .18*math.sin(4*math.pi*frequency*t))
    return out
synthetics = {
 'countdown': ([(0, 523.251, .17)], .24),
 'start': ([(0, 783.991, .42), (0, 1046.502, .42)], .52),
 'lap': ([(0, 659.255, .17), (.12, 783.991, .17), (.24, 1046.502, .28)], .6),
 'finish': ([(0, 523.251, .23), (.17, 659.255, .23), (.34, 783.991, .23), (.53, 1046.502, .65), (.53, 783.991, .65), (.53, 659.255, .65)], 1.3)}
for name, (notes, duration) in synthetics.items():
    export_audio(name, tone_sequence(notes, duration), [ROOT / 'scripts/racing/prepare-assets.py'], .48)
    record(SOURCE / 'audio' / (name + '-master.wav'), 'wire-board contributors', 'Project original', 'https://github.com/k0ngk0ng/wire-board/tree/main/scripts/racing', method='Offline additive synthesis; sine fundamentals, second harmonic, 9 ms attack and decaying release.')
for filename in ('cc0-deed.html.txt', 'ccby-3-deed.html.txt', 'tire-loop-page.html.txt', 'engine-loops-page.html.txt', 'impact-License.txt'):
    retain(audio_evidence / filename, 'licenses/' + filename, 'Source authors / Creative Commons', 'License evidence', 'https://opengameart.org/')
attribution = {'game': 'racing', 'version': 1, 'sources': [
 {'id': 'kenney', 'author': 'Kenney', 'title': 'Racing Pack; Smoke Particles; Particle Pack; Impact Sounds', 'sourceURL': pack, 'license': 'CC0-1.0', 'licenseURL': 'https://creativecommons.org/publicdomain/zero/1.0/', 'modifications': 'SVG car crops rendered at 4x; six seat palettes, sprite packing, mild desaturation, audio conversion and gain.'},
 {'id': 'domasx2-engine', 'author': 'domasx2', 'title': 'racing car engine sound loops', 'sourceURL': audio_meta[0][4], 'license': 'CC0-1.0', 'licenseURL': 'https://creativecommons.org/publicdomain/zero/1.0/', 'modifications': 'Mono 44.1 kHz, DC removal, peak normalization, 3 ms seam fades, MP3 encoding.'},
 {'id': 'tom-haigh-tire', 'author': 'audible-edge (Tom Haigh); loop edited by qubodup', 'title': 'Car tire squeal skid loop', 'sourceURL': audio_meta[1][4], 'upstreamURL': 'https://freesound.org/people/audible-edge/sounds/71739/', 'license': 'CC-BY-3.0', 'licenseURL': 'https://creativecommons.org/licenses/by/3.0/', 'modifications': 'Resampled to mono 44.1 kHz; DC removal, peak normalization, 3 ms seam fades and MP3 encoding.'},
 {'id': 'original', 'author': 'wire-board contributors', 'title': 'Racing cover; countdown, start, lap and finish cues', 'sourceURL': 'https://github.com/k0ngk0ng/wire-board/tree/main/games/racing', 'license': 'Project original', 'modifications': 'Original SVG drawing and offline additive audio synthesis.'}]}
(PUBLIC / 'attribution.json').write_text(json.dumps(attribution, ensure_ascii=False, indent=2) + '\n')
emit('attribution', 'metadata', '.json', license_paths)
(ASSETS / 'source-manifest.json').write_text(json.dumps({'version': 1, 'entries': list({s['path']: s for s in sources}.values())}, ensure_ascii=False, indent=2) + '\n')
(ASSETS / 'manifest.json').write_text(json.dumps({'version': 1, 'game': 'racing', 'assetsBaseURL': 'https://cdn.ichenj.com/bga/v20261002-1', 'entries': entries}, ensure_ascii=False, indent=2) + '\n')
# Stable visual artifact: downsampled sprites on a warm neutral background.
preview = Image.new('RGB', (1152, 1600), '#e7e4d8'); preview.paste(cars, mask=cars.getchannel('A'))
preview.resize((576, 800), Image.Resampling.LANCZOS).save(TMP / 'cars-contact.png')
preview = Image.new('RGB', scenery.size, '#d9d9ce'); preview.paste(scenery, mask=scenery.getchannel('A')); preview.save(TMP / 'scenery-contact.png')
print('Prepared', len(entries), 'racing media objects; 30 SVG-rendered cars and', len(scenery_sprites), 'scene sprites.')
