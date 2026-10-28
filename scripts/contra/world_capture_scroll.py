"""Observe the original terrain-scroll phase and its queued graphics writes.

Failure inventory: collision changes delayed with graphics; wrong half-screen
lookahead decode; attributes scheduled with tile columns instead of at +3;
wrong nametable switch or enemy-stream reset; overwriting partial map windows.
"""
import gzip,hashlib,json,os,shutil,sys
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2]
os.environ.setdefault('CONTRA_REFERENCE_ROOT',str(ROOT/'.local/contra-world/scroll-oracle'))
from capture_reference import BASE,run

FIELDS=[('level',0x30),('sceneType',0x40),('scrollType',0x41),('alternateGraphics',0x48),
 ('column',0x60),('loadCountdown',0x61),('tileLo',0x62),('tileHi',0x63),('screen',0x64),('screenScroll',0x65),
 ('attributeLo',0x66),('attributeHi',0x67),('frameScroll',0x68),('mapOffset',0x69),
 ('autoA',0x75),('autoB',0x76),('tankScroll',0x77),('enemyOffset',0x82),('bossScrollComplete',0x84),
 ('scrollY',0xfc),('scrollX',0xfd),('ppuControl',0xff)]
HOOK=r'''
local file=assert(io.open(out..'/scroll-phase.jsonl','w'))
local fields=FIELDS;local pending=nil;local exits={}
local function read(a)return memory.readbyte(a)end
local function hex(a,n)local t={};for i=a,a+n-1 do t[#t+1]=string.format('%02x',read(i))end;return table.concat(t)end
local function snapshot()
 local values={};for _,a in ipairs(fields)do values[#values+1]=tostring(read(a))end
 return string.format('{"state":[%s],"maps":"%s","collision":"%s","queue":"%s"}',table.concat(values,','),hex(0x600,128),hex(0x680,128),hex(0x700,read(0x21)))
end
memory.registerexec(0xfa3a,1,function()
 assert(not pending);local s=memory.getregister('s')
 local pc=read(0x100+AND(s+1,255))+256*read(0x100+AND(s+2,255))+1
 pending={pc=pc,sp=AND(s+2,255),frame=emu.framecount(),before=snapshot()}
 if not exits[pc] then
  exits[pc]=true
  memory.registerexec(pc,1,function()
   if pending and pending.pc==pc and pending.sp==memory.getregister('s')then
    local p=pending;pending=nil
    file:write(string.format('{"frame":%d,"before":%s,"after":%s}\n',p.frame,p.before,snapshot()))
   end
  end)
 end
end)
function finish_probe()assert(not pending);file:close()end
'''

def main():
 label=sys.argv[1] if len(sys.argv)>1 else 'v1'
 (BASE/'states').mkdir(parents=True,exist_ok=True)
 src=ROOT/'.local/contra/states/coop-play.fcs';dst=BASE/'states/coop-play.fcs'
 if dst.exists():assert dst.read_bytes()==src.read_bytes()
 else:shutil.copyfile(src,dst)
 spec={'name':'scroll-phase-'+label,'load_state':'coop-play','frames':600,'captures':[1,300,600],
  'segments':[{'start':1,'end':600,'player':p,'keys':['right','B']}for p in (1,2)]+
             [{'start':n,'end':n+1,'player':p,'keys':['A']}for p in (1,2)for n in range(60,600,90)],
  'hooks_lua':HOOK.replace('FIELDS','{'+','.join(hex(a)for _,a in FIELDS)+'}'),'timeout_seconds':90}
 path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');observed=run(path)
 records=[json.loads(line)for line in (observed/'scroll-phase.jsonl').read_text().splitlines()]
 result={'source_is_original_execution':True,'scope':'Original horizontal stage-one scroll phase, controller input only.',
  'source':json.loads((observed/'source.json').read_text()),'fields':[{'name':n,'address':hex(a)}for n,a in FIELDS],
  'scenario':spec,'records':records,'ramSHA256':hashlib.sha256((observed/'ram.bin').read_bytes()).hexdigest()}
 target=ROOT/'docs/research/nes-contra/evidence/world-scroll-original.json.gz'
 if target.exists():
  previous=json.loads(gzip.decompress(target.read_bytes()));assert previous['records']==records and previous['ramSHA256']==result['ramSHA256']
 else:target.write_bytes(gzip.compress(json.dumps(result,separators=(',',':')).encode(),mtime=0))
 print(json.dumps({'records':len(records),'sha256':hashlib.sha256(target.read_bytes()).hexdigest()}))

if __name__=='__main__':main()
