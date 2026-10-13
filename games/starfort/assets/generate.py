#!/usr/bin/env python3
"""Reproducible original vector art and synthesized sound for 星垒.

Run with Pillow and resvg installed; no downloaded art, font or samples.
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
SOURCE = ROOT / "src"
PUBLIC = ROOT / "public/starfort/v1"
INK = "#334a49"
SPRITES = {}


def star(cx, cy, radius, fill="#ffe99a", stroke="#aa7b42", sw=2):
    points = []
    for i in range(10):
        a = i * math.pi / 5 - math.pi / 2
        r = radius if i % 2 == 0 else radius * .46
        points.append(f"{cx+math.cos(a)*r:.2f},{cy+math.sin(a)*r:.2f}")
    return f'<polygon points="{" ".join(points)}" fill="{fill}" stroke="{stroke}" stroke-width="{sw}" stroke-linejoin="round"/>'


def svg(w, h, body):
    return f'''<svg xmlns="http://www.w3.org/2000/svg" width="{w}" height="{h}" viewBox="0 0 {w} {h}">
<defs>
<linearGradient id="stone" x2=".2" y2="1"><stop stop-color="#f8e9be"/><stop offset="1" stop-color="#b6a48a"/></linearGradient>
<linearGradient id="wood" x2=".3" y2="1"><stop stop-color="#e6b378"/><stop offset="1" stop-color="#9a6349"/></linearGradient>
<linearGradient id="metal" x2=".7" y2="1"><stop stop-color="#a3bbc2"/><stop offset=".5" stop-color="#607a86"/><stop offset="1" stop-color="#344c5b"/></linearGradient>
<linearGradient id="leaf" x2=".3" y2="1"><stop stop-color="#b1d07a"/><stop offset="1" stop-color="#54866a"/></linearGradient>
<linearGradient id="ice" x2=".5" y2="1"><stop stop-color="#e5ffff"/><stop offset=".45" stop-color="#97dfef"/><stop offset="1" stop-color="#4897c5"/></linearGradient>
<linearGradient id="magic" x2=".2" y2="1"><stop stop-color="#ddc8f4"/><stop offset="1" stop-color="#8265ad"/></linearGradient>
<linearGradient id="gold" x2=".4" y2="1"><stop stop-color="#fff2ab"/><stop offset=".5" stop-color="#e9bc5d"/><stop offset="1" stop-color="#b77f3f"/></linearGradient>
<radialGradient id="glow"><stop stop-color="#fff7c4" stop-opacity=".9"/><stop offset="1" stop-color="#fff7c4" stop-opacity="0"/></radialGradient>
</defs>
<g stroke-linejoin="round" stroke-linecap="round">{body}</g></svg>'''


def export(name, w, h, body, sprite=True):
    source = svg(w, h, body)
    (SOURCE / f"{name}.svg").write_text(source)
    rendered = subprocess.run(["node", str(ROOT / "rasterize.mjs"), str(SOURCE / f"{name}.svg")], check=True, capture_output=True, cwd=ROOT.parents[2])
    # 2x render followed by high quality downsampling preserves round outlines.
    image = Image.open(io.BytesIO(rendered.stdout)).convert("RGBA").resize((w, h), Image.Resampling.LANCZOS)
    image.save(PUBLIC / f"{name}.webp", "WEBP", lossless=True, method=6)
    if sprite:
        SPRITES[name] = image


def tower(kind, level):
    accents = {"arrow": "#698f6a", "cannon": "#c77d4f", "frost": "#699fb2", "arcane": "#9573ae"}
    c = accents[kind]
    body = '<ellipse cx="96" cy="207" rx="70" ry="10" fill="#19382b" opacity=".2"/>'
    # Low, warm stone pedestal with distinct hand-laid masonry.
    body += f'''<g stroke="{INK}" stroke-width="3.2">
<path d="M46 192L52 143Q94 132 139 143L147 192Q99 210 46 192Z" fill="url(#stone)"/>
<path d="M113 143L139 143L147 192L120 199Z" fill="#968c77" opacity=".65" stroke="none"/>
<path d="M48 176Q97 192 144 176M55 158Q99 172 141 157M75 165V183M117 161V184M94 187V201" fill="none" stroke="#827e69" stroke-width="2"/>
<path d="M39 190Q95 208 153 190L154 201Q94 219 38 201Z" fill="#9fac84"/>
<path d="M43 187Q94 202 149 187L153 192Q98 210 39 192Z" fill="#cfcca2"/>
<path d="M79 181V164Q96 150 111 164V181Z" fill="#4a6361"/>
<path d="M85 180V166Q96 158 105 166V180Z" fill="{c}" stroke="none"/>
<ellipse cx="96" cy="142" rx="52" ry="15" fill="#dcd3ac"/>
</g>'''
    if level >= 2:
        body += f'<g stroke="{INK}" stroke-width="2.5"><path d="M37 153L49 146L49 185L38 189Z" fill="{c}"/><path d="M143 147L155 154L154 189L144 185Z" fill="{c}"/></g>'
        body += star(96, 185, 9, sw=1.8)
    if kind == "arrow":
        body += f'''<g stroke="{INK}" stroke-width="3.5">
<path d="M58 140L62 71L127 69L134 141Q94 153 58 140Z" fill="url(#wood)"/>
<path d="M111 73L127 69L134 141L114 145Z" fill="#895b47" stroke="none"/>
<path d="M63 108L128 109M66 88L129 130M65 133L125 85" stroke="#6e5844" stroke-width="5"/>
<path d="M56 71Q98 82 137 70L133 83Q96 96 59 84Z" fill="#edc98e"/>
<path d="M48 67L93 28L145 64L138 74Q96 87 48 74Z" fill="{c}"/>
<path d="M48 67L93 28L104 38L64 72Z" fill="#a8bd79" stroke="none"/>
<path d="M57 64L89 38M78 70L99 42M103 72L108 46M122 69L119 51" stroke="#476a55" stroke-width="2"/>
<path d="M72 127V105Q87 97 100 105V127Z" fill="#3e5048"/>
<path d="M86 93V135" stroke="#ecc38b" stroke-width="5"/>
<path d="M70 113Q86 95 108 115Q89 134 70 113" fill="none" stroke="#dcac67"/>
<path d="M86 96L88 134M82 99L93 93L94 105" stroke="#eee9b7" stroke-width="2"/>
</g>'''
    elif kind == "cannon":
        body += f'''<g stroke="{INK}" stroke-width="3.5">
<path d="M45 142L49 109L59 104L69 112L81 104L93 112L106 104L118 112L131 104L143 111L146 142Q98 159 45 142Z" fill="{c}"/>
<path d="M52 127Q96 142 138 126M72 132V147M116 133V147" fill="none" stroke="#884f3d" stroke-width="2"/>
<ellipse cx="96" cy="112" rx="38" ry="14" fill="#734f42"/>
<circle cx="79" cy="110" r="21" fill="url(#wood)"/><circle cx="79" cy="110" r="11" fill="url(#gold)"/>
<path d="M82 100L108 42Q119 28 143 38Q158 44 157 59L126 112Q119 124 103 117Z" fill="url(#metal)"/>
<path d="M106 54L140 74M96 78L127 96" stroke="#c5c7ad" stroke-width="8"/>
<ellipse cx="134" cy="49" rx="22" ry="16" transform="rotate(25 134 49)" fill="#768b8b"/>
<ellipse cx="134" cy="49" rx="14" ry="9" transform="rotate(25 134 49)" fill="#263b42"/>
<path d="M123 39Q137 34 149 47" stroke="#d1dfcf" stroke-width="3" fill="none"/>
<circle cx="79" cy="110" r="6" fill="#705548"/>
</g>'''
        if level >= 2:
            body += '<g fill="#46555c" stroke="#263c44" stroke-width="2"><circle cx="45" cy="167" r="11"/><circle cx="60" cy="175" r="10"/><circle cx="43" cy="184" r="8"/></g>'
    elif kind == "frost":
        body += f'''<g stroke="{INK}" stroke-width="3">
<path d="M64 142L64 107L129 107L129 142Q96 151 64 142Z" fill="#b7d6d5"/>
<path d="M62 106Q95 93 131 106L126 119Q96 128 66 118Z" fill="#d8eddf"/>
<path d="M76 108L56 67L72 42L88 65L106 18L130 52L120 106Z" fill="url(#ice)"/>
<path d="M106 18L105 106L120 106L130 52Z" fill="#5299bf" stroke="none"/>
<path d="M56 67L88 82L72 42M88 65L106 18L105 106M130 52L106 64" fill="none" stroke="#e6ffff" stroke-width="2.5"/>
<path d="M67 134L79 130L91 137L107 130L125 135" fill="none" stroke="#6f9caa"/>
</g>'''
        body += '<g stroke="#ddffff" stroke-width="3"><path d="M46 47V63M38 55H54M140 81V95M133 88H147"/></g>'
    else:
        body += f'''<g stroke="{INK}" stroke-width="3.2">
<path d="M65 141L70 75L123 75L132 142Q98 154 65 141Z" fill="url(#magic)"/>
<path d="M113 78L122 75L132 142L113 145Z" fill="#745c94" stroke="none"/>
<path d="M67 104Q96 117 127 103M66 127Q94 140 130 127" fill="none" stroke="#c4a1d6" stroke-width="5"/>
<path d="M56 78L96 22L139 78Q103 94 56 78Z" fill="#725788"/>
<path d="M56 78L96 22L97 51L80 83Z" fill="#a17fbb" stroke="none"/>
<path d="M83 139V116Q97 99 110 117V139Z" fill="#3e3d64"/>
<ellipse cx="97" cy="63" rx="40" ry="12" transform="rotate(-25 97 63)" fill="none" stroke="#d8ba75" stroke-width="4"/>
<circle cx="97" cy="48" r="22" fill="url(#magic)"/>
<path d="M83 43Q85 32 96 32" stroke="#eee4ff" stroke-width="4" fill="none"/>
</g>'''
        body += star(98, 47, 10, "#ffe9a3", "#bd9358", 1.5)
    if level >= 2:
        body += f'<path d="M148 120V68L170 77L149 88" fill="{c}" stroke="{INK}" stroke-width="2.5"/>'
        body += star(157, 77, 4.2, sw=1)
    if level == 3:
        body += f'<path d="M29 149V57L54 66L31 82" fill="#e3bc67" stroke="{INK}" stroke-width="2.5"/>'
        body += star(38, 66, 4.2, c, INK, 1)
        body += '<path d="M41 192L55 205M149 192L137 205" stroke="#e9c874" stroke-width="5"/>'
        body += star(96, 16, 10, sw=1.8)
    export(f"tower-{kind}-{level}", 192, 224, body)


def eyes(x, y, gap=12, serious=False):
    result = f'<g fill="#253d3d"><ellipse cx="{x-gap}" cy="{y}" rx="3.4" ry="5"/><ellipse cx="{x+gap}" cy="{y}" rx="3.4" ry="5"/></g>'
    result += f'<g fill="#fff5d9"><circle cx="{x-gap+1}" cy="{y-2}" r="1.2"/><circle cx="{x+gap+1}" cy="{y-2}" r="1.2"/></g>'
    if serious:
        result += f'<path d="M{x-gap-5} {y-10}l9 3M{x+gap+5} {y-10}l-9 3" stroke="#334a49" stroke-width="3"/>'
    else:
        result += f'<path d="M{x-4} {y+10}q4 4 8 0" stroke="#39564d" stroke-width="2" fill="none"/>'
    return result


def enemy(kind, pose):
    bob = (0, -4, 1)[pose]
    stride = (0, 5, -5)[pose]
    body = '<ellipse cx="64" cy="111" rx="42" ry="7" fill="#244435" opacity=".20"/>'
    if kind in ("slime", "swarm"):
        # Three hand-authored elastic poses. Small sprout keeps the silhouette identifiable.
        height = (0, -5, 3)[pose]
        sx = (1, .92, 1.06)[pose]
        body += f'<g transform="translate(64 {108+height}) scale({sx} {1/sx}) translate(-64 -108)">'
        body += f'<path d="M23 98Q25 71 41 54Q49 41 62 44Q80 33 94 56Q106 74 105 94Q102 111 82 108Q66 116 47 109Q25 113 23 98Z" fill="url(#{"leaf" if kind == "slime" else "magic"})" stroke="{INK}" stroke-width="3"/>'
        body += '<path d="M37 76Q38 60 53 56" stroke="#e2efb3" stroke-width="7" fill="none" opacity=".85"/>'
        body += '<ellipse cx="36" cy="96" rx="7" ry="4" fill="#dcab96" opacity=".65"/><ellipse cx="88" cy="94" rx="7" ry="4" fill="#dcab96" opacity=".65"/>'
        body += eyes(64, 83, 13)
        body += f'<path d="M64 45Q52 25 42 36Q41 48 64 45M64 45Q67 21 82 24Q91 39 64 45" fill="{ "#6b985e" if kind == "slime" else "#b5a0d3"}" stroke="{INK}" stroke-width="2"/>'
        body += '</g>'
        if kind == "swarm":
            body += '<path d="M17 56l-8 8M107 46l8 9" stroke="#b08bc8" stroke-width="4"/>'
    elif kind == "runner":
        body += f'<g transform="translate(0 {bob})" stroke="{INK}" stroke-width="3">'
        body += f'<path d="M46 89L{36-stride} 104L{51-stride} 107M76 90L{87+stride} 105L{102+stride} 106" fill="none" stroke="#5f6852" stroke-width="9"/>'
        body += '<path d="M39 66Q60 52 83 62L89 88Q63 103 36 88Z" fill="#c88652"/>'
        body += '<path d="M38 70L24 83L30 87M82 68L99 81L103 77" fill="none" stroke="#92b47c" stroke-width="9"/>'
        body += '<path d="M37 44L17 32L24 57L43 64M84 45L111 31L106 57L85 64" fill="#85aa75"/>'
        body += '<path d="M27 40L33 53M102 40L96 52" stroke="#dcc09b" stroke-width="3"/>'
        body += '<path d="M36 43Q39 27 61 28Q82 25 90 44L90 67Q82 82 63 79Q39 79 34 62Z" fill="url(#leaf)"/>'
        body += '<path d="M35 43Q46 9 76 21L91 44Z" fill="#668789"/><path d="M41 34L33 18L46 24" fill="#dda667"/>'
        body += '<path d="M30 80L43 68L80 69L96 79L84 87L61 77L40 88Z" fill="#e5b567"/>'
        body += eyes(63, 56, 12, True) + '</g>'
    elif kind in ("armored", "elite", "boss"):
        elite = kind in ("elite", "boss")
        body += f'<g transform="translate(0 {bob})" stroke="{INK}" stroke-width="3">'
        body += f'<path d="M40 88L{36-stride} 104L{54-stride} 109L61 95M71 94L{75+stride} 109L{94+stride} 103L87 85" fill="#7d8380"/>'
        body += '<path d="M37 58L23 68L19 89L32 96L43 78M88 58L104 66L111 89L97 98L85 79" fill="url(#stone)"/>'
        body += f'<path d="M40 52L85 47L97 83L79 102L45 100L32 79Z" fill="url(#{"magic" if elite else "metal"})"/>'
        body += '<path d="M45 58L63 51L81 59L80 82L64 94L48 83Z" fill="#acbcb8"/>'
        body += '<path d="M36 35L49 20L79 20L91 34L86 59L72 70L50 65L36 54Z" fill="url(#stone)"/>'
        body += '<path d="M49 21L53 38L43 50M78 22L68 36L82 46L75 63" fill="none" stroke="#929584" stroke-width="2"/>'
        body += '<path d="M45 41L58 44L57 51L46 50M69 45L82 41L80 51L70 52" fill="#3c6266" stroke="none"/>'
        body += '<path d="M47 46L55 48M72 48L79 46" stroke="#d9eea6" stroke-width="3"/>'
        body += '<path d="M53 58L72 58" stroke="#576762" stroke-width="2"/>'
        body += star(64, 74, 10, "#ccf0d5" if not elite else "#ead386", "#607a73", 2)
        if elite:
            body += '<path d="M34 31L27 10L49 22L63 9L79 22L98 10L94 32Z" fill="url(#gold)"/>'
            body += '<path d="M30 77L23 62L18 72M98 77L108 62L113 74" fill="#d6b978"/>'
        body += '</g>'
        if kind == "boss":
            cape = f'<path d="M42 {42+bob}Q17 51 12 106L38 99L62 112L90 101L114 108Q108 53 83 {42+bob}Z" fill="#a7707a" stroke="{INK}" stroke-width="3"/><path d="M24 74L22 97M101 75L105 98" stroke="#d5a29a" stroke-width="3"/>'
            body = '<g transform="translate(2 0) scale(1.22)">' + cape + body + star(64, 15+bob, 7, "#fbe8a1", "#907343", 1.5) + '</g>'
    else:  # shield bearer, a mushroom sentry with a translucent rune shell.
        body += f'<g transform="translate(0 {bob})" stroke="{INK}" stroke-width="3">'
        body += f'<path d="M48 96L{42-stride} 108M77 95L{86+stride} 107" stroke="#576b77" stroke-width="10"/>'
        body += '<path d="M42 60L83 60L92 99Q66 111 37 98Z" fill="#e1c698"/>'
        body += '<path d="M20 65Q20 31 59 24Q100 22 109 65Q65 81 20 65Z" fill="#839bc3"/>'
        body += '<path d="M31 52Q36 37 53 35" stroke="#b9cee6" stroke-width="6" fill="none"/>'
        body += '<g fill="#d6e4e1" stroke="none"><ellipse cx="68" cy="43" rx="9" ry="7"/><ellipse cx="92" cy="58" rx="7" ry="5"/><ellipse cx="39" cy="59" rx="6" ry="4"/></g>'
        body += eyes(65, 86, 10)
        body += '<path d="M29 76L53 69L54 89Q53 104 41 111Q24 103 24 91Z" fill="#8bc9de" stroke="#4e7e99"/>'
        body += star(40, 89, 8, "#e3ffff", "#6f9ea9", 1.5) + '</g>'
    size = 160 if kind == "boss" else 128
    export(f"enemy-{kind}-{pose}", size, size, body)


def base(damaged=False):
    b = '<ellipse cx="112" cy="204" rx="91" ry="13" fill="#264936" opacity=".2"/>'
    b += f'''<g stroke="{INK}" stroke-width="3.5">
<path d="M43 170L174 171L195 196L178 209L47 209L28 196Z" fill="#92a580"/>
<path d="M53 86L164 86L169 193Q111 210 51 193Z" fill="url(#stone)"/>
<path d="M148 91L164 86L169 193L144 199Z" fill="#a6a084" stroke="none"/>
<path d="M52 151Q110 165 168 150M53 175Q111 190 169 174M83 156V181M142 158V182" fill="none" stroke="#9b937a" stroke-width="2"/>
<path d="M78 195V165Q110 120 145 165V196" fill="#51686b"/>
<path d="M92 194V166Q112 145 132 166V195" fill="#334b56"/>
<path d="M37 184V98L51 91L68 98V191M155 192V99L174 91L189 99V184" fill="#c8bea0"/>
<path d="M33 101V82L43 81V90H52V77H62V89H72V82L77 87V101Q55 111 33 101M148 101V83L158 81V91H168V77H178V88H191V100Q174 110 148 101Z" fill="#e3d6b4"/>
<path d="M43 79L107 28L177 80L165 98Q111 108 52 97Z" fill="#658a86"/>
<path d="M43 79L107 28L99 62L67 98L52 97Z" fill="#98b4a1" stroke="none"/>
<path d="M51 84Q114 104 168 84M67 69Q114 85 152 67M83 55Q108 65 134 52" fill="none" stroke="#486d70" stroke-width="2"/>
<path d="M111 30V13L148 23L113 34" fill="#d9b66c"/>
<circle cx="111" cy="103" r="28" fill="#486d73"/><circle cx="111" cy="103" r="22" fill="url(#gold)"/>
<path d="M43 143V126Q53 115 61 126V143M165 143V126Q175 115 182 126V143" fill="#526b6a"/>
</g>'''
    b += star(111, 102, 16, "#fff6ba", "#a88449", 2)
    b += '<path d="M104 83Q91 87 91 99" stroke="#fffbd9" stroke-width="3" fill="none"/>'
    if damaged:
        b += '<g stroke="#776b64" stroke-width="4" fill="none"><path d="M77 113l10 18-7 14 12 13M155 116l-14 12 9 17M50 175l10 4 8-11"/></g>'
        b += '<path d="M119 33L132 54L121 68L149 66L139 51Z" fill="#ad9a80" stroke="#596967" stroke-width="3"/>'
        b += '<g fill="#bd9667" stroke="#776f5e" stroke-width="2"><path d="M35 202l6-12 10 7-3 10Z"/><path d="M174 201l7-12 13 10-8 9Z"/></g>'
    export("base-damaged" if damaged else "base-intact", 224, 224, b)


def projectiles():
    export("projectile-arrow", 64, 64, f'<g transform="rotate(45 32 32)" stroke="{INK}" stroke-width="2"><path d="M32 12V53" stroke="#cba365" stroke-width="5"/><path d="M32 8L23 26L32 23L41 26Z" fill="#cee0d3"/><path d="M29 43L22 48L24 57L31 52M35 43L42 48L40 57L33 52" fill="#7da66d"/></g>')
    export("projectile-cannon", 64, 64, '<path d="M21 43L7 52L13 34" fill="#d7ad61" opacity=".7"/><circle cx="35" cy="29" r="19" fill="url(#metal)" stroke="#324c54" stroke-width="3"/><path d="M25 22Q27 15 36 16" fill="none" stroke="#c2d5d2" stroke-width="4"/>')
    export("projectile-frost", 64, 64, '<path d="M16 45L29 13L46 8L52 26L30 48L13 53Z" fill="url(#ice)" stroke="#598b9e" stroke-width="2"/><path d="M15 50L39 22L46 8M29 13L39 22L52 26" fill="none" stroke="#e7ffff" stroke-width="2"/>')
    export("projectile-arcane", 64, 64, '<circle cx="34" cy="29" r="25" fill="#a986d0" opacity=".18"/><circle cx="34" cy="29" r="17" fill="url(#magic)" stroke="#73568e" stroke-width="2"/>' + star(34, 29, 13, "#ffeab7", "#af83b4", 1.5) + '<path d="M11 45L6 55M17 48L15 58" stroke="#c6a3d9" stroke-width="3"/>')
    b = '<g stroke="#bf7d46" stroke-width="2.5"><path d="M65 14L74 37L96 25L89 49L117 58L92 70L106 94L80 89L70 115L56 93L32 108L37 82L11 70L36 57L22 32L49 39Z" fill="#eeb963"/><path d="M64 37L75 51L92 60L77 74L64 93L51 74L33 65L51 54Z" fill="#ffed9a" stroke="none"/></g>'
    export("effect-burst", 128, 128, b)
    frost = '<circle cx="64" cy="64" r="43" fill="#a0e0e5" opacity=".20"/>'
    for angle in range(0, 360, 60):
        frost += f'<path d="M64 17V111M64 33L53 24M64 33L75 24M64 95L53 104M64 95L75 104" transform="rotate({angle} 64 64)" stroke="#d9ffff" stroke-width="4" fill="none"/>'
    export("effect-frost", 128, 128, frost)
    magic = '<circle cx="64" cy="64" r="43" fill="none" stroke="#ac8bc9" stroke-width="6"/><circle cx="64" cy="64" r="33" fill="#c6a3e3" opacity=".22"/>'
    for i in range(5):
        a = i * math.tau / 5
        magic += star(64+math.cos(a)*42, 64+math.sin(a)*42, 9, "#f7dfac", "#a585ba", 1.8)
    export("effect-arcane", 128, 128, magic)
    export("coin", 64, 64, '<ellipse cx="32" cy="34" rx="23" ry="23" fill="#bc8b42" stroke="#886b45" stroke-width="2"/><circle cx="32" cy="29" r="23" fill="url(#gold)" stroke="#a17a42" stroke-width="2"/><circle cx="32" cy="29" r="17" fill="none" stroke="#bf9349" stroke-width="2"/>' + star(32, 29, 12, "#fff0ab", "#b18a4c", 1.4))


def tree(x, y, scale=1):
    return f'''<g transform="translate({x} {y}) scale({scale})" stroke="#426755" stroke-width="2.5">
<ellipse cx="0" cy="40" rx="26" ry="8" fill="#467955" opacity=".18" stroke="none"/>
<path d="M-6 3L-8 35Q0 41 9 34L6 3Z" fill="#a28058"/>
<path d="M-2 15L-14 7M2 23L12 12" fill="none" stroke="#715f45" stroke-width="2"/>
<path d="M-29 3Q-36-12-23-21Q-27-37-12-40Q1-55 15-37Q33-34 29-17Q44-2 30 9Q14 24-3 14Q-23 22-29 3Z" fill="url(#leaf)"/>
<path d="M-19-19Q-21-31-9-32M-9-38Q1-47 10-33M10-24Q20-24 20-14" fill="none" stroke="#bfd590" stroke-width="4"/>
<path d="M-23 2Q-13 11-3 4M11 8Q24 12 29 0" fill="none" stroke="#6a956e" stroke-width="2"/>
</g>'''


def map_art():
    b = '<rect width="960" height="640" fill="#c6d4a8"/><path d="M0 0H960V70Q700 116 463 59T0 72Z" fill="#b5c99d"/><path d="M0 563Q197 535 358 595T960 566V640H0Z" fill="#b4c697"/>'
    rng = random.Random(8411)
    for _ in range(380):
        x, y = rng.randrange(8, 950), rng.randrange(8, 635)
        b += f'<path d="M{x} {y}l-2-4m2 4l3-3" fill="none" stroke="{rng.choice(("#b2c798", "#a9bf8c", "#d4deb8"))}" stroke-width="1.6" opacity=".65"/>'
    for x, y, scale in ((46, 32, 1.4),(120, 15, 1.1),(819, 26, 1.5),(907, 54, 1.2),(30, 330, 1.2),(44, 422, 1.3),(889, 265, 1.1),(940, 201, 1.3),(90, 590, 1.4),(186, 624, 1.2),(395, 611, 1.3),(507, 630, 1.4),(875, 618, 1.3)):
        b += tree(x, y, scale)
    for x, y in ((123, 405),(575, 68),(903, 357),(373, 480),(801, 112)):
        b += f'<g transform="translate({x} {y})" stroke="#869c7e" stroke-width="2"><ellipse cx="0" cy="7" rx="20" ry="7" fill="#719673" opacity=".18" stroke="none"/><path d="M-18 4L-11-9L3-13L17-3L15 9L-4 13Z" fill="#aeb9a0"/><path d="M-11-9L3-5L17-3M3-5L-4 13" fill="none" stroke="#cdd2b9"/></g>'
    for x, y in ((177, 53),(67, 214),(397, 41),(534, 579),(937, 413),(797, 584)):
        b += f'<g transform="translate({x} {y})"><path d="M0 7V-4M0 3L-5-1" stroke="#729865" stroke-width="2"/><g fill="#efd5a0"><circle cx="0" cy="-7" r="3"/><circle cx="-4" cy="-4" r="3"/><circle cx="4" cy="-4" r="3"/><circle cx="0" cy="0" r="3"/></g><circle cx="0" cy="-4" r="2" fill="#c39c5d"/></g>'
    export("map", 960, 640, b, False)


def pack_atlas():
    width = 1024
    x = y = row_height = 0
    frames = {}
    for name, image in SPRITES.items():
        if x + image.width + 4 > width:
            x, y, row_height = 0, y + row_height + 4, 0
        frames[name] = {"x": x, "y": y, "w": image.width, "h": image.height}
        x += image.width + 4
        row_height = max(row_height, image.height)
    height = y + row_height
    atlas = Image.new("RGBA", (width, height))
    for name, image in SPRITES.items():
        f = frames[name]
        atlas.paste(image, (f["x"], f["y"]))
    atlas.save(PUBLIC / "atlas.webp", "WEBP", lossless=True, method=6)
    (PUBLIC / "atlas.json").write_text(json.dumps({"frames": frames, "meta": {"image": "atlas.webp", "width": width, "height": height}}, indent=2) + "\n")


def audio():
    output = PUBLIC / "audio"
    output.mkdir(parents=True, exist_ok=True)
    rate = 22050
    # All notes and noise are original, deterministic, band-limited by soft envelopes.
    melodies = {
        "build": [(0, .13, 523), (.10, .19, 784)],
        "upgrade": [(0, .13, 523), (.10, .13, 659), (.20, .13, 784), (.30, .3, 1047)],
        "sell": [(0, .12, 1047), (.09, .12, 784), (.18, .22, 659)],
        "wave": [(0, .22, 392), (.24, .22, 523), (.48, .4, 784)],
        "victory": [(0, .18, 523), (.18, .18, 659), (.36, .18, 784), (.54, .65, 1047), (.54, .65, 659), (.54, .65, 784)],
        "defeat": [(0, .25, 392), (.25, .25, 349), (.5, .65, 262)],
    }
    specs = {"arrow":(.12, 900, 180, .30), "cannon":(.34, 140, 35, .60), "frost":(.32, 1450, 650, .15), "arcane":(.30, 340, 880, .08), "hit":(.08, 240, 100, .5), "leak":(.45, 250, 120, .15)}
    for index, name in enumerate((*melodies, *specs)):
        rng = random.Random(581 + index)
        if name in melodies:
            notes = melodies[name]
            duration = max(start + length for start, length, _ in notes) + .02
        else:
            duration = specs[name][0] + .02
        samples = []
        low_noise = 0.0
        for frame in range(round(duration * rate)):
            t = frame / rate
            value = 0.0
            if name in melodies:
                for start, length, freq in notes:
                    u = t - start
                    if 0 <= u < length:
                        env = min(1, u / .008) * ((1 - u / length) ** 1.7)
                        value += .2 * env * (math.sin(math.tau * freq * u) + .25 * math.sin(math.tau * freq * 2 * u))
            else:
                length, f0, f1, noise = specs[name]
                if t < length:
                    env = min(1, t / .006) * ((1 - t / length) ** 2)
                    phase = math.tau * (f0 * t + (f1 - f0) * t * t / (2 * length))
                    low_noise = .65 * low_noise + .35 * rng.uniform(-1, 1)
                    value = .43 * env * ((1-noise) * math.sin(phase) + noise * low_noise)
            samples.append(round(max(-.9, min(.9, value)) * 32767))
        with wave.open(str(output / f"{name}.wav"), "wb") as wav:
            wav.setnchannels(1)
            wav.setsampwidth(2)
            wav.setframerate(rate)
            wav.writeframes(array("h", samples).tobytes())


def main():
    SOURCE.mkdir(parents=True, exist_ok=True)
    PUBLIC.mkdir(parents=True, exist_ok=True)
    for kind in ("arrow", "cannon", "frost", "arcane"):
        for level in (1, 2, 3):
            tower(kind, level)
    for kind in ("slime", "runner", "armored", "swarm", "shield", "elite", "boss"):
        for pose in (0, 1, 2):
            enemy(kind, pose)
    base()
    base(True)
    projectiles()
    map_art()
    pack_atlas()
    audio()
    files = []
    for path in sorted(PUBLIC.rglob("*")):
        if path.is_file():
            content = path.read_bytes()
            files.append({"path": str(path.relative_to(PUBLIC)), "bytes": len(content), "sha256": hashlib.sha256(content).hexdigest()})
    (ROOT / "manifest.json").write_text(json.dumps({"version": 1, "attribution": "Original SVG illustrations and synthesized audio created for wire-board / 星垒. No external imagery, fonts, samples or generative-image models.", "source": "games/starfort/assets/generate.py", "files": files}, ensure_ascii=False, indent=2) + "\n")
    # Contact sheet retained alongside sources for manual visual review.
    sheet = Image.new("RGB", (1024, 2000), "#e2e8d5")
    draw = ImageDraw.Draw(sheet)
    y = 0
    x = 0
    row = 0
    for name, image in SPRITES.items():
        if x + 200 > sheet.width:
            x, y, row = 0, y + row + 28, 0
        thumb = image.copy()
        thumb.thumbnail((180, 180))
        sheet.paste(thumb, (x + 10, y + 5), thumb)
        draw.text((x + 10, y + 185), name, fill="#334a49")
        x += 204
        row = 180
    sheet.crop((0, 0, sheet.width, y + row + 28)).save(ROOT / "contact-sheet.webp", lossless=True)
    print(json.dumps({"sprites": len(SPRITES), "files": len(files), "bytes": sum(item["bytes"] for item in files)}))


if __name__ == "__main__":
    main()
