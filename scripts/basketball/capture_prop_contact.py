"""Capture actual player-to-prop responses with unmodified game inputs and RAM."""
import csv,hashlib,json,sys
from capture_reference import ROOT,BASE,run
from extract_world_actions import decode_world_action_state
HOOK=r'''
local raw=assert(io.open(out..'/prop-contact.bin','wb'));local csv=assert(io.open(out..'/prop-contact.csv','w'));csv:write('frame,source,target,sounds\n')
local pending=nil;local sounds={}
local function snapshot() local b={};for i=0,2047 do b[#b+1]=string.char(memory.readbyte(i)) end;return table.concat(b) end
memory.registerexec(0x8653,1,function()
 if memory.readbyte(0xbfff)~=5 then return end
 local source=memory.getregister('x');if source>=4 then return end
 local target=memory.readbyte(0x3c+source);if target<4 or target>=6 then return end
 assert(not pending);pending={emu.framecount(),source,target};sounds={};raw:write(snapshot())
end)
memory.registerexec(0xc000,1,function() if pending and memory.readbyte(0xbfff)==5 then sounds[#sounds+1]=memory.getregister('a') end end)
memory.registerexec(0x84be,1,function()
 if not pending or memory.readbyte(0xbfff)~=5 then return end
 raw:write(snapshot());csv:write(table.concat(pending,',')..','..table.concat(sounds,';')..'\n');pending=nil
end)
function finish_probe() assert(not pending);raw:close();csv:close() end
'''
def state(r):
 w=decode_world_action_state(r)
 return dict(players=[dict(player=a['player'],attackVariant=a['attackVariant'],attackGrade=r[0x625+i],endurance=a['endurance']) for i,a in enumerate(w['actors'])],auxiliary=w['auxiliary'],specialHolder=w['specialHolder'])
def main():
 records=[];sources=[]
 for tick in [198,207,216]:
  spec=dict(name='prop-approach-'+str(tick),load_slot=5,frames=420,segments=[dict(start=1,end=6,keys=['right']),dict(start=1,end=21,keys=['up']),dict(start=13,end=300,keys=['right']),dict(start=tick,end=tick+6,keys=['A'])],captures=[420])
  path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n')
  if '--extract' not in sys.argv:run(path)
  reference=BASE/'runs'/spec['name'];spec.update(name='prop-contact-'+str(tick),hooks_lua=HOOK);path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n')
  if '--extract' not in sys.argv:run(path)
  observed=BASE/'runs'/spec['name'];assert (observed/'ram.bin').read_bytes()==(reference/'ram.bin').read_bytes()
  raw=(observed/'prop-contact.bin').read_bytes();rows=list(csv.DictReader((observed/'prop-contact.csv').open()));assert len(raw)==len(rows)*4096
  for i,row in enumerate(rows):
   b=raw[i*4096:i*4096+2048];a=raw[i*4096+2048:(i+1)*4096]
   records.append(dict(frame=int(row['frame']),source=int(row['source']),target=int(row['target']),before=state(b),after=state(a),sounds=[int(v) for v in row['sounds'].split(';') if v]))
  sources.append(dict(scenario=spec['name'],records=len(rows),trace_sha256=hashlib.sha256(raw).hexdigest(),source=json.loads((observed/'source.json').read_text())))
 p=ROOT/'games/basketball/engine/testdata/prop-contact-natural.json';p.write_text(json.dumps(records,separators=(',',':'))+'\n');p.with_name('prop-contact-natural-source.json').write_text(json.dumps(dict(source_is_original_execution=True,records=len(records),entry='bank5:8653',exit='bank5:84be',scope='all source players0..3 to props4/5',sha256=hashlib.sha256(p.read_bytes()).hexdigest(),sources=sources),indent=2)+'\n');print('natural prop contacts',len(records))
if __name__=='__main__':main()
