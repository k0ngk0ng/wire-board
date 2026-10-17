"""One declared Court7 world origin, then unchanged original full-machine play."""
import hashlib,json
from capture_reference import ROOT,BASE,run
from capture_rest_stall_original import CONFIG
from encode_world_probe import encode_world_probe
from extract_world_loop import loop_state
from compare_rest_stall_worlds import normalized,differences

def main():
 out=ROOT/'.local/basketball/rea/court-seven-stall';origin=json.loads((out/'controlled-origin.json').read_text());patch,names=encode_world_probe(origin)
 config=CONFIG.replace('memory.writebyte(0x587,5)','memory.writebyte(0x587,1)').replace('"teams":[0,5]','"teams":[0,1]').replace('memory.writebyte(0x58b,1)','memory.writebyte(0x58b,7)').replace('"court":1','"court":7')
 writes=';'.join(f'memory.writebyte({a},{v})' for a,v in sorted(patch.items()))
 hook=r'''
local aligned=false;local count=0
local trace=assert(io.open(out..'/world-after.ram','wb'));local index=assert(io.open(out..'/world-after.jsonl','w'))
local function raw()local t={};for i=0,2047 do t[#t+1]=string.char(memory.readbyte(i))end;return table.concat(t)end
memory.registerexec(0xd355,1,function()
 if not aligned and court and memory.readbyte(0x589)==129 and memory.readbyte(0x58b)==7 then
  writefile('alignment-before.ram',raw());PATCH
  aligned=true;writefile('alignment-after.ram',raw());writefile('alignment-frame.json',string.format('{"frame":%d}',emu.framecount()))
 end
 if aligned and OBSERVE then trace:write(raw());index:write(string.format('{"frame":%d,"index":%d}\n',emu.framecount(),count));count=count+1 end
end)
function finish_probe()assert(aligned);trace:close();index:close()end
'''.replace('PATCH',writes)
 paths=[]
 for label,observe in [('reference','false'),('observed','true')]:
  spec=json.loads((BASE/'opening-from-menu.json').read_text());spec.pop('save_slot',None);spec.update(name='court7-rest-stall-'+label,frames=13000,captures=[1500,3000,8000,12000,13000],hooks_lua=config+'\n'+hook.replace('OBSERVE',observe))
  path=out/(label+'-spec.json');path.write_text(json.dumps(spec,indent=2)+'\n');run(path);paths.append(BASE/'runs'/spec['name'])
 reference,observed=paths;assert(reference/'ram.bin').read_bytes()==(observed/'ram.bin').read_bytes()
 for n in spec['captures']:assert(reference/f'frame-{n:05d}.png').read_bytes()==(observed/f'frame-{n:05d}.png').read_bytes()
 first=(observed/'alignment-after.ram').read_bytes();initial=loop_state(first,origin['playing']['world']['Updates']);ds=list(differences(normalized(initial),normalized(origin['playing']['world'])));assert not ds,ds
 raw=(observed/'world-after.ram').read_bytes();rows=[json.loads(x) for x in (observed/'world-after.jsonl').read_text().splitlines()];assert len(raw)==2048*len(rows);stalls=[]
 for i,row in enumerate(rows):
  r=raw[i*2048:(i+1)*2048]
  if r[0x589]&127==1 and r[0x53f]==6 and r[0x518]&127==0 and r[0x5fd]&128 and (r[0xbc] or r[0xc9]) and not any(r[a] for a in [0x47e,0x48b,0x498,0x4a5,0x4b2,0x4bf,0x4cc,0x4d9]):stalls.append(dict(**row,x=r[0x6e]+256*r[0x7b]+65536*r[0x88],depth=r[0x95]+256*r[0xa2],height=r[0xaf]+256*r[0xbc]+65536*r[0xc9]))
 r=raw[-2048:];result=dict(scope='one controlled canonical world origin, no later gameplay writes; not natural menu coverage',originGoVideo=origin['videoFrames'],initialSemanticRoundTrip=True,referenceRAMAndScreenshotsMatch=True,worlds=len(rows),stalledWorlds=len(stalls),firstStalls=stalls[:5],lastStalls=stalls[-5:],lastPhase=r[0x589],lastClock=list(r[0x6c2:0x6c6]),lastScores=[r[0x166]+256*r[0x168],r[0x167]+256*r[0x169]],source=json.loads((observed/'source.json').read_text()),traceSHA256=hashlib.sha256(raw).hexdigest(),ramSHA256=hashlib.sha256((observed/'ram.bin').read_bytes()).hexdigest())
 (out/'original-world-result.json').write_text(json.dumps(result,indent=2)+'\n');print(json.dumps(result))
if __name__=='__main__':main()
