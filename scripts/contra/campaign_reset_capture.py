"""Controlled original transition/menu boundaries; never inject return state.

Failures considered before independent reset code: retaining any transient
player/enemy/projectile/map fields; erasing persistent snow/OAM/score/lives;
losing 0195/0196 generator state; clearing deferred palette request with its
timers; duplicate level/continue/audio decisions; Start+Select precedence;
inactive P2 lives changed by continue; partial mutation on invalid reset input.
"""
import os
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2]
os.environ.setdefault('CONTRA_REFERENCE_ROOT',str(ROOT/'.local/contra-campaign-reset/oracle'))
from capture_reference import BASE,run
import argparse,gzip,hashlib,json,shutil
HOOK=r'''
local f=assert(io.open(out..'/reset.jsonl','w'));local pending={};local hooks={};local exits={};local active=0;local restoring=nil;local recording=false
local function on(pc,callback)
 if not hooks[pc]then hooks[pc]={};memory.registerexec(pc,1,function()for _,fn in ipairs(hooks[pc])do fn()end end)end
 hooks[pc][#hooks[pc]+1]=callback
end
local function hex(first,n,reader)local v={};for a=first,first+n-1 do v[#v+1]=string.format('%02x',reader(a))end;return table.concat(v)end
local function ram()return hex(0,2048,memory.readbyte)end
local function ppuhex()return hex(0x2000,4096,ppu.readbyte)end
local function retpc()local sp=memory.getregister('s');return memory.readbyte(0x100+AND(sp+1,255))+256*memory.readbyte(0x100+AND(sp+2,255))+1 end
local function finish(pc)
 local names={};for name,p in pairs(pending)do if p.pc==pc and memory.getregister('s')==AND(p.sp+2,255)then names[#names+1]=name end end;table.sort(names)
 for _,name in ipairs(names)do local p=pending[name];pending[name]=nil;f:write(string.format('{"kind":"%s","case":%d,"frame":%d,"before":"%s","after":"%s","ppuBefore":"%s","ppuAfter":"%s","sounds":[%s]}\n',name,p.case,p.frame,p.before,ram(),p.ppu,ppuhex(),table.concat(p.sounds,',')))end
 if restoring and restoring.pc==pc and memory.getregister('s')==AND(restoring.sp+2,255)and next(pending)==nil then
  for a,v in pairs(restoring.bytes)do memory.writebyte(a,v)end;restoring=nil;recording=false
 end
end
local function arm(pc)if not exits[pc]then exits[pc]=true;on(pc,function()finish(pc)end)end end
local function enter(name)
 if not recording then return end
 assert(not pending[name],name..' nested');local pc=retpc();arm(pc)
 pending[name]={pc=pc,sp=memory.getregister('s'),case=active,frame=emu.framecount(),before=ram(),ppu=ppuhex(),sounds={}}
end
on(0xf9bc,function()for _,p in pairs(pending)do p.sounds[#p.sounds+1]=tostring(memory.getregister('a'))end end)
on(0xf9d4,function()for _,p in pairs(pending)do p.sounds[#p.sounds+1]='-1'end end)
local cases=CASES
on(0xcc42,function()
 assert(not restoring and next(pending)==nil,'unfinished reset case')
 if active>=#cases then recording=false;return end
 active=active+1;recording=true;restoring={pc=retpc(),sp=memory.getregister('s'),bytes={}};arm(restoring.pc)
 for a=0x18,0xff do restoring.bytes[a]=memory.readbyte(a)end
 for a=0x180,0x1df do restoring.bytes[a]=memory.readbyte(a)end
 for a=0x300,0x7ff do restoring.bytes[a]=memory.readbyte(a)end
 for a=0x40,0xef do memory.writebyte(a,AND(a*37+17,255))end
 for a=0x300,0x6ff do memory.writebyte(a,AND(a*23+13,255))end
 for a=0x190,0x196 do memory.writebyte(a,AND(a*7+3,255))end
 for a,v in pairs(cases[active])do memory.writebyte(a,v)end
end)
on(0xcd1f,function()enter('transition')end)
on(0xc309,function()if pending.transition then enter('runtimeClear')end end)
on(0xcd71,function()if pending.transition then enter('gameoverDisplay')end end)
on(0xcdaf,function()enter('continueMenu')end)
on(0xc2cf,function()if pending.continueMenu then enter('continueCounters')end end)
on(0xcdbf,function()if pending.continueMenu then enter('continueDisplay')end end)
on(0xcdf1,function()enter('exhaustedMenu')end)
function finish_probe()assert(next(pending)==nil and not restoring,'unfinished reset probe');assert(active==#cases,'incomplete cases');f:close()end
'''
def cases():
 base={0x18:5,0x1c:0,0x1d:7,0x20:0,0x21:0,0x22:1,0x23:0,0x24:0,0x25:0,0x2c:5,0x2d:3,0x30:0,0x31:0,0x32:5,0x33:4,0x36:0,0x37:1,0x38:0,0x39:0,0x3a:3,0x3b:1,0x3c:144,0x3d:1,0x3e:188,0x3f:2,0x6b:0,0x90:1,0x91:1,0xaa:19,0xab:2,0xf5:0,0xf6:0,0xfb:6,0xfc:200,0xfd:123,0xfe:30,0xff:177,0x700:0,0x7e0:44,0x7e1:1,0x7e2:123,0x7e3:0,0x7e4:231,0x7e5:3}
 out=[]
 def add(fields):v=dict(base);v.update(fields);out.append(v)
 for level in range(8):
  for mode in (0,1):
   for dead in (0,1):add({0x30:level,0x22:mode,0x38:dead,0x39:1-mode,0x32:0 if dead else 5})
 for mode in (0,1):
  for continues in range(4):add({0x22:mode,0x3a:continues,0x3b:0,0x38:1,0x39:1,0x32:0,0x33:0})
 for mode in (0,1):
  for choice in (0,1):
   for pressed in (0,16,32,48):
    for cheat in (0,1):add({0x22:mode,0x2c:6,0x3a:2,0x3b:0,0x6b:choice,0xf5:pressed,0x24:cheat,0x38:1,0x39:1,0x32:0,0x33:7})
 for pressed in (0,16):add({0x2c:7,0x3a:255,0x3b:0,0xf5:pressed})
 return out
def main():
 p=argparse.ArgumentParser();p.add_argument('--name',required=True);p.add_argument('--verify-against');a=p.parse_args()
 (BASE/'states').mkdir(parents=True,exist_ok=True)
 src=ROOT/'.local/contra/states/coop-play.fcs';dst=BASE/'states/coop-play.fcs'
 if not dst.exists():shutil.copyfile(src,dst)
 assert dst.read_bytes()==src.read_bytes()
 rows=cases();lua='{'+','.join('{'+','.join(f'[{a}]={v}'for a,v in row.items())+'}'for row in rows)+'}'
 spec={'name':a.name,'load_state':'coop-play','frames':len(rows)+64,'segments':[],'captures':[],'timeout_seconds':120,'hooks_lua':HOOK.replace('CASES',lua)}
 path=BASE/(a.name+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');out=run(path)
 records=[json.loads(l)for l in(out/'reset.jsonl').read_text().splitlines()]
 assert len({r['case']for r in records})==len(rows)
 payload={'scope':'Declared transition/menu entry inputs, restored only after full original return; no return state injection and no natural victory claim.','source':json.loads((out/'source.json').read_text()),'cases':[{hex(a):v for a,v in row.items()}for row in rows],'scenario':spec,'records':records}
 if a.verify_against:
  assert Path(a.verify_against).name==a.verify_against
  prior=json.loads(gzip.decompress((ROOT/'docs/research/nes-contra/evidence'/a.verify_against).read_bytes()))
  assert prior['records']==records,'independent original replay differs'
  report={'passed':True,'cases':len(rows),'records':len(records),'records_sha256':hashlib.sha256(json.dumps(records,sort_keys=True,separators=(',',':')).encode()).hexdigest()}
  (out/'repeat-verification.json').write_text(json.dumps(report,indent=2)+'\n');print(json.dumps(report));return
 data=gzip.compress(json.dumps(payload,separators=(',',':')).encode(),mtime=0)
 (ROOT/'docs/research/nes-contra/evidence/campaign-reset-original.json.gz').write_bytes(data)
 print(json.dumps({'cases':len(rows),'records':len(records),'sha256':hashlib.sha256(data).hexdigest()}))
if __name__=='__main__':main()
