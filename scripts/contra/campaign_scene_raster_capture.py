"""Read-only sprite-hit/PPU-write timing for original Intro and Interlude.

No state intervention is introduced: reuse the verified reserve30 controller
route verbatim, replace its hooks with observation, and require full-RAM identity.
"""
import os,gzip,json,hashlib,shutil,argparse
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2]
os.environ['CONTRA_REFERENCE_ROOT']=str(ROOT/'.local/contra-raster/oracle')
from capture_reference import BASE,run
HOOK=r'''
local log=assert(io.open(out..'/raster.jsonl','w'));local staged=false
local function rb(a)return memory.readbyte(a)end
local function cycles()return debugger.getcyclescount()end
local lastNmi=0;local lastMiss={};local hit={}
local function record(kind)
 local pc=memory.getregister('pc');local t={};for a=0x200,0x203 do t[#t+1]=tostring(rb(a))end
 log:write(string.format('{"kind":"%s","frame":%d,"cycles":%d,"nmiCycles":%d,"pc":%d,"a":%d,"status":%d,"phase":%d,"sequence":%d,"level":%d,"busy":%d,"scroll":%d,"shadowY":%d,"shadowX":%d,"control":%d,"mask":%d,"oam0":[%s]}\n',kind,emu.framecount(),cycles(),lastNmi,pc,memory.getregister('a'),memory.getregister('p'),rb(0x2c),rb(0x2d),rb(0x30),rb(0x20),rb(0x70),rb(0xfc),rb(0xfd),rb(0xff),rb(0xfe),table.concat(t,',')))
end
memory.registerexec(0xf889,1,function()lastNmi=cycles();if rb(0x18)==5 then record('nmi')end end)
memory.registerexec(0xc001,1,function()
 if staged then return end;staged=true
 local l0,l1=rb(0x32),rb(0x33);memory.writebyte(0x32,30);memory.writebyte(0x33,30)
 writefile('interventions.json',string.format('[{"address":50,"before":%d,"after":30},{"address":51,"before":%d,"after":30}]',l0,l1))
end)
for _,pc in ipairs({0xf968,0xf974,0xf980,0xf985,0xf988,0x9fc6,0x9fe1,0x9fe5,0x9fec,0x9fef,0xf926,0xf940,0xf945})do
 local at=pc;memory.registerexec(at,1,function()if rb(0x18)==5 then record(string.format('pc-%04x',at))end end)
end
for _,pc in ipairs({0xf971,0x9fd8,0x9fde})do
 local at=pc;memory.registerexec(at,1,function()
  if rb(0x18)~=5 then return end
  if AND(memory.getregister('a'),64)==0 then lastMiss[at]=cycles()
  else log:write(string.format('{"kind":"hit-observed","frame":%d,"pc":%d,"cycles":%d,"lastMiss":%d}\n',emu.framecount(),at,cycles(),lastMiss[at]or-1))end
 end)
end
function finish_probe()log:close()end
'''
def main():
 p=argparse.ArgumentParser();p.add_argument('--label',required=True);a=p.parse_args()
 original=json.loads(gzip.decompress((ROOT/'docs/research/nes-contra/evidence/world-boss-original.json.gz').read_bytes()));spec=dict(original['scenario']);spec['name']='raster-'+a.label
 # Retain exactly the previously declared controller function after the probe.
 control=spec['hooks_lua'][spec['hooks_lua'].index('function control_frame'):]
 spec['hooks_lua']=HOOK+'\n'+control;spec['captures']=[6475,6500,6550,6600,7200,7250,7300,7500,7800];spec['timeout_seconds']=180
 (BASE/'states').mkdir(parents=True,exist_ok=True);src=ROOT/'.local/contra/states/coop-play.fcs';dst=BASE/'states/coop-play.fcs'
 if not dst.exists():shutil.copyfile(src,dst)
 assert src.read_bytes()==dst.read_bytes();path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');out=run(path)
 assert hashlib.sha256((out/'ram.bin').read_bytes()).hexdigest()==original['ramSHA256']
 events=[json.loads(l)for l in (out/'raster.jsonl').read_text().splitlines()]
 target=ROOT/'docs/research/nes-contra/evidence/campaign-scene-raster-original.json.gz';target.write_bytes(gzip.compress(json.dumps({'source':json.loads((out/'source.json').read_text()),'scope':'Read-only timing hooks on the existing reserve30 route; entire RAM unchanged.','events':events},separators=(',',':')).encode(),mtime=0));print(json.dumps({'events':len(events),'RAMUnchanged':True}))
if __name__=='__main__':main()
