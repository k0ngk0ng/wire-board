"""Capture original sound-controller ticks for the independent Go controller.

Failure inventory before implementation: per-slot vs global priority, same-tick
collapse vs later retrigger, finite slot lifetime vs WAV length, only pause cue
advancing while paused, clearing/replacing pending requests on reset, cue62/77
special guards, music percussion running through the same noise slot, and DPCM
bypassing that priority. Inputs are declared API requests/pause values; original
slot/driver/APU results are only observed, never written by this probe.
"""
import argparse
import gzip
import hashlib
import json
from pathlib import Path
import struct
import zipfile

from audio_export import ROOT,BASE,OUT,SHA,LUA,record

WARM=120
CASES=[
 ('priority',180,[(0,'sound',26),(0,'sound',26),(0,'sound',47),(10,'sound',26),(20,'sound',28),(24,'sound',11),(40,'sound',56),(50,'sound',26),(70,'sound',114),(71,'sound',116),(120,'sound',26)]),
 ('spread-reset',140,[(0,'sound',40)]*5+[(1,'sound',26),(50,'reset',0),(50,'sound',26),(60,'sound',51),(61,'sound',28)]),
 ('long-pause-shot',220,[(0,'sound',70),(0,'sound',26),(5,'pause',1),(5,'sound',116),(120,'pause',0),(120,'sound',26)]),
 ('long-pause-high-noise',220,[(0,'sound',70),(0,'sound',114),(5,'pause',1),(5,'sound',116),(120,'pause',0),(120,'sound',26),(150,'sound',26)]),
 ('clear-heart',420,[(0,'sound',70),(20,'sound',98),(30,'sound',26),(40,'sound',11),(50,'sound',119),(260,'sound',26),(280,'reset',0),(280,'sound',119),(290,'sound',98),(300,'sound',26)]),
 ('preview-veto',240,[(0,'sound',22),(1,'sound',24),(10,'sound',22),(11,'sound',24),(100,'sound',24),(120,'sound',26)]),
 ('dpcm-pause-reset',220,[(0,'sound',82),(0,'sound',114),(10,'sound',127),(11,'pause',1),(11,'sound',128),(50,'sound',129),(60,'reset',0),(65,'sound',127),(100,'pause',0),(110,'sound',70),(140,'sound',255)]),
 ('music-cycle',6500,[(0,'sound',70)]),
]

OBSERVER=r'''
local trace=assert(io.open(out..'/controller.jsonl','w'))
local active=nil;local commands={};local apuWrites={}
local function word(a)return memory.readbyte(a)+256*memory.readbyte(a+1)end
local function hex(a,n)local t={};for i=a,a+n-1 do t[#t+1]=string.format('%02x',memory.readbyte(i))end;return table.concat(t)end
local function tick()return word(0x5f0)-120 end
memory.registerexec(0x87a2,1,function()
 local id=memory.getregister('a')
 calls:write(string.format('%d,request,%d\n',emu.framecount(),id))
 commands[#commands+1]=string.format('{"id":%d,"internal":%s}',id,tostring(active~=nil))
end)
memory.registerexec(0x80d5,1,function()
 calls:write(string.format('%d,update,0\n',emu.framecount()))
 active={tick=tick(),before=hex(0x100,130),paused=memory.readbyte(0x25)}
end)
memory.registerwrite(0x4000,0x18,function(a,s,v)
 apu:write(string.format('%d,%d,%d\n',emu.framecount(),a,v))
 if a==0x4015 then apuWrites[#apuWrites+1]=v end
end)
memory.registerexec(AFTER,1,function()
 assert(active)
 if active.tick>=0 then
  local status={};for _,v in ipairs(apuWrites)do status[#status+1]=tostring(v)end
  trace:write(string.format('{"tick":%d,"paused":%d,"before":"%s","after":"%s","requests":[%s],"statusWrites":[%s]}\n',active.tick,active.paused,active.before,hex(0x100,130),table.concat(commands,','),table.concat(status,',')))
 end
 active=nil;commands={};apuWrites={}
end)
'''


def carrier(rom,events):
    prg=rom[16:131088];data=bytearray(prg[0x4000:0x8000]+prg[0x1c000:])
    init=bytes([0x20,0x0b,0xf9,0x20,0x19,0x87,0xa9,0,0x8d,0xf0,5,0x8d,0xf1,5,0x85,0x25,0x60])
    program=bytearray([0xee,0xf0,5,0xd0,3,0xee,0xf1,5])
    for tick,kind,value in events:
        at=WARM+tick
        action=bytes([0xa9,value,0x20,0xa2,0x87]) if kind=='sound' else bytes([0x20,0x19,0x87]) if kind=='reset' else bytes([0xa9,value,0x85,0x25])
        # Two comparisons skip only this requested input; no sound result writes.
        program.extend([0xad,0xf1,5,0xc9,at>>8,0xd0,7+len(action),0xad,0xf0,5,0xc9,at&255,0xd0,len(action)])
        program.extend(action)
    program.extend([0x20,0xd5,0x80,0xea,0x60])
    assert len(program)<0x6e0
    assert data[0x3900:0x3900+len(program)]==bytes([255])*len(program)
    assert data[0x3fe0:0x3fe0+len(init)]==bytes([255])*len(init)
    data[0x3900:0x3900+len(program)]=program;data[0x3fe0:0x3fe0+len(init)]=init
    header=bytearray(128);header[:8]=b'NESM\x1a\x01\x01\x01';struct.pack_into('<HHH',header,8,0x8000,0xbfe0,0xb900);struct.pack_into('<H',header,110,16639)
    return header+data,0xb900+len(program)-2


def main():
    parser=argparse.ArgumentParser();parser.add_argument('--label',default='v1');args=parser.parse_args()
    with zipfile.ZipFile('/Users/jason/Documents/nes/roms/03/魂斗罗日版(完全版).zip')as z:rom=z.read('Contra (J).nes')
    assert hashlib.sha256(rom).hexdigest()==SHA
    base=BASE/'control-oracle';base.mkdir(parents=True,exist_ok=True)
    for p in [BASE/'runtime',BASE/'tmp']:p.mkdir(parents=True,exist_ok=True)
    results=[]
    for name,ticks,events in CASES:
        nsf,after=carrier(rom,events);path=base/(name+'.nsf');path.write_bytes(nsf)
        hook=OBSERVER.replace('AFTER',hex(after))
        # Replace callbacks so request/APU rows are still logged exactly once.
        lua=LUA.replace("memory.registerexec(0x87a2,1,function()calls:write(string.format('%d,request,%d\\n',emu.framecount(),memory.getregister('a')))end)",'')
        lua=lua.replace("memory.registerexec(0x80d5,1,function()calls:write(string.format('%d,update,0\\n',emu.framecount()))end)",'')
        lua=lua.replace("memory.registerwrite(0x4000,0x18,function(a,s,v)apu:write(string.format('%d,%d,%d\\n',emu.framecount(),a,v))end)",'')
        lua=lua.replace("assert(package.loadlib(lib,'start'))()",hook+"\nassert(package.loadlib(lib,'start'))()")
        lua=lua.replace("assert(package.loadlib(lib,'stop'))();apu:close();calls:close();driver:close()","assert(package.loadlib(lib,'stop'))();assert(not active);trace:close();apu:close();calls:close();driver:close()")
        out=base/f'{name}-{args.label}';record(path,out,OUT/'wave-bridge.dylib',ticks+WARM+3,lua)
        rows=[json.loads(line)for line in (out/'controller.jsonl').read_text().splitlines()]
        rows=[r for r in rows if r['tick']<ticks]
        assert [r['tick']for r in rows]==list(range(ticks)),(name,len(rows))
        results.append({'name':name,'ticks':ticks,'inputs':[{'tick':t,'kind':k,'value':v}for t,k,v in events],'rows':rows,
                        'carrierSHA256':hashlib.sha256(nsf).hexdigest(),'traceSHA256':hashlib.sha256((out/'controller.jsonl').read_bytes()).hexdigest()})
    target=ROOT/'docs/research/nes-contra/evidence/audio-control-original.json.gz'
    fixture={'source_is_original_execution':True,'romSHA256':SHA,'scope':'Controlled original driver/API ticks, not gameplay/PCM reconstruction.',
             'interventions':'Tiny padding wrapper supplies declared sound/reset/pause input schedule; only its private counter and the game pause input byte are written.',
             'scenarios':results}
    if target.exists():
        old=json.loads(gzip.decompress(target.read_bytes()))
        for a,b in zip(old['scenarios'],results,strict=True):assert a['rows']==b['rows'] and a['traceSHA256']==b['traceSHA256'],b['name']
    else:target.write_bytes(gzip.compress(json.dumps(fixture,separators=(',',':')).encode(),mtime=0))
    print(json.dumps({'cases':len(results),'ticks':sum(s['ticks']for s in results),'sha256':hashlib.sha256(target.read_bytes()).hexdigest()}))


if __name__=='__main__':main()
