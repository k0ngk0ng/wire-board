"""Retain the full natural input phase exposing the original Y-index alias."""
import argparse,csv,gzip,hashlib,json
from pathlib import Path
from extract_world_inputs import decode_world_input_state
from extract_world_preparation import context as preparation_context

ROOT=Path(__file__).resolve().parents[2]


def main():
    parser=argparse.ArgumentParser();parser.add_argument('--index',type=int,choices=[22,54],default=22);args=parser.parse_args()
    run=ROOT/'.local/nes-basket/runs/world-rider-natural';raw=(run/'world-inputs-ram.bin').read_bytes();rows=list(csv.DictReader((run/'world-inputs.csv').open()));index=args.index
    assert len(raw)==len(rows)*4096
    b,a=raw[index*4096:index*4096+2048],raw[index*4096+2048:(index+1)*4096]
    frames=[]
    for actor in range(4):
        slot=b[0x4e8+actor];frames.append(dict(held=b[slot],pressed=b[slot+4]) if slot<4 else dict(held=0,pressed=0))
    record=dict(frame=int(rows[index]['frame']),before=decode_world_input_state(b),after=decode_world_input_state(a),calls=list(range(10)),context=dict(world=preparation_context(b)['actions'],frames=frames,difficulty=b[0x58c],scoreDifference=b[0x696],cachedZones=list(b[0x66a:0x66e]),objectKinds=list(b[0x3d6:0x3e3]),hoopLinks=list(b[0x649:0x64f]),humanTactics=list(b[0x666:0x668])),pickupFlags=list(b[0x634:0x638]),jumpEnhancements=[b[0x6e6+3*b[0x3e3+i]] for i in range(4)],probe=dict(action=b[0x531]&127,x=(b[0x94]+256*b[0xa1])*256,depth=b[0xbb]*256))
    stem='pursuit-alias-input'
    if index==54:
        stem='pursuit-alias74-input'
        record['context']['pursuitAlias74']=dict(actionByte=b[0x55c],depthByte=b[0xe6])
        record['probe']=dict(action=b[0x55c]&127,x=(b[0xbf]+256*b[0xcc])*256,depth=b[0xe6]*256)
    payload=(json.dumps(record,separators=(',',':'))+'\n').encode();packed=gzip.compress(payload,mtime=0)
    out=ROOT/'games/basketball/engine/testdata';p=out/(stem+'.json.gz');p.write_bytes(packed)
    p.with_name(stem+'-source.json').write_text(json.dumps(dict(source_is_original_execution=True,index=index,frame=record['frame'],records=1,calls_source='fixed D32C actor loop then D33D objects4..9; no per-call timestamps in this capture',uncompressed_sha256=hashlib.sha256(payload).hexdigest(),sha256=hashlib.sha256(packed).hexdigest(),trace_sha256=hashlib.sha256(raw).hexdigest(),source=json.loads((run/'source.json').read_text())),indent=2)+'\n')
    print(stem,record['frame'],record['probe'])


if __name__=='__main__':main()
