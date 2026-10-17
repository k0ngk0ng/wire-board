"""Original A2E9 mode7 parameter0 against a real non-player target index7."""
import hashlib,itertools,json
from verify_prop_contact_original import ROOT,code,MPU
from verify_pass_floor_original import put

def main():
    rows=[]
    for actor,x,target_x in itertools.product(range(4),[100*256+255,200*256,200*256+255,65535*256+2],[100*256,200*256+127,65534*256]):
        r=bytearray(2048);r[0x53f]=6;r[0x672+actor]=7;r[0x67a+actor]=7;r[0x682+actor]=0;r[0x512+actor]=130;r[0x66e+actor]=9;r[0x576+actor]=11
        put(r,x,0x68+actor,0x75+actor,0x82+actor);put(r,target_x,0x6f,0x7c,0x89)
        put(r,123,0x478+actor,0x485+actor);put(r,-99,0x492+actor,0x49f+actor)
        m=MPU();m.memory[0x8000:]=code;m.memory[:2048]=r;m.x=actor;m.sp=255;m.stPushWord(0x6ffe);m.pc=0xa2e9
        for n in range(2000):
            if m.pc==0x6fff:break
            m.step()
        assert m.pc==0x6fff
        rows.append(dict(actor=actor,x=x,targetX=target_x,action=m.memory[0x512+actor],mode=m.memory[0x672+actor],counter=m.memory[0x66e+actor],jumpPhase=m.memory[0x576+actor],target=m.memory[0x67a+actor],heading=m.memory[0x4ec+actor],facing=m.memory[0x46b+actor],velocity=[m.memory[a+actor] for a in [0x478,0x485,0x492,0x49f]]))
    p=ROOT/'games/basketball/engine/testdata/engagement-hoop-isolated.json';p.write_text(json.dumps(rows,separators=(',',':'))+'\n');p.with_name('engagement-hoop-isolated-source.json').write_text(json.dumps(dict(records=len(rows),natural_gameplay=False,entry='bank5:A2E9 -> AEB6 -> B1E9',original_instructions_unchanged=True,bank5_sha256=hashlib.sha256(code).hexdigest(),sha256=hashlib.sha256(p.read_bytes()).hexdigest()),indent=2)+'\n');print('hoop engagement cases',len(rows))

if __name__=='__main__':main()
