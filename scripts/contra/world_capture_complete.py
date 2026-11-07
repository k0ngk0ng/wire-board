"""Record complete active-world work through next OAM construction.

This is an original execution oracle. Raw RAM is retained as test evidence, not
as the shipping engine's state or executable runtime. The pre-frame random byte
is an explicit entropy input for rule comparison; the platform sequence is an
independently verified, user-authorized adapter.

Failures tracked before integration: dropping cross-phase work values; comparing
next OAM to current display; missing reset/sound ordering; resetting state every
frame instead of advancing a continuous segment; unlogged controller changes.
"""
import gzip,hashlib,json,os,shutil,sys
from pathlib import Path

ROOT=Path(__file__).resolve().parents[2]
os.environ.setdefault('CONTRA_REFERENCE_ROOT',str(ROOT/'.local/contra-world/complete-oracle'))
from capture_reference import BASE,run

HOOK=r'''
local records=assert(io.open(out..'/world-frames.jsonl','w'))
local state=assert(io.open(out..'/world-frames.bin','wb'))
local pending=nil;local ordinal=0
memory.registerexec(0xc001,1,function()
 if memory.readbyte(0x18)~=5 or memory.readbyte(0x2c)~=4 then return end
 assert(not pending,'nested world frame')
 pending={frame=emu.framecount(),before=memory.readbyterange(0,2048),commands={}}
end)
memory.registerexec(0xf9bc,1,function()
 if pending then pending.commands[#pending.commands+1]=string.format('{"kind":"sound","id":%d}',memory.getregister('a')) end
end)
memory.registerexec(0xf9d4,1,function()
 if pending then pending.commands[#pending.commands+1]='{"kind":"reset"}' end
end)
memory.registerexec(0xf8db,1,function()
 if not pending then return end
 local p=pending;pending=nil
 state:write(p.before);state:write(memory.readbyterange(0,2048))
 records:write(string.format('{"index":%d,"emulatorFrame":%d,"commands":[%s]}\n',ordinal,p.frame,table.concat(p.commands,',')))
 ordinal=ordinal+1
end)
function finish_probe()assert(not pending);records:close();state:close()end
'''

def main():
 label=sys.argv[1] if len(sys.argv)>1 else 'v1'
 (BASE/'states').mkdir(parents=True,exist_ok=True)
 src=ROOT/'.local/contra/states/coop-play.fcs';dst=BASE/'states/coop-play.fcs'
 if dst.exists():assert src.read_bytes()==dst.read_bytes()
 else:shutil.copyfile(src,dst)
 spec={'name':f'complete-world-{label}','load_state':'coop-play','frames':600,'captures':[1,120,320,600],
       'segments':[{'start':1,'end':600,'player':p,'keys':['right','B']}for p in (1,2)]+
                  [{'start':n,'end':n+1,'player':p,'keys':['A']}for p in (1,2)for n in range(60,600,90)],
       'hooks_lua':HOOK,'timeout_seconds':90}
 path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');out=run(path)
 rows=[json.loads(line)for line in (out/'world-frames.jsonl').read_text().splitlines()]
 states=(out/'world-frames.bin').read_bytes();assert len(states)==len(rows)*4096
 for i,row in enumerate(rows):
  row['before']=states[i*4096:i*4096+2048].hex();row['after']=states[i*4096+2048:(i+1)*4096].hex()
 data={'source_is_original_execution':True,'scope':'Original normal-stage world call plus OAM composition, with controller-only inputs; no independent-world or full-campaign completion claim.',
       'source':json.loads((out/'source.json').read_text()),'scenario':spec,
       'snapshot':'2 KiB source RAM, before C001 and after world/sprite work at F8DB; stored only in test evidence.',
       'inputTraceSHA256':hashlib.sha256((out/'inputs.csv').read_bytes()).hexdigest(),
       'ramSHA256':hashlib.sha256((out/'ram.bin').read_bytes()).hexdigest(),'frames':rows}
 target=ROOT/'docs/research/nes-contra/evidence/world-complete-original.json.gz'
 if target.exists():
  old=json.loads(gzip.decompress(target.read_bytes()));assert old['frames']==rows and old['ramSHA256']==data['ramSHA256']
 else:target.write_bytes(gzip.compress(json.dumps(data,separators=(',',':')).encode(),mtime=0))
 baseline=ROOT/'.local/contra-background/runs/background-v1/ram.bin'
 assert baseline.read_bytes()==(out/'ram.bin').read_bytes(),'complete hooks changed original natural run'
 print(json.dumps({'frames':len(rows),'sourceRAMUnchanged':True,'sha256':hashlib.sha256(target.read_bytes()).hexdigest()}))

if __name__=='__main__':main()
