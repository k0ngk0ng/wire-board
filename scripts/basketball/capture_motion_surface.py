"""Read-only original action-phase physical-entry and surface-write evidence."""
import csv
import gzip
import hashlib
import json
from capture_reference import ROOT, BASE, run

HOOK=r'''
local phases=assert(io.open(out..'/world-actions.csv','w'));phases:write('frame\n')
local raw=assert(io.open(out..'/world-actions-ram.bin','wb'))
local events=assert(io.open(out..'/motion-surfaces.csv','w'));events:write('phase,frame,object,x,depth,height,surface\n')
local extra=assert(io.open(out..'/extra-surface-writes.csv','w'));extra:write('phase,pc,address,value,pendingObject\n')
local phase=0;local active=false;local frame=0;local pending=nil
local function snapshot() local t={};for i=0,2047 do t[#t+1]=string.char(memory.readbyte(i)) end;raw:write(table.concat(t)) end
local function word(lo,hi,x) return memory.readbyte(lo+x)+256*memory.readbyte(hi+x) end
local function wide(lo,mid,hi,x) local v=word(lo,mid,x)+65536*memory.readbyte(hi+x);if v>=8388608 then v=v-16777216 end;return v end
memory.registerexec(0xd345,1,function() assert(not active);active=true;frame=emu.framecount();snapshot() end)
memory.registerexec(0xd348,1,function() assert(active);if pending then extra:write('unwritten-end,'..table.concat(pending,',')..'\n');extra:flush();pending=nil end;snapshot();phases:write(frame..'\n');active=false;phase=phase+1 end)
memory.registerexec(0xef5f,1,function()
 if not active then return end
 if pending then extra:write('unwritten-next,'..table.concat(pending,',')..'\n');extra:flush() end;local x=memory.getregister('x')
 pending={phase,frame,x,wide(0x68,0x75,0x82,x),word(0x8f,0x9c,x),wide(0xa9,0xb6,0xc3,x)}
end)
memory.registerwrite(0x5ea,13,function(address,size,value)
 if not active then return end
 if not pending or address~=0x5ea+pending[3] then extra:write(string.format('%d,%04x,%04x,%d,%d\n',phase,memory.getregister('pc'),address,value,pending and pending[3] or -1));extra:flush();return end
 pending[#pending+1]=value
 events:write(table.concat(pending,',')..'\n');pending=nil
end)
function finish_probe() phases:close();raw:close();events:close();extra:close() end
'''


def main():
    output=ROOT/'games/basketball/engine/testdata'
    for reference_name,fixture in [('hoop-control-cpu-long','motion-surface'),('rider-normal-reference','motion-surface-rider')]:
        reference=BASE/'runs'/reference_name
        spec=json.loads((reference/'scenario.json').read_text());spec.pop('save_slot',None)
        spec.update(name=fixture+'-capture',hooks_lua=HOOK)
        path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');run(path)
        observed=BASE/'runs'/spec['name']
        assert (observed/'ram.bin').read_bytes()==(reference/'ram.bin').read_bytes()
        for image in reference.glob('frame-*.png'):assert image.read_bytes()==(observed/image.name).read_bytes()
        phases=list(csv.DictReader((observed/'world-actions.csv').open()))
        records=[dict(frame=int(p['frame']),writes=[],surfaces=[]) for p in phases]
        tail=[]
        for row in csv.DictReader((observed/'motion-surfaces.csv').open()):
            i=int(row['phase'])
            if i>=len(records):tail.append(row);continue
            assert int(row['frame'])==records[i]['frame']
            records[i]['writes'].append(dict(object=int(row['object']),origin={k:int(row[k]) for k in ['x','depth','height']}))
            records[i]['surfaces'].append(int(row['surface']))
        extras=(observed/'extra-surface-writes.csv').read_bytes()
        skipped=extras.decode().splitlines()[1:]
        assert all(row.startswith(('unwritten-next,','unwritten-end,')) for row in skipped), 'surface write outside matching physical entry'
        raw=(observed/'world-actions-ram.bin').read_bytes()
        assert len(records)*4096<=len(raw)<=len(records)*4096+2048
        payload=(json.dumps(records,separators=(',',':'))+'\n').encode();packed=gzip.compress(payload,mtime=0)
        (output/(fixture+'.json.gz')).write_bytes(packed)
        manifest=dict(physical_entries_without_surface_write=len(skipped),skipped_entries_sha256=hashlib.sha256(extras).hexdigest(),source_is_original_execution=True,records=len(records),motion_events=sum(len(r['writes']) for r in records),tail_events_preserved=tail,sha256=hashlib.sha256(packed).hexdigest(),uncompressed_sha256=hashlib.sha256(payload).hexdigest(),complete_action_trace_sha256=hashlib.sha256(raw[:len(records)*4096]).hexdigest(),full_action_trace_sha256=hashlib.sha256(raw).hexdigest(),reference_ram_sha256=hashlib.sha256((reference/'ram.bin').read_bytes()).hexdigest(),source=json.loads((observed/'source.json').read_text()))
        (output/(fixture+'-source.json')).write_text(json.dumps(manifest,indent=2)+'\n')
        print(fixture,len(records),manifest['motion_events'])


if __name__=='__main__':main()
