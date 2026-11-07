"""Check named semantic projections against observed full-frame work values.

Failures: query/packing byte roles swapped; missing row scans and support
restores; odd-column partial write; player overflow incorrectly copied from
entity overflow; indoor perspective low byte omitted; stale graphics address;
wrong same-frame order; unnoticed writes between C001 frames. Raw writes are
only coverage evidence, never replayed as the model's state transitions.
"""
from pathlib import Path
import argparse,gzip,json
ROOT=Path(__file__).resolve().parents[2]
KINDS={'support-query','query-coordinate','query-prepared-x','query-page-toggle','support-index','read-index','stream-column','stream-row','graphic-location','player-perspective','weapon-perspective','entity-probe','collision-clear','guardian-pair-clear','guardian-row-shift'}

def query(x,y,c):
    sy=y+c['scrollY'];row=sy&255
    if sy>=240:row=(row+16)&255
    sx=x+c['scrollX'];page=(c['control']^c.get('toggle',0))&1
    if sx>=256:page^=1
    column=(sx&255)>>4;index=((row>>4)*4)|(column>>2)|(page<<6)
    return (column&3)|(index<<8)

def project(value,o):
    i=o['input'];name=o['name']
    if name in ['support-query','query-coordinate']:return query(i['a'],i['y'],i)
    if name=='query-prepared-x':return query(value>>8,i['y'],i)
    if name=='query-page-toggle':return query(i['a'],i['y'],dict(i,toggle=i['pageToggle']))
    if name in ['support-index','read-index']:return (value&255)|(i['y']<<8)
    if name=='stream-column':
        address=i['tileAddress'];high=address>>8;low=address&255
        if low&1:return(value&0xff00)|high
        index=((high&4)<<4)|(low>>3)
        return((index+56)&255)|(((low>>1)&3)<<8)
    if name=='stream-row':return((((i['axis']>>1)*4)+(4 if i['axis']&1==0 else 0))&255)
    if name=='graphic-location':
        sy=i['y']+i['scrollY'];row=sy&255
        if sy>=240:row=(row+16)&255
        sx=i['graphicX']+i['scrollX'];page=(i['control']&1)^(int(sx>=256))
        return 0x2000|(page<<10)|((row&0xf8)<<2)|((sx&255)>>3)
    if name in ['player-perspective','weapon-perspective']:
        x=i['playerX']if name=='player-perspective'else i['sourceX']
        return(value&0xff00)|((x-128)&255)
    if name=='entity-probe':
        x=(i['a']+i['entityX'])&255;y=i['y']+i['entityY']
        return(value&255)|(x<<8)if y>=256 else query(x,y,i)
    if name=='collision-clear':return(value&0xff00)|((value<<5)&255)
    if name=='guardian-row-shift':return(value-32)&65535
    if name=='guardian-pair-clear':
        value=(value+4)&65535
        return(value&0xff00)|((value<<5)&255)
    raise ValueError(name)

def selected(frame):
    all_ops={o['id']:o for o in frame['operations']};chosen=[]
    for o in all_ops.values():
        if o['name']not in KINDS:continue
        parent=o['parent'];nested=False
        while parent:
            p=all_ops[parent]
            if p['name']in KINDS:nested=True;break
            parent=p['parent']
        if not nested:chosen.append(o)
    return sorted(chosen,key=lambda o:o['first'])

def analyze(data):
    failures=[];counts={};checked=0;checkpoints=0;gaps=0;uncovered={}
    value=data['frames'][0]['before']['work']
    for f in data['frames']:
        if value!=f['before']['work']:failures.append({'frame':f['frame'],'boundary':'C001','got':value,'want':f['before']['work']})
        ops=selected(f);coverage=set()
        events=[(o['last'],0,o)for o in ops]+[(c['sequence'],1,c)for c in f['checkpoints']]
        for _,kind,o in sorted(events,key=lambda e:e[:2]):
            if kind:
                if value!=o['work']:failures.append({'frame':f['frame'],'checkpoint':o['name'],'got':value,'want':o['work']})
                checkpoints+=1;continue
            counts[o['name']]=counts.get(o['name'],0)+1
            if value!=o['input']['work']:failures.append({'frame':f['frame'],'operation':o['name'],'boundary':'before','got':value,'want':o['input']['work']})
            try:value=project(value,o)
            except KeyError as e:failures.append({'frame':f['frame'],'operation':o['name'],'missingInput':str(e)});break
            if value!=o['after']:failures.append({'frame':f['frame'],'operation':o['name'],'boundary':'after','got':value,'want':o['after']})
            for w in f['writes']:
                if o['first']<w['sequence']<o['last']:coverage.add(w['sequence'])
            checked+=1
        if value!=f['after']['work']:failures.append({'frame':f['frame'],'boundary':'F8DB','got':value,'want':f['after']['work']})
        for w in f['writes']:
            if w['sequence']not in coverage:
                k=f'{w["bank"]:02x}:{w["pc"]:04x}:{w["address"]:02x}';uncovered[k]=uncovered.get(k,0)+1
        gaps+=len(f['priorGap'])
    return {'passed':not failures and not uncovered and not gaps,'seededOnce':True,'frames':len(data['frames']),'projectedOperations':checked,'checkpoints':checkpoints,'counts':counts,'uncoveredWrites':uncovered,'outsideWrites':gaps,'failures':failures}

def main():
    p=argparse.ArgumentParser();p.add_argument('name');args=p.parse_args()
    path=ROOT/f'docs/research/nes-contra/evidence/terrain-work-{args.name}.json.gz';data=json.loads(gzip.decompress(path.read_bytes()));report=analyze(data)
    out=ROOT/'.local/contra-terrain-work/analysis';out.mkdir(parents=True,exist_ok=True);(out/(args.name+'.json')).write_text(json.dumps(report,indent=2)+'\n')
    print(json.dumps({k:(v[:10]if k=='failures'else v)for k,v in report.items()}))
    if not report['passed']:raise SystemExit(1)

if __name__=='__main__':main()
