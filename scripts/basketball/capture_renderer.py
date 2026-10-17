"""Observe semantic sprite draw inputs and committed court columns.
Original RAM is read only; no visible emulator or replacement gameplay is run.
"""
import hashlib,json
from pathlib import Path
from capture_reference import ROOT,BASE,run
EXTRA=r'''
local draw_inputs='{}';local rendered_inputs='{}';local rendered_ball_chr=memory.readbyte(0x54)
local commit_history={};local prepared_column=memory.readbyte(0x3a1);local prepared_phase=AND(memory.readbyte(0x589),127)
local field_page=1
local function s16(lo,hi) local n=memory.readbyte(lo)+256*memory.readbyte(hi);if n>=32768 then n=n-65536 end;return n end
local function u24(a,b,c) return memory.readbyte(a)+256*memory.readbyte(b)+65536*memory.readbyte(c) end
local function semantic_inputs()
 local objects={};local order={}
 for i=0,12 do
  local height=u24(0xa9+i,0xb6+i,0xc3+i);if height>=8388608 then height=height-16777216 end
  local pose=memory.readbyte(0x5b+i)
  objects[#objects+1]=string.format('{"id":%d,"kind":%d,"character":%d,"pose":%d,"projected":{"x":%d,"y":%d},"position":{"x":%d,"depth":%d,"height":%d},"airborne":%s,"shadowSuppressed":%s,"action":%d}',i,memory.readbyte(0x3d6+i),i<4 and memory.readbyte(0x3e3+i) or 0,pose,s16(0x3e7+i,0x3f4+i),s16(0x401+i,0x40e+i),u24(0x68+i,0x75+i,0x82+i),memory.readbyte(0x8f+i)+256*memory.readbyte(0x9c+i),height,tostring(AND(memory.readbyte(0x5f7+i),128)~=0),tostring(memory.readbyte(0x5ea+i)~=0),AND(memory.readbyte(0x512+i),127))
 end
 for i=0,11 do local v=memory.readbyte(0x41d+i);if v<13 then order[#order+1]=tostring(v) end end
 return string.format('{"objects":[%s],"drawOrder":[%s],"teams":[%d,%d],"ballSource":%d,"phase":%d}',table.concat(objects,','),table.concat(order,','),memory.readbyte(0x586),memory.readbyte(0x587),memory.readbyte(0x6d1),AND(memory.readbyte(0x589),127))
end
draw_inputs=semantic_inputs();rendered_inputs=draw_inputs
memory.registerexec(0xbc74,1,function()
 if memory.readbyte(0xbfff)==1 then prepared_column=memory.readbyte(0x3a1);prepared_phase=AND(memory.readbyte(0x589),127) end
end)
memory.registerexec(0xcb16,1,function()
 commit_history[#commit_history+1]=string.format('{"column":%d,"phase":%d}',prepared_column,prepared_phase)
end)
'''

def main():
 evidence=json.loads((ROOT/'.local/basketball/graphics/verification.json').read_text());sources=[]
 for name in sorted({r['scenario'] for r in evidence['frames_detail']}):
  reference=BASE/'runs'/('graphics-'+name);spec=json.loads((reference/'scenario.json').read_text());hook=spec['hooks_lua']
  hook=hook.replace('local rendered_order=false','local rendered_order=false\n'+EXTRA)
  hook=hook.replace('function() order_reversed=false end)','function() order_reversed=false;draw_inputs=semantic_inputs() end)')
  hook=hook.replace('rendered_order=order_reversed','rendered_order=order_reversed;rendered_inputs=draw_inputs;rendered_ball_chr=memory.readbyte(0x54)')
  hook=hook.replace('fieldx,fieldy=scrollx,scrolly','fieldx,fieldy=scrollx,scrolly;field_page=memory.readbyte(0x39a)')
  hook=hook.replace('writefile(stem.."-oam.bin",oam)','writefile(stem.."-oam.bin",oam)\n writefile(stem.."-semantic.json",string.format(\'{"state":%s,"courtCommits":[%s],"camera":{"x":%d,"y":%d},"reverseSprites":%s,"ballCHR":%d}\',rendered_inputs,table.concat(commit_history,","),fieldx,fieldy+256*field_page,tostring(rendered_order),rendered_ball_chr))\n commit_history={}')
  spec.update(name='renderer-'+name,hooks_lua=hook)
  path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');run(path)
  observed=BASE/'runs'/spec['name'];assert (observed/'ram.bin').read_bytes()==(reference/'ram.bin').read_bytes()
  for image in reference.glob('frame-*.png'):assert image.read_bytes()==(observed/image.name).read_bytes()
  sources.append(dict(scenario=name,ram_matches=True,source=json.loads((observed/'source.json').read_text())))
 out=ROOT/'.local/basketball/renderer';out.mkdir(parents=True,exist_ok=True);(out/'capture.json').write_text(json.dumps({'scope':'renderer input observation, not Go world execution','sources':sources},indent=2)+'\n')
if __name__=='__main__':main()
