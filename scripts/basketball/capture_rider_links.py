"""Observe bank 5 $809A rider-link phase without changing original RAM/input."""
import argparse,hashlib,json
from capture_reference import ROOT,BASE,run
HOOK=r'''
local f=assert(io.open(out..'/rider-links.jsonl','w'));local pending=nil
local function word(a,b) return memory.readbyte(a)+256*memory.readbyte(b) end
local function wide(a,b,c) return word(a,b)+65536*memory.readbyte(c) end
local function state()
 local objects={};local links={}
 for i=0,5 do
  objects[#objects+1]=string.format('{"x":%d,"depth":%d,"height":%d,"vz":%d,"kind":%d,"action":%d,"air":%d,"control":%d,"ai":%d}',wide(0x68+i,0x75+i,0x82+i),word(0x8f+i,0x9c+i),wide(0xa9+i,0xb6+i,0xc3+i),word(0x4ac+i,0x4b9+i),memory.readbyte(0x3d6+i),memory.readbyte(0x512+i),memory.readbyte(0x5f7+i),memory.readbyte(0x539+i),memory.readbyte(0x672+i))
 end
 for i=0,6 do links[#links+1]=tostring(memory.readbyte(0x642+i)) end
 return '{"objects":['..table.concat(objects,',')..'],"links":['..table.concat(links,',')..']}'
end
memory.registerexec(0x809a,1,function()
 if memory.readbyte(0xbfff)~=5 then return end
 assert(not pending);pending=string.format('{"frame":%d,"before":%s,',emu.framecount(),state())
end)
memory.registerexec(0x801a,1,function()
 if memory.readbyte(0xbfff)~=5 then return end
 assert(pending);f:write(pending..'"after":'..state()..'}\n');pending=nil
end)
function finish_probe() assert(not pending);f:close() end
'''
def main():
 parser=argparse.ArgumentParser();parser.add_argument('--natural',action='store_true');args=parser.parse_args()
 if args.natural:
  spec=dict(name='rider-normal-reference',load_slot=5,frames=180,captures=[60,63,66,90,180],segments=[dict(start=1,end=24,keys=['down']),dict(start=25,end=30,keys=['A','B','right'])])
  initial=BASE/(spec['name']+'.json');initial.write_text(json.dumps(spec,indent=2)+'\n');run(initial)
  reference=BASE/'runs'/spec['name']
 else:
  reference=BASE/'runs/hoop-control-cpu-long';spec=json.loads((reference/'scenario.json').read_text())
 spec.pop('save_slot',None);spec.update(name='rider-links-natural' if args.natural else 'rider-links-cpu-long',hooks_lua=HOOK)
 path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');run(path)
 observed=BASE/'runs'/spec['name'];assert (observed/'ram.bin').read_bytes()==(reference/'ram.bin').read_bytes()
 for image in reference.glob('frame-*.png'):assert image.read_bytes()==(observed/image.name).read_bytes()
 rows=[json.loads(line) for line in (observed/'rider-links.jsonl').read_text().splitlines()]
 assert rows
 path=ROOT/'games/basketball/engine/testdata'/('rider-links-natural.json' if args.natural else 'rider-links.json');path.write_text(json.dumps(rows,separators=(',',':'))+'\n')
 path.with_name(path.stem+'-source.json').write_text(json.dumps(dict(source_is_original_execution=True,entry='bank5:809a',exit='bank5:801a',records=len(rows),changed=sum(r['before']!=r['after'] for r in rows),sha256=hashlib.sha256(path.read_bytes()).hexdigest(),source=json.loads((observed/'source.json').read_text())),indent=2)+'\n')
 print(json.dumps(dict(records=len(rows),changed=sum(r['before']!=r['after'] for r in rows))))
if __name__=='__main__':main()
