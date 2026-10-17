"""Derive font/scroll fetch timing from original CPU clocks, not image pixels.
FCEUX 2.6.6's NTSC frame budget and saved CPU remainder bind the scanline origin.
Only two semantic fetch boundaries enter the renderer; no CPU/PPU runtime state.
"""
from pathlib import Path
import json,hashlib
ROOT=Path(__file__).resolve().parents[2]
def derive():
 state=ROOT/'.local/nes-basket/states/slot-2.fcs';raw=state.read_bytes();assert raw[:4]==b'FCSX' and raw[12:16]==b'\xff'*4
 fields={};position=16
 while position<len(raw):
  length=int.from_bytes(raw[position+1:position+5],'little');cursor=position+5;end=cursor+length
  while cursor+8<=end:
   name=raw[cursor:cursor+4].rstrip(b'\0').decode('ascii',errors='replace');size=int.from_bytes(raw[cursor+4:cursor+8],'little');fields[name]=raw[cursor+8:cursor+8+size];cursor+=8+size
  position=end
 base=int.from_bytes(fields['TSBS'],'little');remainder=int.from_bytes(fields['ICou'],'little',signed=True);initial_kook=fields['KOOK'][0]
 assert initial_kook in [0,1] and remainder%16==0 and fields['DEAD']==b'\0'
 base_dot=base*3+remainder//16;out=ROOT/'.local/basketball/irq-raster';cases={};sources=[]
 for file in ['clock-capture.json','holdout-timing-capture.json']:
  capture=json.loads((out/file).read_text())
  for record in capture['records']:
   run=ROOT/'.local/nes-basket/runs'/record['run'];initial=json.loads((run/'initial-clock.json').read_text());assert initial['cycles']==base
   events=[json.loads(line) for line in (run/'irq.jsonl').read_text().splitlines()]
   for item in record['frames']:
    n=item['frame'];prior=n-1;kook=initial_kook^(prior&1);prior_dots=prior*89342-(prior//2+(prior%2)*initial_kook)
    scanline_origin=base_dot+prior_dots+(22+193)*341-16-kook
    selected={e['pc']:e for e in events if e['frame']==item['emulator_frame'] and e['kind']=='write' and e['pc'] in [0xf334,0xf33e,0xf34b,0xf353]}
    assert len(selected)==4
    dots={pc:e['cycles']*3-scanline_origin for pc,e in selected.items()}
    assert dots[0xf334]<0 and dots[0xf33e]//8<=0,'Font must be loaded before the first boundary fetch'
    timing={'addressResetFetch':dots[0xf34b]//8,'fineScrollResetFetch':dots[0xf353]//8}
    assert 5<=timing['addressResetFetch']<=6 and 8<=timing['fineScrollResetFetch']<=10,timing
    key=f'{record["court"]}:{n}';entry={'court':record['court'],'frame':n,'timing':timing,'write_dots':{hex(pc):dot for pc,dot in dots.items()},'source_run':record['run']}
    if key in cases:assert cases[key]['timing']==timing
    cases[key]=entry
   sources.append({'run':record['run'],'clock_log_sha256':hashlib.sha256((run/'irq.jsonl').read_bytes()).hexdigest(),'ram_and_png_equal':record['ram_and_png_equal']})
 report={'scope':'CPU clock and saved phase derivation; no reference image data read','savestate_sha256':hashlib.sha256(raw).hexdigest(),'initial_cpu_cycles':base,'initial_cpu_remainder':remainder,'initial_kook':initial_kook,'formula':'line193_origin_dot = TSBS*3 + ICou/16 + prior_frames*89342 - skipped_odd_dots + (22+193)*341 -16 - current_kook','sources':sources,'cases':cases}
 (out/'timing.json').write_text(json.dumps(report,indent=2)+'\n');return report
if __name__=='__main__':
 r=derive();print(json.dumps({'timings':len(r['cases']),'scope':r['scope'],'fetches':sorted({tuple(c['timing'].values()) for c in r['cases'].values()})}))
