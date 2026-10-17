"""Retain original B100 search results and actual lazy-read branches, without writes."""
import csv,gzip,hashlib,json
from capture_reference import ROOT,BASE,run
from extract_world_actions import decode_world_action_state

HOOK=r'''
local raw=assert(io.open(out..'/pursuit-ram.bin','wb'))
local records=assert(io.open(out..'/pursuit.jsonl','w'))
local writes=assert(io.open(out..'/reserved-writes.csv','w'));writes:write('frame,pc,address,value\n')
local pending=nil
local function snapshot() local t={};for i=0,2047 do t[#t+1]=string.char(memory.readbyte(i)) end;raw:write(table.concat(t)) end
memory.registerexec(0xb100,1,function()
 if memory.readbyte(0xbfff)~=5 then return end
 assert(not pending);pending={frame=emu.framecount(),actor=memory.getregister('x'),probes={}};snapshot()
end)
memory.registerexec(0xb10a,1,function()
 if not pending or memory.readbyte(0xbfff)~=5 then return end
 local index=memory.getregister('y')
 pending.probes[#pending.probes+1]={index=index,action=AND(memory.readbyte(0x512+index),127),depth=memory.readbyte(0x9c+index),x=memory.readbyte(0x75+index)+256*memory.readbyte(0x82+index),readDepth=false,readX=false}
end)
memory.registerexec(0xb11b,1,function() if pending and memory.readbyte(0xbfff)==5 then pending.probes[#pending.probes].readDepth=true end end)
memory.registerexec(0xb12c,1,function() if pending and memory.readbyte(0xbfff)==5 then pending.probes[#pending.probes].readX=true end end)
local function finish(target)
 if not pending or memory.readbyte(0xbfff)~=5 then return end
 local probes={}
 for _,p in ipairs(pending.probes) do probes[#probes+1]=string.format('{"index":%d,"action":%d,"depth":%d,"x":%d,"readDepth":%s,"readX":%s}',p.index,p.action,p.depth,p.x,tostring(p.readDepth),tostring(p.readX)) end
 records:write(string.format('{"frame":%d,"actor":%d,"target":%d,"probes":[%s]}\n',pending.frame,pending.actor,target,table.concat(probes,',')));pending=nil
end
memory.registerexec(0xb145,1,function() finish(-1) end)
memory.registerexec(0xb15a,1,function() finish(memory.getregister('y')) end)
for _,address in ipairs({0x55b,0xe5,0xe7}) do
 memory.registerwrite(address,1,function(a,size,value) writes:write(string.format('%d,%04x,%04x,%d\n',emu.framecount(),memory.getregister('pc'),a,value or -1)) end)
end
function finish_probe()
 if pending then writefile('incomplete-pursuit.json','{"frame":'..pending.frame..',"actor":'..pending.actor..'}') end
 raw:close();records:close();writes:close()
end
'''


def main():
 all_records=[];sources=[]
 for name in ['hoop-control-cpu-long','special-ball-cpu-command','rider-normal-reference']:
  reference=BASE/'runs'/name
  spec=json.loads((reference/'scenario.json').read_text());spec.pop('save_slot',None);spec.update(name='pursuit-search-'+name,hooks_lua=HOOK)
  path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');run(path)
  observed=BASE/'runs'/spec['name'];assert (observed/'ram.bin').read_bytes()==(reference/'ram.bin').read_bytes()
  for image in reference.glob('frame-*.png'):assert image.read_bytes()==(observed/image.name).read_bytes()
  rows=[json.loads(s) for s in (observed/'pursuit.jsonl').read_text().splitlines()];raw=(observed/'pursuit-ram.bin').read_bytes();assert len(rows)*2048<=len(raw)<=len(rows)*2048+2048
  for i,row in enumerate(rows):
   before=raw[i*2048:(i+1)*2048]
   all_records.append(dict(row,scenario=name,before=decode_world_action_state(before),controls=list(before[0x539:0x53d]),reserved=dict(action73=before[0x55b],depth73=before[0xe5],depth75=before[0xe7]),alias74=dict(actionByte=before[0x55c],depthByte=before[0xe6])))
  sources.append(dict(name=name,records=len(rows),writes=list(csv.DictReader((observed/'reserved-writes.csv').open())),trace_sha256=hashlib.sha256(raw).hexdigest(),source=json.loads((observed/'source.json').read_text())))
 payload=(json.dumps(all_records,separators=(',',':'))+'\n').encode();packed=gzip.compress(payload,mtime=0)
 out=ROOT/'games/basketball/engine/testdata';(out/'pursuit-search.json.gz').write_bytes(packed)
 (out/'pursuit-search-source.json').write_text(json.dumps(dict(source_is_original_execution=True,records=len(all_records),sha256=hashlib.sha256(packed).hexdigest(),uncompressed_sha256=hashlib.sha256(payload).hexdigest(),sources=sources),indent=2)+'\n')
 print(json.dumps(dict(records=len(all_records),indices=sorted({p['index'] for r in all_records for p in r['probes']}),reserved_writes=[len(s['writes']) for s in sources])))


if __name__=='__main__':main()
