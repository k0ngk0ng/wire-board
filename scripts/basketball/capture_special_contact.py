"""Observe the original post-combat ball contact tail, without changing RAM."""
import csv,gzip,hashlib,json,sys
from capture_reference import ROOT,BASE,run
from extract_world_actions import decode_world_action_state
HOOK=r'''
local csv=assert(io.open(out..'/special-contact.csv','w'));csv:write('frame,sounds\n')
local raw=assert(io.open(out..'/special-contact.bin','wb'));local pending=nil;local sounds={}
local function snapshot() local b={};for i=0,2047 do b[#b+1]=string.char(memory.readbyte(i)) end;return table.concat(b) end
memory.registerexec(0x84c3,1,function()
 if memory.readbyte(0xbfff)~=5 then return end
 local candidate=memory.readbyte(0x42);local action=AND(memory.readbyte(0x518),127)
 if candidate>=4 or action<13 then return end
 assert(not pending);pending=emu.framecount();sounds={};raw:write(snapshot())
end)
memory.registerexec(0xc000,1,function()
 if pending and memory.readbyte(0xbfff)==5 then sounds[#sounds+1]=memory.getregister('a') end
end)
memory.registerexec(0x801d,1,function()
 if not pending or memory.readbyte(0xbfff)~=5 then return end
 raw:write(snapshot());csv:write(tostring(pending)..','..table.concat(sounds,';')..'\n');pending=nil
end)
function finish_probe() assert(not pending);csv:close();raw:close() end
'''
def contact_state(r):
 w=decode_world_action_state(r)
 players=[]
 for i,p in enumerate(w['actors']):players.append(dict(player=p['player'],attackVariant=p['attackVariant'],attackGrade=r[0x625+i],endurance=p['endurance']))
 return dict(players=players,ball=w['ball'],holder=w['holder'],lastTouch=w['lastTouch'],selectedHoop=w['selectedHoop'])
def contact_context(r):
 signed=lambda v:v-256 if v>=128 else v
 return dict(courtID=r[0x58b],candidates=[signed(v) for v in r[0x3c:0x43]],controls=list(r[0x539:0x53d]),attachments=list(r[0x642:0x646]),excludedCatcher=signed(r[0x561]),previousHolder=signed(r[0x546]),auxiliaryOwners=[signed(v) for v in r[0x53d:0x53f]],hoopOwners=[signed(v) for v in r[0x540:0x546]])
def main():
 references=['special-ball-cpu-command']
 references += ['special-run-release-left-'+str(x) for x in [37,49,61,73]]
 references += ['special-air-release-10-'+str(x) for x in [9,12,15,18]]
 records=[];sources=[]
 for index,name in enumerate(references):
  reference=BASE/'runs'/name;spec=json.loads((reference/'scenario.json').read_text());spec.update(name='special-contact-'+str(index),hooks_lua=HOOK);path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n')
  if '--extract' not in sys.argv:run(path)
  observed=BASE/'runs'/spec['name'];assert (observed/'ram.bin').read_bytes()==(reference/'ram.bin').read_bytes()
  rows=list(csv.DictReader((observed/'special-contact.csv').open()));raw=(observed/'special-contact.bin').read_bytes();assert len(raw)==len(rows)*4096
  for i,row in enumerate(rows):
   b=raw[i*4096:i*4096+2048];a=raw[i*4096+2048:(i+1)*4096]
   records.append(dict(scenario=name,frame=int(row['frame']),before=contact_state(b),after=contact_state(a),context=contact_context(b),sounds=[int(v) for v in row['sounds'].split(';') if v]))
  sources.append(dict(scenario=name,records=len(rows),trace_sha256=hashlib.sha256(raw).hexdigest(),source=json.loads((observed/'source.json').read_text())))
 p=ROOT/'games/basketball/engine/testdata/special-contact-natural.json';p.write_text(json.dumps(records,separators=(',',':'))+'\n');p.with_name('special-contact-natural-source.json').write_text(json.dumps(dict(source_is_original_execution=True,scope='all candidate players0..3 and ball actions13+ at 84C3..801D',records=len(records),sha256=hashlib.sha256(p.read_bytes()).hexdigest(),sources=sources),indent=2)+'\n');print('special contacts',len(records))
if __name__=='__main__':main()
