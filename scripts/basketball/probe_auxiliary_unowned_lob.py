"""Original BC74 column preparation followed by B07E with owner FF."""
import gzip,hashlib,itertools,json
from verify_pass_floor_original import ROOT,Memory,MPU,put
from extract_world_actions import decode_world_action_state


def execute(cpu,entry,obj):
    cpu.pc,cpu.x,cpu.p=entry,obj,0x20;cpu.sp=255;cpu.stPushWord(0x6fff)
    for n in range(30000):
        if cpu.pc==0x7000:return
        cpu.step()
    raise AssertionError(hex(cpu.pc))


def main():
    rom=(ROOT/'.local/nes-basket/basketball.nes').read_bytes();prg=rom[16:16+131072];base=(ROOT/'.local/nes-basket/runs/ball-pass/ram.bin').read_bytes()[:2048];rows=[]
    for obj,column,phase,facing,arc in itertools.product([4,5],range(64),[0,1],[0,128],[1,3]):
        r=bytearray(base)
        for address,value in {0x512+obj:2,0x51f+obj:128,0x52c+obj:255,0x437+obj:0,0x444+obj:0,0x451+obj:0,0x46b+obj:facing,0x5f7+obj:128,0x642+obj:0,0x539+obj:255,0x3d6+obj:12 if obj==4 else 11,0x55d:7,0x58d:128,0x589:phase,0x58b:5,0x3a1:column}.items():r[address]=value
        put(r,200*256+13,0x68+obj,0x75+obj,0x82+obj);put(r,40*256+17,0x8f+obj,0x9c+obj);put(r,32*256+11,0xa9+obj,0xb6+obj,0xc3+obj)
        r[0x69]=37;r[0x6f0]=arc
        memory=Memory(prg,r);memory[0x8000:0xc000]=prg[16384:32768];cpu=MPU(memory=memory);execute(cpu,0xbc74,0)
        height=(memory[0x135]|memory[0x142]<<8)*256
        memory[0x8000:0xc000]=prg[6*16384:7*16384];before=decode_world_action_state(memory);execute(cpu,0xb07e,obj)
        rows.append(dict(object=obj,column=column,phase=phase,arcFlags=arc,ownerHeight=height,before=before,after=decode_world_action_state(memory)))
    out=ROOT/'games/basketball/engine/testdata';packed=gzip.compress(json.dumps(rows,separators=(',',':')).encode(),mtime=0)
    (out/'auxiliary-unowned-lob.json.gz').write_bytes(packed)
    manifest=dict(records=len(rows),scope='configured original BC74 background-column generation followed by B07E owner-FF launch, no natural-game claim',romSHA256=hashlib.sha256(rom).hexdigest(),sha256=hashlib.sha256(packed).hexdigest(),unchanged_ROM_instructions=True,ownerHeightAddresses=['0135','0142'])
    (out/'auxiliary-unowned-lob-source.json').write_text(json.dumps(manifest,indent=2)+'\n');print(json.dumps(manifest))


if __name__=='__main__':main()
