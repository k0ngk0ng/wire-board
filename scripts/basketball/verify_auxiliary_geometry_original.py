"""Execute original 8318 over restored auxiliary action domains 0..15."""
import hashlib,json
from verify_prop_contact_original import ROOT,code,MPU

def main():
    read=lambda a:code[a-0x8000]
    word=lambda a:read(a)+256*read(a+1)
    pointers=[word(0x8fa5+2*i) for i in range(30)]
    boundaries=sorted(set(pointers+[0x912f]))
    rows=[];dynamic=[]
    for kind in [10,11,12,13,14]:
        action_map=word(0x8cf2+2*kind);flags_map=word(0x9301+2*kind)
        for action in range(16):
            group=read(action_map+action)
            if group>=len(pointers):
                assert (kind,action,group)==(14,3,30)
                for seed in (0,17):
                    for frame in range(4):
                        m=MPU();m.memory[0x8000:]=code;m.memory[:2048]=[(i*13+seed)&255 for i in range(2048)];m.x=5;m.memory[0x3db]=kind;m.memory[0x517]=action|128;m.memory[0x449]=frame;m.sp=255;m.stPushWord(0x6ffe);m.pc=0x8318
                        for step in range(1000):
                            if m.pc==0x6fff:break
                            m.step()
                        assert m.pc==0x6fff
                        shape=m.memory[0x2c]+256*m.memory[0x2d]
                        assert shape==[0x80,0,0x80,0][frame]
                        dynamic.append(dict(kind=kind,action=action,frame=frame,syntheticRAMSeed=seed,shapeAddress=shape,shape=list(m.memory[shape:shape+8]),group=group,unsupportedDynamicRAM=True))
                continue
            start=pointers[group];end=next(v for v in boundaries if v>start)
            for frame in range((end-start)//2):
                m=MPU();m.memory[0x8000:]=code;m.x=4;m.memory[0x3da]=kind;m.memory[0x516]=action|128;m.memory[0x448]=frame;m.sp=255;m.stPushWord(0x6ffe);m.pc=0x8318
                for step in range(1000):
                    if m.pc==0x6fff:break
                    m.step()
                assert m.pc==0x6fff
                shape=m.memory[0x2c]+256*m.memory[0x2d];assert shape==word(start+2*frame)
                rows.append(dict(kind=kind,action=action,frame=frame,shape=list(m.memory[shape:shape+8]),flags=read(flags_map+action),shapeAddress=shape,group=group))
    p=ROOT/'games/basketball/engine/testdata/auxiliary-geometry-isolated.json';p.write_text(json.dumps(rows,separators=(',',':'))+'\n')
    p.with_name('auxiliary-geometry-isolated-source.json').write_text(json.dumps(dict(records=len(rows),kindActionMaps=['8F51','8F61','8F71','8F85','8F95'],sharedFlagMap='93A3',entry='bank5:8318',original_instructions_unchanged=True,method='Shape pointer from original machine-code function; flags are original ROM source data',synthetic_inputs=True,natural_gameplay=False,dynamic_records='auxiliary-geometry-dynamic-isolated.json',bank5_sha256=hashlib.sha256(code).hexdigest(),sha256=hashlib.sha256(p.read_bytes()).hexdigest()),indent=2)+'\n')
    q=p.with_name('auxiliary-geometry-dynamic-isolated.json');q.write_text(json.dumps(dynamic,separators=(',',':'))+'\n')
    q.with_name('auxiliary-geometry-dynamic-isolated-source.json').write_text(json.dumps(dict(records=len(dynamic),entry='bank5:8318',bank5_sha256=hashlib.sha256(code).hexdigest(),sha256=hashlib.sha256(q.read_bytes()).hexdigest(),natural_gameplay=False,method='Original instructions; explicit synthetic RAM patterns i*13+seed; original RAM-dependent pointers retained',static_profile_supported=False,restored_context_cursors=[0,1,2,3]),indent=2)+'\n')
    print('auxiliary original geometry records',len(rows))

if __name__=='__main__':main()
