"""Capture original action-58 players and action-10 balls with full RAM evidence."""
import hashlib,json,csv,sys
from capture_reference import BASE,ROOT,run
HOOK=r'''
local f=assert(io.open(out..'/special-shot-ram.bin','wb'))
local c=assert(io.open(out..'/special-shot.csv','w'));c:write('frame,actor,kind\n')
local pending=nil
local function ram()
 local parts={};for i=0,2047 do parts[#parts+1]=string.char(memory.readbyte(i)) end
 return table.concat(parts)
end
memory.registerexec(0xe165,1,function()
 local i=memory.getregister('x');if i>3 then return end
 local a=AND(memory.readbyte(0x512+i),127)
 local q=memory.readbyte(0x51f+i)
 if a~=0x1b and a~=0x1c and q~=0x1b and q~=0x1c then return end
 assert(not pending);pending=i;f:write(ram());c:write(string.format('%d,%d,player\n',emu.framecount(),i))
end)
memory.registerexec(0xe168,1,function()
 if pending==nil then return end
 f:write(ram());pending=nil
end)
memory.registerexec(0xe149,1,function()
 local a=AND(memory.readbyte(0x518),127)
 if a~=4 and (a<12 or a>23) then return end
 assert(not pending);pending=6;f:write(ram());c:write(string.format('%d,6,ball\n',emu.framecount()))
end)
memory.registerexec(0xe14c,1,function()
 if pending==nil then return end
 assert(pending==6);f:write(ram());pending=nil
end)
function finish_probe() assert(pending==nil);f:close();c:close() end
'''

def decode(r,i):
 def word(a,b):return r[a]+256*r[b]
 def wide(a,b,c):return word(a,b)+65536*r[c]
 names=dict(action=0x512,request=0x51f,facing=0x46b,pose=0x5b,clock=0x437,cursor=0x444,repeatCount=0x451,nextAction=0x52c,air=0x5f7,attachment=0x642,heading=0x4ec,passage=0x6d7,health=0x611,recoveryGrade=0x621,linkedActor=0x550,control=0x539)
 return dict(**{n:r[a+i] for n,a in names.items()},x=wide(0x68+i,0x75+i,0x82+i),depth=word(0x8f+i,0x9c+i),height=wide(0xa9+i,0xb6+i,0xc3+i),vx=word(0x478+i,0x485+i),vy=word(0x492+i,0x49f+i),vz=word(0x4ac+i,0x4b9+i),gravity=word(0x4c6+i,0x4d3+i),holder=r[0x53f],specialHolder=r[0x56b],assistedActor=r[0x570],attachments=list(r[0x642:0x646]),kinds=list(r[0x3d6:0x3e3]),actions=list(r[0x512:0x51f]),facings=list(r[0x46b:0x478]),owners=list(r[0x539:0x546]),ballVX=word(0x47e,0x48b),random=list(r[0x48:0x4c]),targetObject=r[0x56e],teams=list(r[0x586:0x588]),characters=list(r[0x3e3:0x3e7]),playerPositions=[dict(x=wide(0x68+j,0x75+j,0x82+j),depth=word(0x8f+j,0x9c+j),height=wide(0xa9+j,0xb6+j,0xc3+j)) for j in range(4)],shotCycle=r[0x572+i],specialShot=r[0x554+i],specialShots=list(r[0x554:0x558]),zone=r[0x66a+i] if i<4 else 0,team=r[0x586+i//2] if i<4 else 0,character=r[0x3e3+i] if i<4 else 0,enhancementEnabled=r[0x58d]<128,shotEquipment=list(r[0x6e4:0x6f0:3]),held=list(r[0x4e0:0x4e4]),pressed=list(r[0x4e4:0x4e8]),mountFlags=list(r[0x6f0:0x6f6]),launchTarget=dict(x=(r[0x69f]+256*r[0x6a0])*256,depth=r[0x6a1]*256,height=(r[0x6a2]+256*r[0x6a3])*256),launchArc=r[0x6a4]*256,landingX=r[0x699]+256*r[0x69a],landingDepth=r[0x698],selectedHoop=r[0x55d],activeDunker=r[0x6d1],auxiliaryMode=r[0x547],carriedPlayer=r[0x569],detachedCount=r[0xe4],durabilities=list(r[0x577:0x584]),ball=dict(x=wide(0x6e,0x7b,0x88),depth=word(0x95,0xa2),height=wide(0xaf,0xbc,0xc9),vx=word(0x47e,0x48b),vy=word(0x498,0x4a5),vz=word(0x4b2,0x4bf),gravity=word(0x4cc,0x4d9),action=r[0x518],request=r[0x525],pose=r[0x61],facing=r[0x471],clock=r[0x43d],cursor=r[0x44a],repeatCount=r[0x457],nextAction=r[0x532],air=r[0x5fd],attachment=r[0x648],bounces=r[0x511],deflection=r[0x564]))

def main():
 records=[];sources=[]
 references=['hoop-control-cpu-long']
 for tick in [25,37,49,61,73,-37,-49,-61,-73]:
  direction='right' if tick>0 else 'left'
  tick=abs(tick)
  spec=dict(name='special-run-release-'+direction+'-'+str(tick),load_slot=7,frames=180,segments=[dict(start=1,end=6,keys=[direction]),dict(start=13,end=tick+6,keys=[direction]),dict(start=tick,end=tick+6,keys=['B'])],captures=[180])
  path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n')
  import sys
  if '--extract' not in sys.argv:run(path)
  references.append(spec['name'])
 for tick,duration in [(10,6),(16,6),(22,6),(28,6),(34,6),(10,9),(10,12),(10,15),(10,18)]:
  spec=dict(name='special-air-release-'+str(tick)+'-'+str(duration),load_slot=7,frames=180,segments=[dict(start=1,end=3,keys=['A','B']),dict(start=tick,end=tick+duration,keys=['B'])],captures=[180])
  path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n')
  if '--extract' not in sys.argv:run(path)
  references.append(spec['name'])
 for name in references:
  reference=BASE/'runs'/name;spec=json.loads((reference/'scenario.json').read_text());spec.update(name='special-shot-'+name,hooks_lua=HOOK)
  path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n')
  if '--extract' not in sys.argv:run(path)
  observed=BASE/'runs'/spec['name'];assert (observed/'ram.bin').read_bytes()==(reference/'ram.bin').read_bytes()
  raw=(observed/'special-shot-ram.bin').read_bytes();rows=list(csv.DictReader((observed/'special-shot.csv').open()));assert len(raw)==len(rows)*4096
  records.extend(dict(scenario=name,frame=int(v['frame']),actor=int(v['actor']),kind=v['kind'],before=decode(raw[j*4096:j*4096+2048],int(v['actor'])),after=decode(raw[j*4096+2048:(j+1)*4096],int(v['actor']))) for j,v in enumerate(rows))
  sources.append(dict(scenario=name,trace_sha256=hashlib.sha256(raw).hexdigest(),source=json.loads((observed/'source.json').read_text())))
 fixture=ROOT/'games/basketball/engine/testdata/special-shot.json';fixture.write_text(json.dumps(records,separators=(',',':'))+'\n')
 fixture.with_name('special-shot-source.json').write_text(json.dumps(dict(source_is_original_execution=True,records=len(records),sha256=hashlib.sha256(fixture.read_bytes()).hexdigest(),sources=sources),indent=2)+'\n');print('special-shot',len(records))

if __name__=='__main__': main()
