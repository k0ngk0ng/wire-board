"""Record full startup world calls and actual PPU state, without RAM patches.

Integration risks: text parser vs rectangle parser, direct vs deferred graphics,
display-disable/busy transitions, source registers reused as cinematic state,
premature CHR/palette submission, duplicate startup audio, and skipping intro.
"""
import gzip,hashlib,json,os,sys,shutil
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2]
os.environ['CONTRA_REFERENCE_ROOT']=str(ROOT/'.local/contra-world/start-oracle')
from world_capture_complete import HOOK
from capture_reference import BASE,run

def main():
    label=sys.argv[1] if len(sys.argv)>1 else 'v1'
    mode=sys.argv[2] if len(sys.argv)>2 else 'start'
    assert mode in ('start','continue','quit')
    hook=HOOK.replace('local pending=nil;local ordinal=0',r'''
local ppuout=assert(io.open(out..'/world-ppu.bin','wb'))
local function ppustate()
 local bytes={}
 for a=0x2000,0x27ff do bytes[#bytes+1]=string.char(ppu.readbyte(a))end
 for a=0x3f00,0x3f1f do bytes[#bytes+1]=string.char(ppu.readbyte(a))end
 return table.concat(bytes)
end
local pending=nil;local ordinal=0''')
    hook=hook.replace('if memory.readbyte(0x18)~=5 or memory.readbyte(0x2c)~=4 then return end','if memory.readbyte(0x18)~=5 or memory.readbyte(0x30)~=0 then return end')
    hook=hook.replace('before=memory.readbyterange(0,2048),commands={}','before=memory.readbyterange(0,2048),beforePPU=ppustate(),sp=AND(memory.getregister("s")+2,255),commands={}')
    # Background copies explicitly wait across NMI. Its nested F8DB is not the
    # return of this C001 call; match the saved stack depth before completing it.
    hook=hook.replace('if not pending then return end','if not pending or memory.getregister("s")~=pending.sp then return end')
    hook=hook.replace('local p=pending;pending=nil','local p=pending;pending=nil;ppuout:write(p.beforePPU);ppuout:write(ppustate())')
    hook=hook.replace('records:close();state:close()','records:close();state:close();ppuout:close()')
    spec={'name':f'world-start-{label}','frames':1800,'captures':[400,800,1100,1400,1800],
          'segments':[{'start':100,'end':102,'keys':['select']},{'start':120,'end':122,'keys':['start']},{'start':720,'end':722,'keys':['start']}],
          'hooks_lua':hook,'timeout_seconds':120}
    if mode!='start':
        (BASE/'states').mkdir(parents=True,exist_ok=True)
        src=ROOT/'.local/contra/states/coop-play.fcs';dst=BASE/'states/coop-play.fcs'
        if dst.exists():assert dst.read_bytes()==src.read_bytes()
        else:shutil.copyfile(src,dst)
        hook=hook.replace('memory.registerexec(0xc001,1,function()',r'''
local staged=false
memory.registerexec(0xc001,1,function()
 if not staged then
  staged=true
  local a=memory.readbyte(0x32);local b=memory.readbyte(0x33)
  memory.writebyte(0x32,0);memory.writebyte(0x33,0)
  writefile('interventions.json',string.format('[{"address":50,"before":%d,"after":0},{"address":51,"before":%d,"after":0}]',a,b))
 end''')
        hook+=r'''
local menuFrames=0;local acted=false;local skipped=false
function control_frame(n,pads)
 for i=1,2 do pads[i]={right=true,left=false,up=false,down=false,A=false,B=false,start=false,select=false}end
 if memory.readbyte(0x2c)==6 and not acted then
  menuFrames=menuFrames+1
  if QUIT and menuFrames==6 then pads[1].select=true end
  if menuFrames==12 then pads[1].start=true;acted=true end
 elseif acted and not QUIT and memory.readbyte(0x2c)==1 and not skipped then
  pads[1].start=true;skipped=true
 end
end
'''.replace('QUIT','true' if mode=='quit' else 'false')
        spec={'name':f'world-{mode}-{label}','load_state':'coop-play','frames':900,
              'captures':[1,277,374,385,450,900],'segments':[],'hooks_lua':hook,'timeout_seconds':120}
    BASE.mkdir(parents=True,exist_ok=True);path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');out=run(path)
    rows=[json.loads(line) for line in (out/'world-frames.jsonl').read_text().splitlines()]
    states=(out/'world-frames.bin').read_bytes();ppu=(out/'world-ppu.bin').read_bytes()
    assert len(states)==len(rows)*4096 and len(ppu)==len(rows)*4160
    phases={}
    for i,row in enumerate(rows):
        before=states[i*4096:i*4096+2048];phases[before[0x2c]]=phases.get(before[0x2c],0)+1
        row['before']=before.hex();row['after']=states[i*4096+2048:(i+1)*4096].hex()
        row['beforePPU']=ppu[i*4160:i*4160+2080].hex();row['afterPPU']=ppu[i*4160+2080:(i+1)*4160].hex()
    if mode=='start':assert set(phases)=={0,1,2,3,4}
    else:assert 5 in phases and 6 in phases and 11 in phases
    scope='Controller-only reset-to-startup; no memory interventions.' if mode=='start' else f'One initial reserve-lives=0 intervention, then actual death and controller-only {mode} menu choice. No outcome injection.'
    data={'source_is_original_execution':True,'scope':scope+' Full C001 through matching-stack F8DB RAM and nametable/palette PPU observations.',
          'source':json.loads((out/'source.json').read_text()),'scenario':spec,'frames':rows,'phases':phases,
          'inputTraceSHA256':hashlib.sha256((out/'inputs.csv').read_bytes()).hexdigest(),
          'ramSHA256':hashlib.sha256((out/'ram.bin').read_bytes()).hexdigest()}
    if mode!='start':data['interventions']=json.loads((out/'interventions.json').read_text())
    target=ROOT/f'docs/research/nes-contra/evidence/world-{mode}-original.json.gz';repeated=target.exists()
    if repeated:
        old=json.loads(gzip.decompress(target.read_bytes()));assert old['frames']==rows and old['ramSHA256']==data['ramSHA256']
    else:target.write_bytes(gzip.compress(json.dumps(data,separators=(',',':')).encode(),mtime=0))
    print(json.dumps({'frames':len(rows),'phases':phases,'repeatedIdentically':repeated,'sha256':hashlib.sha256(target.read_bytes()).hexdigest()}))

if __name__=='__main__':main()
