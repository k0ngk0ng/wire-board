"""Capture original campaign decision boundaries before independent implementation.

Failures: confusing JP routine 0B with US 0A; counting the current continue offer
twice; resetting score/high-score or weapons at the wrong transition; boss clear
beating a simultaneous full wipe; treating FF delay sentinel as an ordinary
countdown; P2/menu button edge confusion; game over advancing while paused;
claiming a controlled boundary record as a natural campaign clear.
"""
import os
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2]
os.environ.setdefault('CONTRA_REFERENCE_ROOT',str(ROOT/'.local/contra-campaign-rules/oracle'))
from capture_reference import BASE,run
import argparse,gzip,hashlib,json,shutil

HOOK=r'''
local output=assert(io.open(out.."/campaign.jsonl","w"));local pending={};local exits={};local activeCase=0;local restore=nil;local hooks={};local recording=false
local function on(pc,callback)
 if not hooks[pc] then hooks[pc]={};memory.registerexec(pc,1,function()for _,fn in ipairs(hooks[pc])do fn()end end)end
 hooks[pc][#hooks[pc]+1]=callback
end
local function read(a)return memory.readbyte(a)end
local function word(a)return read(a)+256*read(a+1)end
local function snapshot()
 return string.format('{"gameRoutine":%d,"levelRoutine":%d,"endSequence":%d,"level":%d,"loops":%d,"continues":%d,"boss":%d,"delayLow":%d,"delayHigh":%d,"gameOverDelay":%d,"paused":%d,"mode":%d,"pauseBlocked":%d,"pressed":%d,"choice":%d,"aliveAtExit":%d,"activeFlags":%d,"cheatLives":%d,"highScore":%d,"score":[%d,%d],"nextLife":[%d,%d],"players":[{"state":%d,"gameOver":%d,"lives":%d,"weapon":%d},{"state":%d,"gameOver":%d,"lives":%d,"weapon":%d}]}',read(0x18),read(0x2c),read(0x2d),read(0x30),read(0x31),read(0x3a),read(0x3b),read(0x2a),read(0x2b),read(0x29),read(0x25),read(0x22),OR(read(0x1c),read(0x26),read(0x20)),read(0xf5),read(0x6b),read(0x194),read(0x1d),read(0x24),word(0x7e0),word(0x7e2),word(0x7e4),word(0x3c),word(0x3e),read(0x90),read(0x38),read(0x32),read(0xaa),read(0x91),read(0x39),read(0x33),read(0xab))
end
local function finish(name)
 local p=pending[name];if not p then return end;pending[name]=nil
 output:write(string.format('{"kind":"%s","case":%d,"frame":%d,"exit":%d,"before":%s,"after":%s,"sounds":[%s],"effects":[%s]}\n',name,p.case,p.frame,memory.getregister('pc'),p.before,snapshot(),table.concat(p.sounds,','),table.concat(p.effects,',')))
end
local function returnpc()local sp=memory.getregister('s');return read(0x100+AND(sp+1,255))+256*read(0x100+AND(sp+2,255))+1 end
local function arm(pc)
 if exits[pc] then return end;exits[pc]=true
 on(pc,function()
  local ended={};for name,p in pairs(pending)do if p.pc==pc then ended[#ended+1]=name end end
  table.sort(ended);for _,name in ipairs(ended)do finish(name)end
  if restore and restore.pc==pc and next(pending)==nil then
   for a,v in pairs(restore.bytes)do memory.writebyte(a,v)end;restore=nil;recording=false
  end
 end)
end
local function enter(name)
 if not recording then return end
 assert(not pending[name],name..' nested')
 local pc=returnpc();pending[name]={pc=pc,case=activeCase,frame=emu.framecount(),before=snapshot(),sounds={},effects={}};arm(pc)
end
on(0xf9bc,function()for _,p in pairs(pending)do p.sounds[#p.sounds+1]=tostring(memory.getregister('a'))end end)
on(0xf9d4,function()for _,p in pairs(pending)do p.effects[#p.effects+1]='"stopAudio"'end end)
on(0xc309,function()for _,p in pairs(pending)do p.effects[#p.effects+1]=string.format('"clearFrom%d"',memory.getregister('x'))end end)
local cases=CASES
on(0xcc42,function()
 if not cases then return end
 assert(not restore and next(pending)==nil,'unfinished boundary case')
 if activeCase>=#cases then recording=false;return end
 activeCase=activeCase+1;local values=cases[activeCase];recording=true
 restore={pc=returnpc(),bytes={}}
 -- Preserve gameplay RAM and the persistent field $0194 below the live stack;
 -- do not rewrite the current NMI return frames in $01E0..$01FF.
 for a=0x18,0xff do restore.bytes[a]=read(a)end
 for a=0x180,0x1df do restore.bytes[a]=read(a)end
 for a=0x300,0x7ff do restore.bytes[a]=read(a)end
 for a,v in pairs(values)do memory.writebyte(a,v)end
 for i=0,15 do memory.writebyte(0x4b8+i,0)end
 arm(restore.pc)
end)
for _,pair in ipairs({{'initialize',0xcc5f},{'prepareBattle',0xcc7d},{'beginPlaying',0xcccc},{'afterCombat',0xcce2},{'clearWait',0xcdfb},{'gameOverWait',0xce52},{'transition',0xcd1f},{'continueMenu',0xcdaf},{'exhaustedMenu',0xcdf1},{'resetCounters',0xc2cf},{'pause',0xce94},{'delay',0xc177}})do
 local name,pc=pair[1],pair[2]
 on(pc,function()enter(name)end)
end
on(0xccdc,function()finish('beginPlaying')end)
on(0xcce8,function()finish('afterCombat')end)
function finish_probe()assert(next(pending)==nil and not restore,'incomplete campaign boundary');assert(activeCase==#cases,'incomplete controlled cases');output:close()end
'''

def cases():
    base={0x18:5,0x2c:4,0x2d:0,0x30:0,0x31:0,0x3a:3,0x3b:0,0x2a:0,0x2b:0,0x29:96,0x25:0,0x22:1,0x1c:0,0x26:0,0x20:0,0xf5:0,0xf6:0,0x6b:0,0x194:0,0x1d:7,0x24:0,0x38:0,0x39:0,0x32:2,0x33:2,0x90:1,0x91:1,0xaa:17,0xab:4,0x7e0:0x2c,0x7e1:1,0x7e2:100,0x7e3:0,0x7e4:250,0x7e5:0,0x3c:0x90,0x3d:1,0x3e:0xbc,0x3f:2}
    result=[]
    def add(**fields):
        row=dict(base);row.update({int(k,16):v for k,v in fields.items()});result.append(row)
    for boss in [0,1]:
        for dead in [0,1]:
            for paused in [0,1]:
                for pressed in [0,16]:add(**{'3b':boss,'38':dead,'39':dead,'25':paused,'f5':pressed})
    for blocked in [1,2]:
        for paused in [0,1]:add(**{'20':blocked,'25':paused,'f5':16})
    for level in range(8):
        for dead in [0,1]:add(**{'2c':5,'30':level,'3b':1,'38':dead,'39':0,'32':0 if dead else 2,'31':0})
    for continues in [3,2,1,0]:add(**{'2c':5,'3a':continues,'38':1,'39':1,'32':0,'33':0})
    for choice in [0,1]:
        for mode in [0,1]:
            for pressed in [0,16,32,48]:add(**{'2c':6,'3a':2,'6b':choice,'22':mode,'f5':pressed,'38':1,'39':1,'32':0,'33':0})
    for pressed in [0,16]:add(**{'2c':7,'3a':255,'f5':pressed,'38':1,'39':1,'32':0,'33':0})
    for low in [0,1,2,255]:
        for high in [0,1]:
            for state in [0,1,2,3]:add(**{'2c':8,'3b':1,'2a':low,'2b':high,'90':state,'91':state})
    add(**{'2c':8,'3b':1,'38':1,'39':1,'32':0,'33':0})
    for level in [4,5,7]:add(**{'2c':8,'3b':1,'30':level})
    for delay in [0,1,2,96]:add(**{'2c':11,'29':delay,'3b':1,'38':1,'39':1,'32':0,'33':0})
    for low in [0,1,2,64,255]:
        for high in [0,1]:add(**{'2c':2,'2a':low,'2b':high})
    for phase in [0,2]:add(**{'2c':phase,'3b':129,'194':2,'2d':7})
    for level in range(1,8):add(**{'2c':2,'30':level})
    return result

def main():
    parser=argparse.ArgumentParser();parser.add_argument('--name',required=True);args=parser.parse_args()
    (BASE/'states').mkdir(parents=True,exist_ok=True)
    original=ROOT/'.local/contra/states/coop-play.fcs';target=BASE/'states/coop-play.fcs'
    if not target.exists():shutil.copyfile(original,target)
    assert target.read_bytes()==original.read_bytes()
    prepared=cases();lua='{'+','.join('{'+','.join(f'[{a}]={v}' for a,v in row.items())+'}' for row in prepared)+'}'
    spec={'name':args.name,'load_state':'coop-play','frames':len(prepared)+64,'segments':[],'captures':[],'hooks_lua':HOOK.replace('CASES',lua),'timeout_seconds':90}
    path=BASE/(args.name+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');out=run(path)
    records=[json.loads(line) for line in (out/'campaign.jsonl').read_text().splitlines()]
    payload={'scope':'Declared campaign boundary interventions; not a natural complete run.','source':json.loads((out/'source.json').read_text()),'cases':[{hex(a):v for a,v in row.items()} for row in prepared],'scenario':spec,'records':records}
    data=gzip.compress(json.dumps(payload,separators=(',',':')).encode(),mtime=0)
    dest=ROOT/'docs/research/nes-contra/evidence/campaign-boundaries-original.json.gz';dest.write_bytes(data)
    print(json.dumps({'cases':len(prepared),'records':len(records),'sha256':hashlib.sha256(data).hexdigest(),'counts':{kind:sum(r['kind']==kind for r in records) for kind in sorted(set(r['kind'] for r in records))}}))

if __name__=='__main__':main()
