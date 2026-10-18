"""Complete original reward calls, including actors/motion/projection.

Failure inventory: omit confirmation actor tail; stop exit before its three
actors; wrong sound order; lose post-animation RNG; redraw without draw-list
reset; award on resume; clone/JSON continuation differs; outer RNG doubled.
Controlled CPU chains with explicit render-boundary resets; not video timing.
"""
import argparse,base64,gzip,hashlib,json
from verify_campaign_settlement import OUT,ROM,PRG,RecordedMemory,MPU,progress

OMIT={0xc000:'audio driver',0xc021:'fade/NMI wait',0xc024:'background upload',0xc036:'message upload'}
def execute(m,entry,bank):
    m[0x8000:0xc000]=PRG[bank*16384:(bank+1)*16384]
    cpu=MPU(memory=m,pc=entry); cpu.sp=255;cpu.p=0x20;cpu.stPushWord(0x6fff)
    sounds=[];actors=[];rng=0;events=[]
    for n in range(30000):
        if cpu.pc==0x7000: break
        if cpu.pc==0xc033: actors.append(cpu.x)
        if cpu.pc==0xf9c3: rng+=1
        if cpu.pc in OMIT:
            events.append(dict(pc=f'{cpu.pc:04X}',a=cpu.a))
            if cpu.pc==0xc000: sounds.append(cpu.a)
            cpu.pc=(cpu.stPopWord()+1)&65535
        else: cpu.step()
    else: raise RuntimeError(hex(cpu.pc))
    return dict(sounds=sounds,actors=actors,rngCalls=rng,events=events)

def main():
    p=argparse.ArgumentParser();p.add_argument('--check',action='store_true');args=p.parse_args()
    source=OUT/'campaign-settlement-cases.json.gz'
    cases=json.loads(gzip.decompress(source.read_bytes()))['cases']; chains=[]
    for shape in ('empty','equipped','full'):
        row=next(r for r in cases if r['label']==f'0-0-1-0-{shape}-True')
        for inputAt in (None,0,1,40,112):
            raw=base64.b64decode(row['phases'][1]['rawAfter']); m=RecordedMemory(raw)
            tail=execute(m,0xbc96,4)
            assert tail['actors']==[0,1,2,3]
            calls=[]
            for tick in range(113):
                # Explicit caller work: render-list reset, one Scene1 RNG draw.
                m[0x41c]=0;m[0x41d:0x429]=[255]*12
                prefix=execute(m,0xf9c3,4);assert prefix['rngCalls']==1
                m[4]=0x80 if tick==inputAt else 0
                before=bytes(m[:2048]);counts=execute(m,0xbd8e,4);after=bytes(m[:2048])
                assert counts['actors']==[0,1,2]
                calls.append(dict(rawBefore=base64.b64encode(before).decode(),rawAfter=base64.b64encode(after).decode(),beforeProgress=progress(before),afterProgress=progress(after),**counts))
                if m[0x588]==2:break
            else: raise RuntimeError('reward did not exit')
            chains.append(dict(shape=shape,inputAt=inputAt,confirmationTail=tail,calls=calls))
    payload=gzip.compress(json.dumps(chains,separators=(',',':')).encode(),mtime=0)
    path=OUT/'reward-presentation.json.gz'
    meta=dict(chains=len(chains),calls=sum(len(c['calls']) for c in chains),romSHA256=hashlib.sha256(ROM).hexdigest(),inputSHA256=hashlib.sha256(source.read_bytes()).hexdigest(),fixtureSHA256=hashlib.sha256(payload).hexdigest(),omittedCalls={f'{k:04X}':v for k,v in OMIT.items()},scope=__doc__)
    if args.check:assert path.read_bytes()==payload
    else:
        path.write_bytes(payload)
        (OUT/'reward-presentation-source.json').write_text(json.dumps(meta,indent=2)+'\n')
    print(json.dumps(meta))

if __name__=='__main__':main()
