"""Original 876F reverse prop contact oracle; explicitly synthetic function inputs."""
import gzip, hashlib, itertools, json
from verify_prop_contact_original import ROOT, code, build, state, MPU, word

def run(r, source):
    m=MPU();m.memory[0x8000:]=code;m.memory[:2048]=r;m.x=source;m.sp=255;m.stPushWord(0x6ffe);m.pc=0x8653;sounds=[]
    for n in range(2000):
        if m.pc==0x6fff:return bytes(m.memory[:2048]),sounds
        if m.pc==0xc000:sounds.append(m.a);m.pc=(m.stPopWord()+1)&65535
        else:m.step()
    raise RuntimeError('prop response did not return')

def main():
    records=[]
    cases=itertools.product([4,5],range(4),[11,12,13,14],[False,True],[0,6,10,13,16,18],[1,2],[71,72])
    for index,(source,target,kind,facing,action,court,depth) in enumerate(cases):
        r=build(index,0,source,1,0,False,0)
        r[0x3c:0x42]=bytes([128]*6);r[0x3c+source]=target;r[0x3d6+source]=kind
        r[0x512+source]=128|action;r[0x46b+source]=128 if facing else 0
        r[0x56b]=3;r[0x58b]=court
        word(r,0x8f+target,0x9c+target,depth*256+127)
        a,sounds=run(r,source)
        kinds=[12,11];kinds[source-4]=kind
        records.append(dict(name=f'player-{index}',source=source,target=target,auxiliaryKinds=kinds,court=court,before=state(r),after=state(a),sounds=sounds))
    for index,(source,action,target_action,side) in enumerate(itertools.product([4,5],[0,6,10,13,16],[0,6],[-1,0,1])):
        target=source^1;r=build(index,0,target,1,0,False,target_action)
        r[0x3c:0x42]=bytes([128]*6);r[0x3c+source]=target;r[0x512+source]=128|action;r[0x56b]=3
        word(r,0x68+source,0x75+source,200*256+123);word(r,0x68+target,0x75+target,(200+side)*256+39)
        a,sounds=run(r,source)
        records.append(dict(name=f'prop-{index}',source=source,target=target,auxiliaryKinds=[12,11],court=1,before=state(r),after=state(a),sounds=sounds,unsupportedDoubleRelease=action==6 and target_action==6))
    p=ROOT/'games/basketball/engine/testdata/prop-attack-isolated.json.gz'
    packed=gzip.compress((json.dumps(records,separators=(',',':'))+'\n').encode(),mtime=0);p.write_bytes(packed)
    p.with_name('prop-attack-isolated-source.json').write_text(json.dumps(dict(records=len(records),source_is_original_execution=True,natural_gameplay=False,entry='bank5:8653 -> 876F',original_instructions_unchanged=True,intercept='C000 sound sink logs A and returns; no gameplay instructions replaced',bank5_sha256=hashlib.sha256(code).hexdigest(),sha256=hashlib.sha256(packed).hexdigest()),indent=2)+'\n')
    print('original reverse prop contacts',len(records))

if __name__=='__main__':main()
