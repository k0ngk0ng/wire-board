"""Capture every hurt/knockdown actor call with full original RAM evidence."""
import hashlib,json,csv
from capture_reference import BASE,ROOT,run
HOOK=r'''
local f=assert(io.open(out..'/hurt-ram.bin','wb'))
local c=assert(io.open(out..'/hurt.csv','w'));c:write('frame,actor\n')
local pending=nil
local function ram()
 local parts={};for i=0,2047 do parts[#parts+1]=string.char(memory.readbyte(i)) end
 return table.concat(parts)
end
memory.registerexec(0xe165,1,function()
 local i=memory.getregister('x');if i>3 then return end
 local a=AND(memory.readbyte(0x512+i),127)
 if a<0x47 or a>0x4d then return end
 assert(not pending);pending=i;f:write(ram());c:write(string.format('%d,%d\n',emu.framecount(),i))
end)
memory.registerexec(0xe168,1,function()
 if pending==nil then return end
 f:write(ram());pending=nil
end)
function finish_probe() assert(pending==nil);f:close();c:close() end
'''

def decode(r,i):
 def word(a,b):return r[a]+256*r[b]
 def wide(a,b,c):return word(a,b)+65536*r[c]
 names=dict(action=0x512,request=0x51f,facing=0x46b,pose=0x5b,clock=0x437,cursor=0x444,repeatCount=0x451,nextAction=0x52c,air=0x5f7,attachment=0x642,heading=0x4ec,passage=0x6d7,health=0x611,recoveryGrade=0x621,linkedActor=0x550,control=0x539)
 return dict(**{n:r[a+i] for n,a in names.items()},x=wide(0x68+i,0x75+i,0x82+i),depth=word(0x8f+i,0x9c+i),height=wide(0xa9+i,0xb6+i,0xc3+i),vx=word(0x478+i,0x485+i),vy=word(0x492+i,0x49f+i),vz=word(0x4ac+i,0x4b9+i),gravity=word(0x4c6+i,0x4d3+i),holder=r[0x53f],specialHolder=r[0x56b],assistedActor=r[0x570],attachments=list(r[0x642:0x646]),kinds=list(r[0x3d6:0x3e3]),actions=list(r[0x512:0x51f]),facings=list(r[0x46b:0x478]),owners=list(r[0x539:0x546]),ballVX=word(0x47e,0x48b),random=list(r[0x48:0x4c]))

def main():
 reference=BASE/'runs/hoop-control-cpu-long'
 spec=json.loads((reference/'scenario.json').read_text());spec.update(name='hurt-player-cpu-long',hooks_lua=HOOK)
 path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');run(path)
 observed=BASE/'runs'/spec['name'];assert (observed/'ram.bin').read_bytes()==(reference/'ram.bin').read_bytes()
 raw=(observed/'hurt-ram.bin').read_bytes();rows=list(csv.DictReader((observed/'hurt.csv').open()));assert len(raw)==len(rows)*4096
 result=[dict(frame=int(v['frame']),actor=int(v['actor']),before=decode(raw[j*4096:j*4096+2048],int(v['actor'])),after=decode(raw[j*4096+2048:(j+1)*4096],int(v['actor']))) for j,v in enumerate(rows)]
 fixture=ROOT/'games/basketball/engine/testdata/hurt-player.json';fixture.write_text(json.dumps(result,separators=(',',':'))+'\n')
 fixture.with_name('hurt-player-source.json').write_text(json.dumps(dict(source_is_original_execution=True,records=len(result),sha256=hashlib.sha256(fixture.read_bytes()).hexdigest(),trace_sha256=hashlib.sha256(raw).hexdigest(),source=json.loads((observed/'source.json').read_text())),indent=2)+'\n');print('hurt-player',len(result))

if __name__=='__main__':main()
