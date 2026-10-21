#!/usr/bin/env python3
"""Verify retained weather evidence against original byte ranges; no execution."""
import hashlib
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[3]
BASE = ROOT / '.local/fortress-weather'
sha = lambda b: hashlib.sha256(b).hexdigest()
provenance = json.loads((BASE/'evidence/slice-provenance.json').read_text())
source = (ROOT/provenance['source']).read_bytes()
assert sha(source) == provenance['sourceSHA256']
derived = (BASE/'weather-slice.wasm').read_bytes()
assert sha(derived) == provenance['derivedSHA256']

def leb(data, pos):
    value, shift = 0, 0
    while True:
        assert pos < len(data) and shift < 70
        byte = data[pos]; pos += 1
        value |= (byte & 127) << shift
        if not byte & 128:
            return value, pos
        shift += 7

bodies = {}
p = 8
while p < len(derived):
    sid = derived[p]; p += 1
    size, p = leb(derived, p)
    end = p + size
    assert end <= len(derived)
    if sid == 10:
        count, q = leb(derived, p)
        for i in range(count):
            n, q = leb(derived, q)
            assert q+n <= end
            bodies[i+283] = derived[q:q+n]
            q += n
        assert q == end
    p = end
assert p == len(derived)
keep = {r['index'] for r in provenance['retainedFunctions']}
for row in provenance['retainedFunctions']:
    body = source[row['offset']:row['offset']+row['bytes']]
    assert sha(body) == row['sha256']
    assert body == bodies[row['index']]
for index, body in bodies.items():
    assert index in keep or body == b'\x00\x00\x0b'
rea = json.loads((BASE/'evidence/rea-weather-slice.json').read_text())
assert rea['subject']['digest']['sha256'] == sha(derived)
assert rea['evidence_id'] == 'ev_c1d72b458cd9648f4b40c9da03b04073df17033188ac56404dcc1a9e0d7c7f88'
for name in ['portal-weather-items.json','original-item-lines.json']:
    data = json.loads((BASE/'evidence'/name).read_text())
    assert sha((ROOT/data['source']).read_bytes()) == data['sourceSHA256']
rain = json.loads((BASE/'evidence/rain-resource.json').read_text())
assert sha((ROOT/rain['source']).read_bytes()) == rain['sha256']
assert sha((BASE/'evidence/rain-resource.png').read_bytes()) == rain['previewSha256']
report = {'status':'passed','sourceSHA256':sha(source),'derivedSHA256':sha(derived),
          'retainedFunctionCount':len(keep),'byteIdenticalToOriginal':True,
          'otherBodiesAreUnreachableStubs':True,'reaEvidenceID':rea['evidence_id'],
          'runtimeExecuted':False,'sourceTextsAndSnowflakePreviewDigestsVerified':True}
(ROOT/'docs/research/fortress-weather/verification.json').write_text(json.dumps(report,indent=2)+'\n')
print(json.dumps(report))
