"""Original BD8E phase calls up to BE63 (before three actor updates).

Failure inventory: repeated award; full inventory overwrite; skipped entry;
timer wrap/112 exit; message order at40; RNG ordering and early input exits;
restoring wrong characters; changing unrelated transition/progress fields.
These are controlled CPU calls, not earned victories or video-timing evidence.
"""
import argparse, base64, gzip, hashlib, json
from verify_campaign_settlement import ROOT, OUT, ROM, call

def main():
    parser=argparse.ArgumentParser(); parser.add_argument('--check',action='store_true'); args=parser.parse_args()
    source=OUT/'campaign-settlement-cases.json.gz'
    cases=json.loads(gzip.decompress(source.read_bytes()))['cases']
    rows=[]
    for case in cases:
        if case['nextStage']!='prepare': continue
        raw=bytearray(base64.b64decode(case['phases'][1]['rawAfter']))
        raw[4]=0
        after,record=call(raw,4,0xbd8e,{0xbe63},'reward-entry')
        rows.append(record)
        # Distinct timer boundaries, input gates, and nonzero/zero RNG states.
        if case['label']!='0-0-1-0-empty-True': continue
        for timer in (0,38,39,40,110,111,112,254,255):
            for buttons in (0,0x10,0x80,0x40,0x90):
                for seed in (0,0x12345678,0xffffffff):
                    r=bytearray(after); r[0x3c3]=timer; r[4]=buttons
                    r[0x48:0x4c]=seed.to_bytes(4,'big')
                    _,record=call(r,4,0xbd8e,{0xbe63},'reward-repeat')
                    rows.append(record)
    payload=gzip.compress(json.dumps(rows,separators=(',',':')).encode(),mtime=0)
    path=OUT/'reward-phase.json.gz'
    meta=dict(calls=len(rows),romSHA256=hashlib.sha256(ROM).hexdigest(),inputSHA256=hashlib.sha256(source.read_bytes()).hexdigest(),fixtureSHA256=hashlib.sha256(payload).hexdigest(),scope=__doc__)
    if args.check: assert path.read_bytes()==payload
    else:
        path.write_bytes(payload)
        (OUT/'reward-phase-source.json').write_text(json.dumps(meta,indent=2)+'\n')
    print(json.dumps(meta))

if __name__=='__main__': main()
