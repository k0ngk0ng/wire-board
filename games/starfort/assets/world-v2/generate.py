#!/usr/bin/env python3
"""Original, deterministic cartoon world tiles, characters and sounds for 星垒.

No models, downloaded images, fonts or sampled audio. Requires Pillow + repo resvg.
"""
from array import array
import hashlib
import io
import json
import math
from pathlib import Path
import random
import subprocess
import wave
from PIL import Image, ImageDraw

ROOT = Path(__file__).resolve().parent
PUBLIC = ROOT.parent / 'public/starfort/world-v2'
SOURCE = ROOT / 'src'
INK = '#334a49'
BIOMES = ('forest', 'canyon', 'river', 'snow', 'ruins', 'marsh')
PROPS = ('tree', 'pine', 'rock', 'bridge', 'highground', 'watchtower', 'mine', 'cart', 'gate', 'ruins', 'reeds')
PALETTE = {
    'forest': ('#c8d59c', '#b7c68a', '#9aaa74'),
    'canyon': ('#e0c6a0', '#d2b78e', '#b99779'),
    'river': ('#bbd3c0', '#a4c6b4', '#83a7a4'),
    'snow': ('#e2ede5', '#d1e1dc', '#b8cdc9'),
    'ruins': ('#cbd0ba', '#b8c2a4', '#929f8e'),
    'marsh': ('#b4c9ae', '#9dbca2', '#7a9e98'),
}
SPRITES = {}


def inventory():
    result = {f'tile-{b}': (256, 256) for b in BIOMES}
    result.update({f'choice-{b}': (256, 160) for b in BIOMES})
    result.update({f'prop-{p}': (128, 128) for p in PROPS})
    result.update({f'enemy-{k}-{pose}': (128, 128) for k in ('healer', 'banner', 'splitter', 'engineer') for pose in range(3)})
    result.update({f'event-{e}': (64, 64) for e in ('fog', 'frost', 'surge', 'star')})
    result['effect-boss-aura'] = (128, 128)
    return result


def layout():
    frames = {}
    x = y = row = 0
    for name, (w, h) in inventory().items():
        if x + w > 1024:
            y += row + 4
            x = row = 0
        frames[name] = {'x': x, 'y': y, 'w': w, 'h': h}
        x += w + 4
        row = max(row, h)
    return {'frames': frames, 'meta': {'image': 'atlas.webp', 'width': 1024, 'height': y + row}}


def star(cx, cy, r, fill='#ffe99a', stroke='#aa7b42', sw=2):
    pts = []
    for i in range(10):
        a = i * math.pi / 5 - math.pi / 2
        rr = r if i % 2 == 0 else r * .46
        pts.append(f'{cx+math.cos(a)*rr:.2f},{cy+math.sin(a)*rr:.2f}')
    return f'<polygon points="{" ".join(pts)}" fill="{fill}" stroke="{stroke}" stroke-width="{sw}"/>'


def svg(w, h, body):
    return f'''<svg xmlns="http://www.w3.org/2000/svg" width="{w}" height="{h}" viewBox="0 0 {w} {h}">
<defs>
<linearGradient id="stone" x2=".2" y2="1"><stop stop-color="#f8e9be"/><stop offset="1" stop-color="#b6a48a"/></linearGradient>
<linearGradient id="wood" x2=".3" y2="1"><stop stop-color="#e6b378"/><stop offset="1" stop-color="#9a6349"/></linearGradient>
<linearGradient id="leaf" x2=".3" y2="1"><stop stop-color="#b1d07a"/><stop offset="1" stop-color="#54866a"/></linearGradient>
<linearGradient id="water" x2=".4" y2="1"><stop stop-color="#baded9"/><stop offset="1" stop-color="#6ea5b0"/></linearGradient>
<linearGradient id="ice" x2=".5" y2="1"><stop stop-color="#e5ffff"/><stop offset=".45" stop-color="#97dfef"/><stop offset="1" stop-color="#4897c5"/></linearGradient>
<linearGradient id="magic" x2=".2" y2="1"><stop stop-color="#ddc8f4"/><stop offset="1" stop-color="#8265ad"/></linearGradient>
<linearGradient id="gold" x2=".4" y2="1"><stop stop-color="#fff2ab"/><stop offset=".5" stop-color="#e9bc5d"/><stop offset="1" stop-color="#b77f3f"/></linearGradient>
<linearGradient id="clay" x2=".3" y2="1"><stop stop-color="#edcdab"/><stop offset="1" stop-color="#b07f65"/></linearGradient>
<radialGradient id="glow"><stop stop-color="#ffe7a5" stop-opacity=".6"/><stop offset="1" stop-color="#ffe7a5" stop-opacity="0"/></radialGradient>
</defs><g stroke-linejoin="round" stroke-linecap="round">{body}</g></svg>'''


def export(name, body):
    w, h = inventory()[name]
    path = SOURCE / (name + '.svg')
    path.write_text(svg(w, h, body))
    result = subprocess.run(['node', str(ROOT.parent / 'rasterize.mjs'), str(path)], cwd=ROOT.parents[3], check=True, capture_output=True)
    im = Image.open(io.BytesIO(result.stdout)).convert('RGBA').resize((w, h), Image.Resampling.LANCZOS)
    im.save(PUBLIC / (name + '.webp'), 'WEBP', lossless=True, method=6, exact=True)
    SPRITES[name] = im


def tile(biome):
    base, mid, dark = PALETTE[biome]
    rng = random.Random('starfort-world-v2-' + biome)
    body = f'<rect width="256" height="256" fill="{base}"/>'
    # Every motif stays at least eight pixels from the tile edge for seam-free repeat.
    for i in range(22):
        x, y = rng.randint(16, 235), rng.randint(16, 237)
        rx, ry = rng.randint(3, 12), rng.randint(2, 5)
        body += f'<ellipse cx="{x}" cy="{y}" rx="{rx}" ry="{ry}" fill="{mid}" opacity=".3"/>'
    for i in range(9):
        x, y = rng.randint(18, 226), rng.randint(18, 226)
        if biome in ('forest', 'river', 'marsh'):
            body += f'<path d="M{x} {y}l-3 -5m3 5l2 -7m-2 7l6 -3" fill="none" stroke="{dark}" stroke-width="1.3" opacity=".42"/>'
        elif biome == 'canyon':
            body += f'<path d="M{x-5} {y}l4 -3 7 1 2 3 -8 2Z" fill="{mid}" stroke="{dark}" stroke-width=".7" opacity=".6"/>'
        elif biome == 'snow':
            body += f'<path d="M{x-4} {y}q5 -4 11 0" stroke="#f5faf4" stroke-width="2" fill="none"/>'
        else:
            body += f'<path d="M{x-6} {y+2}l1 -5 11 -2 2 5 -6 4Z" fill="{mid}" stroke="{dark}" stroke-width=".9" opacity=".5"/>'
    export('tile-' + biome, body)


def prop(kind):
    body = '<ellipse cx="64" cy="111" rx="49" ry="8" fill="#244435" opacity=".17"/>'
    if kind == 'tree':
        body += f'''<g stroke="{INK}" stroke-width="2.8"><path d="M55 105L56 65L72 63L77 107Q66 116 55 105Z" fill="url(#wood)"/>
<path d="M63 92L48 78M68 82L82 68" fill="none" stroke="#8e7254"/>
<path d="M23 67Q13 43 35 36Q28 17 51 19Q67 4 79 24Q103 16 104 44Q121 57 103 75Q88 88 72 78Q51 93 36 79Q22 85 23 67Z" fill="url(#leaf)"/>
<path d="M30 53Q27 42 42 42M50 31Q62 24 70 34M84 48Q98 42 100 55" stroke="#cfdf9b" stroke-width="5" fill="none"/>
<path d="M44 69Q56 77 64 64M79 73Q87 65 86 59" fill="none" stroke="#54866a" stroke-width="2"/></g>'''
    elif kind == 'pine':
        body += f'''<g stroke="{INK}" stroke-width="2.8"><path d="M55 106V73H73V108Z" fill="url(#wood)"/>
<path d="M18 95L38 66L30 67L48 41L43 42L64 12L85 43L80 42L98 69L90 68L111 96Q66 114 18 95Z" fill="url(#leaf)"/>
<path d="M64 12L48 41L43 42L64 37L82 42M38 66L61 55L91 69M23 89L56 73L95 88" fill="none" stroke="#dce9d0" stroke-width="7"/>
<path d="M64 46L69 63M53 88L62 95M80 89L88 93" stroke="#568778" fill="none"/></g>'''
    elif kind in ('rock', 'highground'):
        body += f'''<g stroke="{INK}" stroke-width="2.8"><path d="M17 94L30 47L58 30L98 40L113 83L98 108L38 111Z" fill="url(#{'clay' if kind == 'highground' else 'stone'})"/>
<path d="M17 94L50 90L67 60L98 40L113 83L98 108L38 111Z" fill="{'#a57864' if kind == 'highground' else '#9faba0'}"/>
<path d="M30 47L56 63L67 60M50 90L38 111M79 78L100 88" fill="none" stroke="#777e70" stroke-width="2"/>
<path d="M33 48L59 38L86 44" fill="none" stroke="#f4deb7" stroke-width="4"/></g>'''
        if kind == 'highground':
            body += f'<path d="M27 49L48 29Q68 20 98 36L101 44Q67 65 27 49Z" fill="#a6b27a" stroke="{INK}" stroke-width="2.5"/><path d="M38 43L55 35M67 40L78 35" stroke="#d4d09a" stroke-width="3"/>'
    elif kind == 'bridge':
        body += f'''<g stroke="{INK}" stroke-width="2.5"><path d="M11 97Q61 71 117 94L116 107Q70 120 13 108Z" fill="url(#water)" stroke="#60979c"/>
<path d="M18 88L33 37Q68 24 105 39L112 89Q95 83 89 79L85 54Q65 48 47 57L40 87Z" fill="url(#stone)"/>
<path d="M22 75Q66 56 108 76L110 88Q64 67 20 88Z" fill="#c5b691"/>
<path d="M34 37L28 28Q65 10 109 30L105 39Q65 24 34 37Z" fill="#eee1b3"/>
<path d="M27 57Q65 39 106 58M51 32V44M76 29V43M93 34V50" fill="none" stroke="#9d987f" stroke-width="2"/>
<path d="M28 28L26 43M109 30L111 44" stroke="#d8cda4" stroke-width="6"/></g>
<path d="M24 101q13 -4 25 -1M78 104q10 -4 22 -2" stroke="#d2ebe0" stroke-width="3" fill="none"/>'''
    elif kind == 'watchtower':
        body += f'''<g stroke="{INK}" stroke-width="2.6"><path d="M41 107L49 56H81L89 108Q64 118 41 107Z" fill="url(#stone)"/>
<path d="M44 88L85 87M47 72H83M64 73V86M59 92V106" stroke="#959780" stroke-width="2" fill="none"/>
<path d="M32 55L36 42L90 42L97 55L89 66L39 66Z" fill="url(#wood)"/>
<path d="M43 45V25M83 45V25" stroke-width="5"/>
<path d="M28 26L64 10L100 27L94 35Q65 27 34 35Z" fill="#72908b"/>
<path d="M50 52L55 43L56 31L64 37L71 24L77 41L75 52Z" fill="url(#gold)" stroke="#b4804c"/>
<path d="M51 106V92Q64 81 76 92V108" fill="#526d66"/></g>'''
    elif kind == 'mine':
        body += f'''<g stroke="{INK}" stroke-width="2.6"><path d="M12 108L21 61L43 30L81 23L104 49L117 108Z" fill="url(#stone)"/>
<path d="M23 58L40 66L55 47L82 49L101 69M81 23L72 37L82 49M43 30L45 47" stroke="#969b85" stroke-width="2" fill="none"/>
<path d="M39 105V65Q62 40 91 65V109Z" fill="#34484c"/>
<path d="M33 107V61L42 60V107M88 107V59L99 62V107M34 58L96 57L97 68L34 70Z" fill="url(#wood)"/>
<path d="M33 110L49 84M101 110L80 83" stroke="#919fa0" stroke-width="4"/>
<path d="M42 94H86M36 102H94" stroke="#af8860" stroke-width="4"/>
<path d="M62 82L61 68L68 59L77 68L76 81Z" fill="url(#gold)"/>
<path d="M53 78L45 68L47 60L55 63L59 73" fill="#94c8c1"/></g>'''
    elif kind == 'cart':
        body += f'''<g stroke="{INK}" stroke-width="2.8"><path d="M24 57L18 84Q60 111 103 85L108 56Z" fill="url(#wood)"/>
<path d="M26 70L99 69M24 81L98 81M44 65V90M77 63V95" fill="none" stroke="#805d48" stroke-width="2"/>
<path d="M21 55L107 54L108 64L20 65Z" fill="#c6aa78"/>
<path d="M28 53L32 28Q52 13 91 31L100 54Z" fill="#92ad82"/>
<path d="M34 29Q54 45 90 32M61 29V53" stroke="#dfcf94" stroke-width="4" fill="none"/>
<path d="M95 86L119 96" stroke="#9e7554" stroke-width="5"/>
<circle cx="35" cy="101" r="15" fill="#705b4c"/><circle cx="87" cy="101" r="15" fill="#705b4c"/>
<circle cx="35" cy="101" r="8" fill="#c8a775"/><circle cx="87" cy="101" r="8" fill="#c8a775"/>
<path d="M35 89V113M23 101H47M87 89V113M75 101H99" stroke="#d8bc87" stroke-width="2"/></g>'''
        body += star(67, 54, 10, sw=1.6)
    elif kind == 'gate':
        body += f'''<g stroke="{INK}" stroke-width="2.8"><path d="M13 110L20 37L43 25L89 25L109 39L117 110L91 113L88 60Q67 42 43 60L41 113Z" fill="url(#stone)"/>
<path d="M21 37L41 46L43 25M89 25L89 46L109 39M18 62L41 65M94 65L112 66M15 90L42 91M90 91L115 88M49 29L47 47M79 28L81 48" fill="none" stroke="#959b85" stroke-width="2"/>
<path d="M46 49L53 35L64 43L78 31L85 47" stroke="#8e79a2" stroke-width="4" fill="none"/>
<path d="M16 108L18 94L31 103L43 99L40 113M91 112L99 98L109 104L116 96L117 110" fill="#89a578" stroke="none"/></g>'''
        body += star(65, 33, 9, '#d6c1ea', '#897196', 1.8)
    elif kind == 'ruins':
        body += f'''<g stroke="{INK}" stroke-width="2.7"><path d="M14 108L16 91L112 91L116 108Z" fill="url(#stone)"/>
<path d="M22 91L26 40L24 28L48 23L52 39L49 91M77 91L78 59L85 46L97 53L105 46L109 91" fill="url(#stone)"/>
<path d="M23 43L51 43M25 62L50 62M78 73L108 73M27 81H48M39 44L36 63M95 57L94 72M48 101H79" stroke="#8e9b83" stroke-width="2" fill="none"/>
<path d="M27 91Q43 83 53 91L63 90Q60 99 47 98L19 104L15 97Z" fill="#87a777" stroke="none"/>
<path d="M94 61L88 68L90 80L81 88" stroke="#7e9c6e" stroke-width="4" fill="none"/></g>'''
    elif kind == 'reeds':
        body += f'''<ellipse cx="64" cy="104" rx="47" ry="12" fill="url(#water)" stroke="#729b93" stroke-width="2"/>
<g stroke="#566f51" stroke-width="3" fill="none"><path d="M40 104Q43 69 35 39M62 103Q62 54 69 25M82 101Q83 69 99 49M52 103Q52 83 30 71M70 104Q69 77 86 61"/></g>
<g stroke="#715b47" stroke-width="5"><path d="M33 29L36 45M69 15L68 31M102 38L96 55"/></g>
<g fill="#8eab70" stroke="#66865c" stroke-width="1.5"><path d="M44 88Q28 56 23 62Q18 80 44 88M68 80Q64 55 79 45Q85 62 68 80M80 94Q95 73 110 80Q102 97 80 94"/></g>'''
    return body


def eyes(x, y):
    return f'<g fill="#2e4441"><ellipse cx="{x-11}" cy="{y}" rx="3" ry="4"/><ellipse cx="{x+11}" cy="{y}" rx="3" ry="4"/></g><path d="M{x-3} {y+10}q3 3 7 -1" fill="none" stroke="#466251" stroke-width="1.8"/>'


def enemy(kind, pose):
    bob, stride = (0, -3, 2)[pose], (0, 5, -5)[pose]
    body = '<ellipse cx="64" cy="112" rx="40" ry="7" fill="#244435" opacity=".20"/>'
    if kind == 'splitter':
        sx = (1, .93, 1.05)[pose]
        body += f'<g transform="translate(64 {108+bob}) scale({sx} {1/sx}) translate(-64 -108)" stroke="{INK}" stroke-width="2.8">'
        body += '<path d="M20 91Q14 59 38 48Q51 31 63 50Q82 26 99 52Q117 72 105 96Q94 118 65 108Q34 116 20 91Z" fill="url(#magic)"/>'
        body += '<path d="M62 50L54 69L68 80L59 96L65 108" stroke="#76569c" stroke-width="2" fill="none"/>'
        body += '<path d="M29 72Q28 57 41 56M79 56Q88 45 95 62" stroke="#eedcf4" stroke-width="5" fill="none"/>'
        body += eyes(44, 79) + eyes(86, 77)
        body += '<path d="M60 50Q48 32 52 24Q66 25 65 43Q72 27 84 32Q83 46 65 51" fill="#ba91c4" stroke-width="2"/></g>'
    else:
        body += f'<g transform="translate(0 {bob})" stroke="{INK}" stroke-width="2.7">'
        body += f'<path d="M45 86L{38-stride} 104L{53-stride} 107M76 86L{80+stride} 107L{95+stride} 104" stroke="#5f6852" stroke-width="8" fill="none"/>'
        if kind == 'healer':
            body += '<path d="M36 67Q60 52 82 66L92 97Q64 112 29 96Z" fill="#e8dcc0"/>'
            body += '<path d="M58 66L51 103L66 107L75 65" fill="#87a787" stroke="none"/>'
            body += '<path d="M37 73L23 86M84 76L96 86" fill="none" stroke="#96b786" stroke-width="8"/>'
            body += '<path d="M99 108L104 38" stroke="#9b7653" stroke-width="5"/>'
            body += '<path d="M103 39Q87 31 97 18Q115 10 118 28Q117 40 103 39Z" fill="#a9d4b4"/>'
            body += '<path d="M99 28L104 20L109 27L106 33Z" fill="#e8f6cf" stroke="none"/>'
            body += '<path d="M38 43Q38 26 61 27Q83 26 87 48L85 66Q66 79 41 66Z" fill="url(#leaf)"/>'
            body += eyes(61, 55)
            body += '<path d="M25 43Q31 19 56 18Q80 14 94 44Q59 56 25 43Z" fill="#c5b69d"/>'
            body += '<path d="M33 34Q48 27 62 32M68 28Q79 28 83 36" stroke="#f6e9cc" stroke-width="5" fill="none"/>'
            body += '<path d="M48 26L52 34L60 31L57 22Z" fill="#94ae7e" stroke-width="1.5"/>'
            body += '</g>'
            body += star(16, 53 + pose * 3, 6, '#d7ecd0', '#75a68a', 1.2)
        elif kind == 'banner':
            body += '<path d="M99 108V17" stroke="#977050" stroke-width="5"/>'
            body += f'<path d="M98 18Q110 {13-stride} 119 22L117 45Q109 {34+stride} 98 45Z" fill="#d3a868"/>'
            body += star(108, 30, 7, '#fbe3a1', '#976d4c', 1.5)
            body += '<path d="M33 66Q53 56 79 64L87 91Q59 107 30 89Z" fill="#749294"/>'
            body += '<path d="M33 70L18 81M81 71L96 76" fill="none" stroke="#97b97e" stroke-width="9"/>'
            body += '<path d="M32 41L15 31L22 54L36 59M75 42L91 29L88 56L77 60" fill="#92b177"/>'
            body += '<path d="M31 42Q34 27 54 27Q76 24 82 44L82 61Q60 80 35 63Z" fill="url(#leaf)"/>'
            body += eyes(57, 53)
            body += '<path d="M26 43L30 28L42 29L47 16L56 28L69 17L74 31L83 28L87 43Z" fill="#bb8a57"/>'
            body += '<path d="M28 75L42 63L70 65L84 77L68 82L53 74L36 84Z" fill="#e9c485"/>'
            body += '</g>'
        else:
            body += '<path d="M36 69L33 42L21 41L19 75Z" fill="url(#wood)"/>'
            body += '<path d="M35 70Q61 55 84 67L91 94Q66 109 31 94Z" fill="#a37956"/>'
            body += '<path d="M42 68L45 95M75 66L76 99" stroke="#d0b48b" stroke-width="6"/>'
            body += '<path d="M39 77L24 88M84 75L99 89" stroke="#99b680" stroke-width="8" fill="none"/>'
            body += '<path d="M88 106L111 68" stroke="#9d7957" stroke-width="5"/>'
            body += '<path d="M94 66Q107 56 118 71L112 80L99 73Z" fill="#91a5a3"/>'
            body += '<path d="M36 44L24 37L28 58L39 63M81 43L96 36L90 62L79 64" fill="#a8bd84"/>'
            body += '<path d="M35 43Q38 29 60 28Q82 27 87 44L86 66Q64 82 38 65Z" fill="url(#leaf)"/>'
            body += eyes(61, 55)
            body += '<path d="M28 43Q28 16 61 17Q91 16 94 43L92 48H28Z" fill="#ccab6f"/>'
            body += '<path d="M47 19L45 42M74 20L78 43" stroke="#edcf8c" stroke-width="4"/>'
            body += '<circle cx="61" cy="32" r="10" fill="#849a91"/><circle cx="61" cy="32" r="6" fill="#f9ecc0"/>'
            body += '<path d="M36 88H84" stroke="#6f5b4b" stroke-width="4"/><rect x="53" y="82" width="13" height="12" rx="2" fill="#dbba76"/>'
            body += '</g>'
    export(f'enemy-{kind}-{pose}', body)


def event(kind):
    body = '<circle cx="32" cy="32" r="25" fill="#f1ecd7" stroke="#7d9687" stroke-width="2"/>'
    if kind == 'fog':
        body += '<path d="M14 28Q14 22 22 23Q25 12 35 19Q46 16 47 29Q56 35 46 39H19Q8 38 14 28Z" fill="#b7c8c2" stroke="#6b8d87" stroke-width="2"/><path d="M17 44H44M23 49H49M11 39H32" stroke="#75958f" stroke-width="3"/>'
    elif kind == 'frost':
        body += '<g stroke="#609bb1" stroke-width="3" fill="none"><path d="M32 14V50M16 23L48 41M16 41L48 23M26 17L32 22L38 17M26 47L32 42L38 47M16 30L23 27L22 20M42 44L41 37L48 34M16 34L23 37L22 44M42 20L41 27L48 30"/></g><circle cx="32" cy="32" r="5" fill="url(#ice)"/>'
    elif kind == 'surge':
        body += '<path d="M16 41L27 27L21 26L38 12L35 30L43 28L29 52L30 38Z" fill="url(#gold)" stroke="#997349" stroke-width="2"/><path d="M43 16L49 13M47 27H53M15 47L11 51" stroke="#c28d50" stroke-width="2.5"/>'
    else:
        body += star(32, 31, 18, '#f5d787', '#9d8253', 2)
        body += '<path d="M15 16L12 13M49 17L52 14M15 47L12 50M49 48L52 51" stroke="#ad9162" stroke-width="2"/>'
    export('event-' + kind, body)


def choice(biome):
    base, mid, _ = PALETTE[biome]
    body = f'<rect x="6" y="6" width="244" height="148" rx="20" fill="{base}" stroke="#70857b" stroke-width="2"/>'
    body += f'<path d="M12 110Q45 79 85 111T164 105Q215 83 244 114V135Q244 148 230 148H26Q12 148 12 135Z" fill="{mid}"/>'
    if biome == 'river':
        body += '<path d="M116 8Q152 40 126 69T148 149" stroke="#76acb2" stroke-width="29" fill="none"/><path d="M116 8Q152 40 126 69T148 149" stroke="#bedbd5" stroke-width="3" fill="none"/>'
        body += '<path d="M21 117Q61 101 117 104L212 62" stroke="#b3a481" stroke-width="24" fill="none"/><path d="M21 117Q61 101 117 104L212 62" stroke="#ebd6a8" stroke-width="18" fill="none"/>'
        body += '<g transform="translate(91 47) scale(.55)">' + prop('bridge') + '</g>'
        body += '<g transform="translate(13 17) scale(.6)">' + prop('tree') + '</g><g transform="translate(166 61) scale(.55)">' + prop('reeds') + '</g>'
    elif biome == 'canyon':
        body += '<path d="M27 149Q36 107 86 87T152 12" fill="none" stroke="#b08a6b" stroke-width="32"/><path d="M27 149Q36 107 86 87T152 12" fill="none" stroke="#eed8ae" stroke-width="21"/>'
        body += '<g transform="translate(131 20) scale(.8)">' + prop('highground') + '</g><g transform="translate(11 9) scale(.55)">' + prop('highground') + '</g>'
        body += '<g transform="translate(101 65) scale(.58)">' + prop('mine') + '</g>'
    elif biome == 'snow':
        body += '<path d="M20 139Q146 138 139 79T225 14" fill="none" stroke="#a9c3c0" stroke-width="27"/><path d="M20 139Q146 138 139 79T225 14" fill="none" stroke="#edf5e9" stroke-width="18"/>'
        body += '<g transform="translate(20 15) scale(.8)">' + prop('pine') + '</g><g transform="translate(167 64) scale(.6)">' + prop('pine') + '</g>'
    elif biome == 'ruins':
        body += '<path d="M15 134L107 94L205 125L240 71" fill="none" stroke="#9fa98e" stroke-width="28"/><path d="M15 134L107 94L205 125L240 71" fill="none" stroke="#dbd4b5" stroke-width="19"/>'
        body += '<g transform="translate(28 16) scale(.8)">' + prop('gate') + '</g><g transform="translate(151 18) scale(.58)">' + prop('ruins') + '</g>'
    elif biome == 'marsh':
        body += '<ellipse cx="111" cy="103" rx="74" ry="32" fill="#83afa6"/><path d="M15 137Q116 89 157 105L234 58" stroke="#927d60" stroke-width="20" fill="none"/><path d="M15 137Q116 89 157 105L234 58" stroke="#ddc599" stroke-width="14" fill="none"/>'
        body += '<g transform="translate(15 6) scale(.75)">' + prop('reeds') + '</g><g transform="translate(145 56) scale(.65)">' + prop('reeds') + '</g>'
    else:
        body += '<path d="M16 131Q69 91 133 113T238 53" fill="none" stroke="#af9b75" stroke-width="27"/><path d="M16 131Q69 91 133 113T238 53" fill="none" stroke="#e6d19f" stroke-width="19"/>'
        body += '<g transform="translate(16 17) scale(.82)">' + prop('tree') + '</g><g transform="translate(145 4) scale(.65)">' + prop('tree') + '</g>'
        body += '<g transform="translate(110 37) scale(.64)">' + prop('watchtower') + '</g>'
    # Clip internal miniature to the rounded illustration, retaining a transparent exterior.
    body = '<defs><clipPath id="card"><rect x="7" y="7" width="242" height="146" rx="19"/></clipPath></defs><g clip-path="url(#card)">' + body + '</g><rect x="7" y="7" width="242" height="146" rx="19" fill="none" stroke="#70857b" stroke-width="2"/>'
    export('choice-' + biome, body)


def sound(name):
    rate = 22050
    duration = {'expand': 1.12, 'event': .68, 'objective': .9}[name]
    count = int(rate * duration)
    pcm = array('h')
    notes = {'expand': (261.626, 329.628, 391.995, 523.251), 'event': (349.228, 440, 349.228), 'objective': (523.251, 659.255, 783.991, 1046.502)}[name]
    step = {'expand': .19, 'event': .16, 'objective': .14}[name]
    for i in range(count):
        t, value = i / rate, 0.0
        for n, frequency in enumerate(notes):
            age = t - n * step
            if age < 0:
                continue
            envelope = min(age / .013, 1) * math.exp(-age * (6 if name == 'event' else 4.5))
            value += envelope * (math.sin(math.tau * frequency * age) + .23 * math.sin(math.tau * frequency * 2 * age) + .08 * math.sin(math.tau * frequency * 3 * age))
        fade = min(t / .01, (duration - t) / .055, 1)
        pcm.append(round(value * max(0, fade) * 5500))
    with wave.open(str(PUBLIC / 'audio' / (name + '.wav')), 'wb') as wav:
        wav.setparams((1, 2, rate, 0, 'NONE', 'not compressed'))
        wav.writeframes(pcm.tobytes())


def main():
    SOURCE.mkdir(parents=True, exist_ok=True)
    (PUBLIC / 'audio').mkdir(parents=True, exist_ok=True)
    metadata = layout()
    (PUBLIC / 'atlas.json').write_text(json.dumps(metadata, indent=2) + '\n')
    for biome in BIOMES:
        tile(biome)
        choice(biome)
    for kind in PROPS:
        export('prop-' + kind, prop(kind))
    for kind in ('healer', 'banner', 'splitter', 'engineer'):
        for pose in range(3):
            enemy(kind, pose)
    for kind in ('fog', 'frost', 'surge', 'star'):
        event(kind)
    aura = '<circle cx="64" cy="64" r="56" fill="url(#glow)"/><ellipse cx="64" cy="92" rx="49" ry="17" fill="none" stroke="#bd8db2" stroke-width="4"/><ellipse cx="64" cy="92" rx="37" ry="10" fill="none" stroke="#ebc092" stroke-width="2"/>'
    for x, y, r in ((20, 65, 8), (104, 60, 8), (64, 19, 10)):
        aura += star(x, y, r, '#e4c7ec', '#94729f', 2)
    export('effect-boss-aura', aura)
    atlas = Image.new('RGBA', (metadata['meta']['width'], metadata['meta']['height']))
    for name, frame in metadata['frames'].items():
        atlas.paste(SPRITES[name], (frame['x'], frame['y']))
    atlas.save(PUBLIC / 'atlas.webp', 'WEBP', lossless=True, method=6, exact=True)
    for name in ('expand', 'event', 'objective'):
        sound(name)
    manifest = {'version': 'world-v2', 'original': True, 'license': 'Original project artwork and procedurally synthesized audio; no third-party sources.', 'files': []}
    for path in sorted(PUBLIC.rglob('*')):
        if path.is_file():
            data = path.read_bytes()
            manifest['files'].append({'path': str(path.relative_to(PUBLIC)), 'bytes': len(data), 'sha256': hashlib.sha256(data).hexdigest()})
    (ROOT / 'manifest.json').write_text(json.dumps(manifest, indent=2) + '\n')
    out = ROOT.parents[3] / '.local/starfort/world-assets'
    out.mkdir(parents=True, exist_ok=True)
    # Large enough to inspect the actual 128px silhouettes and fine outlines.
    sheet = Image.new('RGB', (1024, 1728), '#eef0e2')
    draw = ImageDraw.Draw(sheet)
    for index, (name, im) in enumerate(SPRITES.items()):
        x, y = (index % 4) * 256, (index // 4) * 156
        thumb = im.copy()
        thumb.thumbnail((232, 128), Image.Resampling.LANCZOS)
        sheet.paste(thumb, (x + (256 - thumb.width) // 2, y + 6), thumb)
        draw.text((x + 12, y + 136), name, fill='#334a49')
    sheet.crop((0, 0, 1024, math.ceil(len(SPRITES) / 4) * 156)).save(out / 'contact-sheet.png')
    print(json.dumps({'sprites': len(SPRITES), 'files': len(manifest['files']), 'atlas': metadata['meta']}))


if __name__ == '__main__':
    main()
