"""Retain the actual producer/IRQ context of bounded fixed-barrier failures."""
from pathlib import Path
import gzip, hashlib, json
ROOT=Path(__file__).resolve().parents[2]
OUT=ROOT/'games/yuyuhakusho/verification/world-audio-barriers'
reports=[]
for kind in ['baseline','pause']:
    path=ROOT/'docs/research/md-yuyuhakusho/audio-phase-budget/compact'/f'audio-phase-{kind}.json.gz'
    events=json.loads(gzip.decompress(path.read_bytes()))['events']
    for frame,code,producer in [(2071,0x7c,0x8100),(2396,0x7d,0x8400)]:
        selected=[e for e in events if e[5]==frame]
        writes=[e for e in selected if e[0]==2 and (e[1]&0xffffff) in [0x2196c,0x2197e,0x21990,0x219a2] and e[4]==code]
        assert len(writes)==1 and writes[0][8]==producer
        starts=[e for e in selected if e[0]==1 and (e[1]&0xffffff)==0x375a6]
        assert len(starts)==1
        irq=[e for e in selected if e[0]==3 and (e[1]&0xffffff)==0xfff000 and e[6]<starts[0][6]]
        assert irq
        def record(e):return {'kind':e[0],'pc':hex(e[1]&0xffffff),'cycle':e[6],'object':hex(e[8]),'behavior':hex(e[9]),'instrumentation_depth':e[10],'watched_state':e[12:19]}
        reports.append({'scenario':kind,'video_step_label':frame,'code':hex(code),'source_sha256':hashlib.sha256(path.read_bytes()).hexdigest(),
                        'producer_store':record(writes[0]),'ram_irq_entry':record(irq[-1]),'driver_entry':record(starts[0]),
                        'native_selected':next(e[14]for e in selected if e[0]==1 and(e[1]&0xffffff)==0x37cb4)})
result={'scope':'Observed source context only; cycle labels and instrumentation depth are not a production schedule or an independently inferred hardware nesting model','cases':reports}
(OUT/'counterexample-context.json').write_text(json.dumps(result,indent=2)+'\n')
print(json.dumps({'pass':True,'cases':len(reports)}))
