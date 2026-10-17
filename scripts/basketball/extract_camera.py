"""Project original complete camera/render stages into compact typed fixtures."""
from pathlib import Path
import csv
import gzip
import hashlib
import json

ROOT = Path(__file__).resolve().parents[2]


def main():
    source = ROOT/'.local/nes-basket/runs/camera-cpu-long'
    dest = ROOT/'games/basketball/engine/testdata'

    def state(r):
        def word(lo,hi): return r[lo] + 256*r[hi]
        def signed24(v): return v if v < 0x800000 else v-0x1000000
        def signed16(v): return v if v < 0x8000 else v-0x10000
        positions = [dict(x=signed24(word(0x68+i,0x75+i)+65536*r[0x82+i]),
                          depth=word(0x8f+i,0x9c+i),
                          height=signed24(word(0xa9+i,0xb6+i)+65536*r[0xc3+i])) for i in range(13)]
        projected = [dict(x=signed16(word(0x3e7+i,0x3f4+i)), y=signed16(word(0x401+i,0x40e+i))) for i in range(13)]
        return dict(camera=dict(displayedX=word(0x397,0x398),x=word(0x39b,0x39c),y=word(0x39d,0x39e),
                                tileColumn=r[0x3a1],shake=r[0x6d5]),
                    context=dict(trackedObject=r[0x3a2],ballHolder=r[0x53f],positions=positions,projected=projected),
                    projection=dict(projected=projected,drawOrder=list(r[0x41d:0x42a]),drawCount=r[0x41c]),
                    projectionContext=dict(cameraX=word(0x39b,0x39c),cameraY=word(0x39d,0x39e),
                                           layoutID=r[0x588],poses=list(r[0x5b:0x68]),positions=positions),
                    crowding=r[0x6d6])

    for name,count in [('camera',1997),('crowding',1997),('projection',1998)]:
        raw = (source/f'{name}-ram.bin').read_bytes()
        rows = list(csv.DictReader((source/f'{name}.csv').open()))
        assert len(rows) == count and len(raw) == count*4096
        result = []
        for i,row in enumerate(rows):
            before = state(raw[i*4096:i*4096+2048])
            after = state(raw[i*4096+2048:(i+1)*4096])
            if name == 'camera':
                record = dict(before=before['camera'],after=after['camera'],context=before['context'])
            elif name == 'projection':
                record = dict(before=before['projection'],after=after['projection'],context=before['projectionContext'])
                assert row['objects'] == ';'.join(map(str,range(13)))
            else:
                record = dict(before=before['crowding'],after=after['crowding'],projected=before['context']['projected'][:4])
            result.append(dict(frame=int(row['frame']),**record))
        output = dest/f'{name}-stages.json.gz'
        output.write_bytes(gzip.compress((json.dumps(result,separators=(',',':'))+'\n').encode(),mtime=0))
        output.with_name(f'{name}-stages-source.json').write_text(json.dumps(dict(
            source_is_original_execution=True,records=count,sha256=hashlib.sha256(output.read_bytes()).hexdigest(),
            trace_sha256=hashlib.sha256(raw).hexdigest(),source=json.loads((source/'source.json').read_text())),indent=2)+'\n')
        print(name,count,output.stat().st_size)


if __name__ == '__main__': main()
