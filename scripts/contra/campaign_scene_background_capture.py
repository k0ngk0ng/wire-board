"""Verify decoded scene background data against actual original C93A PPU writes."""
import os
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2]
os.environ.setdefault('CONTRA_REFERENCE_ROOT',str(ROOT/'.local/contra-campaign-scene/background-oracle'))
from capture_reference import BASE,run
import argparse,gzip,hashlib,json,shutil
HOOK=r'''
local f=assert(io.open(out..'/backgrounds.jsonl','w'));local pending=nil;local exits={}
local function ram(a)return memory.readbyte(a)end
local function ppuhex()local t={};for a=0x2000,0x27ff do t[#t+1]=string.format('%02x',ppu.readbyte(a))end;return table.concat(t)end
memory.registerexec(0xc93a,1,function()
 local index=memory.getregister('x');if ram(0x18)~=5 or (index~=0 and index~=14 and index~=16 and index~=18)then return end
 assert(not pending,'nested background function');local sp=memory.getregister('s');local ret=ram(0x100+AND(sp+1,255))+256*ram(0x100+AND(sp+2,255))+1
 pending={index=index,bank=ram(0x8000),frame=emu.framecount(),before=ppuhex(),ret=ret}
 if not exits[ret]then exits[ret]=true;memory.registerexec(ret,1,function()
  if not pending or pending.ret~=memory.getregister('pc')then return end
  local p=pending;pending=nil;f:write(string.format('{"index":%d,"bank":%d,"frame":%d,"before":"%s","after":"%s"}\n',p.index,p.bank,p.frame,p.before,ppuhex()))
 end)end
end)
STAGING
function finish_probe()assert(not pending,'unfinished immediate background');f:close()end
'''
def main():
 p=argparse.ArgumentParser();p.add_argument('--label',required=True);a=p.parse_args();assets=json.loads((ROOT/'docs/research/nes-contra/evidence/campaign-scene-assets.json').read_text());captures=[]
 for kind,stage in [('natural',0),('interlude',0),('interlude',5)]:
  name=f'background-{a.label}-{kind}-{stage+1}';staging='';spec={'name':name,'frames':1800,'segments':[],'captures':[800,1200,1700],'timeout_seconds':120}
  if kind=='natural':spec['segments']=[{'start':100,'end':102,'keys':['select']},{'start':120,'end':122,'keys':['start']},{'start':720,'end':722,'keys':['start']}]
  else:
   (BASE/'states').mkdir(parents=True,exist_ok=True);src=ROOT/'.local/contra/states/coop-play.fcs';target=BASE/'states/coop-play.fcs'
   if not target.exists():shutil.copyfile(src,target)
   assert src.read_bytes()==target.read_bytes();spec['load_state']='coop-play'
   staging=f"local staged=false;memory.registerexec(0xcc42,1,function()if staged then return end;staged=true;memory.writebyte(0x2c,10);memory.writebyte(0x2d,0);memory.writebyte(0x30,{stage});memory.writebyte(0x3b,2)end)"
  spec['hooks_lua']=HOOK.replace('STAGING',staging);BASE.mkdir(parents=True,exist_ok=True);path=BASE/(name+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');out=run(path);rows=[json.loads(l)for l in (out/'backgrounds.jsonl').read_text().splitlines()]
  for row in rows:
   expected=bytearray.fromhex(row['before']);art=assets['backgrounds'][str(row['index'])]
   for write in art['writes']:
    for offset,value in enumerate(write['values']):expected[(write['address']-0x2000+offset)%2048]=value
   assert expected.hex()==row['after'],(name,row['index'],row['frame'])
  captures.append({'name':name,'scope':'Natural power-on controls'if kind=='natural'else'Declared one-time interlude phase/level input; no subsequent write','source':json.loads((out/'source.json').read_text()),'records':rows})
 payload={'passed':True,'scope':'All recovered direct background data compared with original PPU contents; no snapshots drive implementation.','assetsSHA256':hashlib.sha256((ROOT/'docs/research/nes-contra/evidence/campaign-scene-assets.json').read_bytes()).hexdigest(),'captures':captures}
 indices=sorted({r['index']for c in captures for r in c['records']});assert indices==[0,14,16,18]
 target=ROOT/'docs/research/nes-contra/evidence/campaign-scene-background-original.json.gz';target.write_bytes(gzip.compress(json.dumps(payload,separators=(',',':')).encode(),mtime=0));print(json.dumps({'passed':True,'backgrounds':indices,'calls':sum(len(c['records'])for c in captures)}))
if __name__=='__main__':main()
