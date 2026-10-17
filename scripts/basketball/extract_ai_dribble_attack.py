"""Keep every original mode-16/17 decision, without filtering by supported paths."""
import csv
import hashlib
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]

def main():
    run = ROOT / '.local/nes-basket/runs/ai-shot-observation'
    raw = (run / 'ai-ram.bin').read_bytes()
    rows = list(csv.DictReader((run / 'ai.csv').open()))
    assert len(raw) == len(rows) * 4096
    def decode(r, actor):
        def pos(i):
            return dict(x=r[0x68+i]+256*r[0x75+i]+65536*r[0x82+i], depth=r[0x8f+i]+256*r[0x9c+i], height=r[0xa9+i]+256*r[0xb6+i]+65536*r[0xc3+i])
        return dict(mode=r[0x672+actor], nextMode=r[0x676+actor], counter=r[0x66e+actor], parameter=r[0x682+actor], target=r[0x67a+actor], destination=r[0x67e+actor], zone=r[0x66a+actor], action=r[0x512+actor], request=r[0x51f+actor], heading=r[0x4ec+actor], facing=r[0x46b+actor], holder=r[0x53f], random=list(r[0x48:0x4c]), players=[pos(i) for i in range(4)], court=r[0x58b], difficulty=r[0x58c], teamHasHuman=(r[0x539+(actor&2)]&r[0x53a+(actor&2)]&128)==0, **pos(actor))
    result=[]
    for i,row in enumerate(rows):
        actor=int(row['actor']); before=decode(raw[i*4096:i*4096+2048],actor)
        if before['mode']&127 not in (16,17): continue
        result.append(dict(actor=actor,frame=int(row['frame']),before=before,after=decode(raw[i*4096+2048:(i+1)*4096],actor)))
    path=ROOT/'games/basketball/engine/testdata/ai-dribble-attack.json'
    path.write_text(json.dumps(result,separators=(',',':'))+'\n')
    path.with_name('ai-dribble-attack-source.json').write_text(json.dumps(dict(source_is_original_execution=True,records=len(result),trace_sha256=hashlib.sha256(raw).hexdigest(),sha256=hashlib.sha256(path.read_bytes()).hexdigest(),source=json.loads((run/'source.json').read_text())),indent=2)+'\n')
    print(len(result))

if __name__ == '__main__': main()
