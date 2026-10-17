"""Parse original sound data into bounded sequence/instrument records.

The product receives custom music commands and named parameters, not a ROM
image, CPU instructions, captured RAM, or prerecorded per-frame work values.
"""
import collections
import hashlib
import json
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2]


def main():
    rom=(ROOT/'.local/nes-basket/basketball.nes').read_bytes()
    assert hashlib.sha256(rom).hexdigest()=='0fa28f60abb8d2ccbbeee90e79e3fa160a5cf32feb90f47ebf1a89168cff569b'
    bank=rom[16:16+16384]
    def byte(address):
        assert 0x8000<=address<0xc000,hex(address)
        return bank[address-0x8000]
    def word(address):return byte(address)+256*byte(address+1)
    data={'schema':1,'rom_sha256':hashlib.sha256(rom).hexdigest(),'sounds':{},'headers':{},'instructions':{},'instruments':{},'envelopes':{},'percussion':{},
          'pitch_low':[byte(0x897b+i) for i in range(65)],'pitch_high':[byte(0x89bc+i) for i in range(64)],
          'handler_high':[byte(0x838e+i) for i in range(32)]}
    pending=collections.deque();visited=set()
    def header(address):
        key=str(address)
        if key in data['headers']:return
        value={'priority':byte(address),'repeat':None,'channels':[]}
        cursor=address+1
        if byte(cursor)&128:value['repeat']=byte(cursor+1);cursor+=2
        while not byte(cursor)&128:
            ch=byte(cursor);stream=word(cursor+1);assert 0<=ch<=8
            value['channels'].append({'channel':ch,'stream':stream});pending.append((stream,ch,0,0));cursor+=3
        value['source_address']=address
        data['headers'][key]=value
    for ident in range(1,77):
        address=word(0x8be0+2*(ident-1));data['sounds'][str(ident)]=address;header(address)
    def envelope(address):
        if str(address) in data['envelopes']:return
        value={'initial':byte(address),'gate':byte(address+1),'release':byte(address+2),'steps':[]}
        cursor=address+3
        for _ in range(128):
            delta,wait=byte(cursor),byte(cursor+1);value['steps'].append({'delta':delta,'wait':wait});cursor+=2
            if wait==0:break
        else:raise AssertionError(('unterminated envelope',hex(address)))
        data['envelopes'][str(address)]=value
    def percussion(table,note):
        assert table,('percussion without table',note)
        address=word(table+2*(note&63));data['percussion'][str(table+2*(note&63))]=address;header(address)
    operands=[1,1,1,1,1,1,1,1,1,0,0,0,1,2,3,3,2,2,0,0,1,0,0,0,1,1,1,3,0,0,3,None]
    while pending:
        cursor,ch,table,ret=pending.popleft()
        context=(cursor,ch,table,ret)
        if context in visited:continue
        visited.add(context);assert len(visited)<200000
        op=byte(cursor);args=[]
        if op<0x40:size=1
        elif op<0xc0:
            size=2 if ch<3 and op&128 else 1
            args=[byte(cursor+1)] if size==2 else []
            if ch==3:percussion(table,op)
        else:
            assert op<0xe0,('unknown music opcode',hex(cursor),hex(op),ch)
            n=operands[op-0xc0]
            if op==0xdf:
                end=cursor+1
                for _ in range(2):
                    note=byte(end);args.append(note);end+=1
                    if ch<3 and note&128:args.append(byte(end));end+=1
                    if ch==3:percussion(table,note)
                size=end-cursor
            else:
                args=[byte(cursor+1+i) for i in range(n)];size=1+n
            if op==0xcd:
                ptr=args[0]+256*args[1]
                if ch==3:table=ptr
                elif ch&3!=2:
                    inst={'envelope':word(ptr),'control':byte(ptr+2),'sweep':byte(ptr+3)}
                    data['instruments'][str(ptr)]=inst
                    if ch<4:envelope(inst['envelope'])
            elif op==0xd9 and 0<args[0]<=76:
                header(data['sounds'][str(args[0])])
        value={'opcode':op,'args':args,'size':size,'source_address':cursor}
        key=str((ch<<16)|cursor)
        if key in data['instructions']:assert data['instructions'][key]==value
        data['instructions'][key]=value
        if op==0xd3:continue
        if op in (0xce,0xcf):pending.append((args[1]+256*args[2],ch,table,ret))
        if op==0xd0:pending.append((args[0]+256*args[1],ch,table,ret));continue
        if op==0xd1:pending.append((args[0]+256*args[1],ch,table,cursor+size));continue
        if op==0xd2:
            assert ret,('return without caller',hex(cursor),ch)
            pending.append((ret,ch,table,ret));continue
        pending.append((cursor+size,ch,table,ret))
    out=ROOT/'games/basketball/audiostate/source-data.json'
    out.write_text(json.dumps(data,separators=(',',':'))+'\n')
    summary={key:len(data[key]) for key in ['sounds','headers','instructions','instruments','envelopes','percussion']}
    summary.update(sha256=hashlib.sha256(out.read_bytes()).hexdigest(),rom_sha256=data['rom_sha256'],generator_sha256=hashlib.sha256(Path(__file__).read_bytes()).hexdigest(),scope='parsed music command graph and instrument parameters; no runtime ROM or instruction emulator')
    (ROOT/'.local/basketball/audio/state/data-export.json').write_text(json.dumps(summary,indent=2)+'\n')
    (out.parent/'source-data-source.json').write_text(json.dumps(summary,indent=2)+'\n')
    print(json.dumps(summary))


if __name__=='__main__':main()
