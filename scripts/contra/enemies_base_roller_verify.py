"""Verify the narrowly scoped BASE-ROLLER-STALE-ATTRIBUTE exception.

This compares original executions only; it neither computes enemy results nor
modifies production rules. The portable bundle retains every compared byte.
"""
from pathlib import Path
import argparse,base64,datetime,gzip,hashlib,json

ROOT=Path(__file__).resolve().parents[2]
EVIDENCE=ROOT/'docs/research/nes-contra/evidence'
BASE=ROOT/'.local/contra-enemies-base/roller-oracle'
ATTRS=[0,12,19,255]
SCENARIOS=[(i,'natural')for i in[140,8586,8715,9174]]+[(140,m)for m in['contact','shot','falcon','reuse']]
CAPTURES=[1,40,100,200,400]
FILES=['initial-ram.bin','initial-prg.bin','ram.bin','dma.bin','hardware.jsonl','calls.jsonl','audio.jsonl','inputs.csv','phases.jsonl','attribute-reads.jsonl','interventions.jsonl','frames.csv','source.json','scenario.json']+[f'frame-{n:05}.{ext}'for n in CAPTURES for ext in['png']]+[f'frame-{n:05}-{part}.bin'for n in CAPTURES for part in['ppu','prg']]
PACKAGE=EVIDENCE/'enemies-base-roller-paired.json.gz'

def digest(b):return hashlib.sha256(b).hexdigest()
def raw(a):return base64.b64decode(a['data'])if a['encoding']=='base64'else a['data'].encode()
def encode(path):
    b=path.read_bytes();binary=path.suffix in['.bin','.png']
    return dict(sha256=digest(b),encoding='base64'if binary else'utf8',data=base64.b64encode(b).decode()if binary else b.decode())
def rows(run,name):return[json.loads(x)for x in raw(run['artifacts'][name]).splitlines()]

def pack():
    runs=[]
    for record,mode in SCENARIOS:
        for attr in ATTRS:
            path=BASE/'runs'/f'roller-{record}-{mode}-{attr}-v3'
            runs.append(dict(record=record,mode=mode,attribute=attr,path=str(path.relative_to(ROOT)),provenance=json.loads((path/'provenance.json').read_text()),artifacts={n:encode(path/n)for n in FILES}))
    bundle=dict(source_is_original_execution=True,exception='BASE-ROLLER-STALE-ATTRIBUTE',format='complete-compared-artifacts-v1',runs=runs)
    with PACKAGE.open('wb')as f:
        with gzip.GzipFile(fileobj=f,mode='wb',mtime=0)as z:z.write(json.dumps(bundle,separators=(',',':')).encode())

def verify():
    bundle=json.loads(gzip.decompress(PACKAGE.read_bytes()));assert bundle['source_is_original_execution']and len(bundle['runs'])==32
    indexed={(r['record'],r['mode'],r['attribute']):r for r in bundle['runs']}
    results=[]
    for record,mode in SCENARIOS:
        baseline=indexed[record,mode,0];base=raw(baseline['artifacts']['ram.bin']);assert len(base)==400*2048
        base_phases=rows(baseline,'phases.jsonl');base_calls=rows(baseline,'calls.jsonl')
        assert len(base_phases)==400 and all(r['dmaFrame']==r['frame']for r in base_phases)
        assert len(raw(baseline['artifacts']['dma.bin']))==400*256 and len(set(raw(baseline['artifacts']['dma.bin'])))>2
        phases=sorted({r['routine']for r in base_calls if r['pc']==0xe58a and r['kind']==17})
        collisions=sorted({r['collision']&15 for r in base_calls if r['pc']==0xe342 and r['kind']==17})
        assert set(collisions)<={2,14}and collisions
        assert 1 in phases and 2 in phases
        if mode in['shot','falcon']:assert phases==[1,2,3,4,5]
        if mode=='shot':assert sorted({r['enemy'][16]for r in base_phases})==[0,1,2,3,4,5]and base_phases[-1]['score'][2]>base_phases[0]['score'][2]
        if mode=='contact':assert 2 in{r['players'][0]for r in base_phases}
        if mode=='falcon':assert any(r['pc']==0xe949 for r in base_calls)and any(r['enemy'][19]==128 for r in base_phases)
        if mode=='reuse':assert any(r['pc']==0xedad and r['kind']==27 for r in base_calls)
        reads=rows(baseline,'attribute-reads.jsonl')
        # FCEUX reports the PC after the 3-byte instruction. The STA hooks
        # include the original indexed-store bus read, not a semantic load.
        assert all(r['pc']in[0xe971,0xe976,0xed73]for r in reads),reads
        variants=[]
        for attr in ATTRS:
            run=indexed[record,mode,attr]
            for a in run['artifacts'].values():assert digest(raw(a))==a['sha256']
            provenance=run['provenance'];assert provenance['attribute']==attr and provenance['mode']==mode and provenance['original_record_id']==record
            candidate=raw(run['artifacts']['ram.bin']);assert len(candidate)==len(base)
            differences={i%2048 for i,(a,b)in enumerate(zip(base,candidate))if a!=b}
            assert differences==({0x5b7}if attr else set()),(record,mode,attr,differences)
            # The exemption stops at actual slot reuse; the newly initialized
            # projectile must have zero Attributes and complete RAM equality.
            reused=[]
            for n in range(400):
                a=base[n*2048:(n+1)*2048];b=candidate[n*2048:(n+1)*2048]
                if a[0x4b8+15]and a[0x528+15]!=17:
                    assert a[0x5b7]==b[0x5b7]==0 and a==b
                    reused.append(n+1)
            same=['initial-ram.bin','initial-prg.bin','dma.bin','hardware.jsonl','calls.jsonl','audio.jsonl','inputs.csv','frames.csv']+[f'frame-{n:05}.png'for n in CAPTURES]+[f'frame-{n:05}-{part}.bin'for n in CAPTURES for part in['ppu','prg']]
            for name in same:assert raw(run['artifacts'][name])==raw(baseline['artifacts'][name]),(record,mode,attr,name)
            states=rows(run,'phases.jsonl');assert len(states)==400
            for a,b in zip(base_phases,states):
                b['enemy'][19]=a['enemy'][19];assert a==b
            other_reads=rows(run,'attribute-reads.jsonl')
            for r in other_reads:r.pop('value')
            expected_reads=[{k:v for k,v in r.items()if k!='value'}for r in reads]
            assert other_reads==expected_reads
            variants.append(dict(attribute=attr,ram_sha256=digest(candidate),differing_addresses=[hex(x)for x in sorted(differences)],complete_ram_equal_after_reuse_frames=len(reused)))
        results.append(dict(original_record_id=record,mode=mode,frames=400,phases=phases,collision_codes=collisions,attribute_accesses=reads,hardware_writes=len(rows(baseline,'hardware.jsonl')),variants=variants,matched_artifacts=same))
    result=dict(exception='BASE-ROLLER-STALE-ATTRIBUTE',verified_at=datetime.datetime.now(datetime.timezone.utc).isoformat(),rom_sha256='6ba53139fa88b8de1ae527c438bda6f1541d1ee7df26d63dec5164a32d166bfe',source_is_original_execution=True,run_count=32,total_original_frames=12800,paired_comparisons=24,sole_permitted_difference='roller Attributes at $05A8 + current roller slot; forbidden after active slot changes kind',bundle=dict(path=str(PACKAGE.relative_to(ROOT)),sha256=digest(PACKAGE.read_bytes())),script_sha256=digest(Path(__file__).read_bytes()),scenarios=results)
    (EVIDENCE/'enemies-base-roller-verification.json').write_text(json.dumps(result,indent=2)+'\n')
    print(json.dumps(dict(runs=32,frames=12800,paired_comparisons=24,all_compared_observations_equal=True)))

if __name__=='__main__':
    p=argparse.ArgumentParser();p.add_argument('--pack',action='store_true');args=p.parse_args()
    if args.pack:pack()
    verify()
