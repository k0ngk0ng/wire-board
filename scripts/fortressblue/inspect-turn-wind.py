#!/usr/bin/env python3
"""Reproduce source-bound turn/wind evidence; never execute the original client."""
import hashlib
import json
from pathlib import Path
import re
import subprocess

ROOT = Path(__file__).resolve().parents[2]
OUT = ROOT / '.local/fortress-oct23'
SOURCE = ROOT / '.local/fortressblue-original/game/tankclient_emsc.eeb2b.wasm'
WABT = ROOT / '.local/fortressblue-refactor/tools/wabt-1.0.42/bin'
EXPECTED = '35974c0d7a06cf86f1f028b0ee1fc46c324921f6d829ce4db684f776199e489d'
b = SOURCE.read_bytes()
sha = lambda data: hashlib.sha256(data).hexdigest()
assert sha(b) == EXPECTED
OUT.mkdir(parents=True, exist_ok=True)

def leb(data, pos, signed=False):
    value, shift = 0, 0
    while True:
        byte = data[pos]; pos += 1
        value |= (byte & 127) << shift; shift += 7
        if not byte & 128: break
    if signed and byte & 64: value -= 1 << shift
    return value, pos

def enc(value):
    result = bytearray()
    while value > 127:
        result.append((value & 127) | 128); value >>= 7
    result.append(value)
    return result

keep = {337, 1496, 1548, 2357, 2411, 3303}
pos = 8; output = bytearray(b[:8]); functions = []; segments = []
memory = bytearray(4000000)
while pos < len(b):
    sid = b[pos]; pos += 1
    size, pos = leb(b, pos); start = pos
    payload = b[pos:pos+size]; pos += size
    if sid == 10:
        count, p = leb(payload, 0); updated = bytearray(enc(count))
        for index in range(count):
            length, p = leb(payload, p); body = payload[p:p+length]
            function = index + 283  # Source-bound original import count.
            if function in keep:
                functions.append(dict(index=function, offset=start+p, bytes=length, sha256=sha(body)))
            else:
                body = b'\x00\x00\x0b'
            p += length; updated += enc(len(body)) + body
        payload = bytes(updated)
    elif sid == 11:
        count, p = leb(payload, 0)
        for index in range(count):
            flag, p = leb(payload, p)
            if flag == 2: _, p = leb(payload, p)
            address = None
            if flag in (0, 2):
                assert payload[p] == 0x41; p += 1
                address, p = leb(payload, p, True)
                assert payload[p] == 0x0b; p += 1
            length, p = leb(payload, p)
            if address is not None:
                memory[address:address+length] = payload[p:p+length]
                segments.append((address, start+p, length))
            p += length
    output.append(sid); output += enc(len(payload)) + payload
assert len(functions) == len(keep)
slice_path = OUT/'turn-wind-slice.wasm'
slice_path.write_bytes(output)
wat_path = OUT/'turn-wind-slice.wat'
subprocess.run([str(WABT/'wasm2wat'), str(slice_path), '--enable-all', '--generate-names', '-o', str(wat_path)], check=True)
text = wat_path.read_text()
for function in sorted(keep):
    match = re.search(r'^  \(func \$f'+str(function)+r'\b.*?(?=^  \()', text, re.M | re.S)
    assert match
    (OUT/f'f{function}.wat').write_text(match[0])
strings = []
for address, expected in [(49726, 'S_PLAY_TURN'), (47472, 'S2C_PLAY_TURNSTART'), (40403, 'NowPlayer %d'), (29840, 'OnGetSPlayTurnMsg'), (40320, 'Fort->m_nTurnCnt %d')]:
    value = bytes(memory[address:]).split(b'\0',1)[0]
    assert value.decode() == expected
    segment = next(s for s in segments if s[0] <= address < s[0]+s[2])
    strings.append(dict(memoryAddress=address, fileOffset=segment[1]+address-segment[0], value=expected))
report = dict(source=str(SOURCE.relative_to(ROOT)), sourceSHA256=sha(b), derivedSHA256=sha(output), functions=functions, strings=strings,
              conclusion='Turn-start dispatch f2411 message 7310 -> f1496 receives signed wind at packet+5 and passes packet+32 to f1548 selecting current player; f2357 provides another turn-start entry.',
              limitations=['Static decoded WASM, not original C++ or observed gameplay.', 'Unselected function bodies replaced with unreachable stubs; this slice is never executed.', 'Server wind-selection probability and frequency of distinct values remain unknown.'])
(OUT/'turn-wind-evidence.json').write_text(json.dumps(report, ensure_ascii=False, indent=2)+'\n')
wat_path.unlink()  # Reproducible full slice text; focused function pages retained.
print(json.dumps(report, ensure_ascii=False, indent=2))
