"""Verify global original audio requests across normal Start pause/resume."""
import hashlib,json
from capture_reference import ROOT,BASE,run
HOOK=r'''
local f=assert(io.open(out..'/sound-requests.jsonl','w'))
local updates=assert(io.open(out..'/audio-updates.jsonl','w'))
memory.registerexec(0xfbcc,1,function()
 f:write(string.format('{"frame":%d,"id":%d,"scene":%d,"phase":%d}\n',emu.framecount(),memory.getregister('a'),memory.readbyte(0x588),memory.readbyte(0x589)))
end)
memory.registerexec(0xfbe3,1,function()
 updates:write(string.format('{"frame":%d,"phase":%d}\n',emu.framecount(),memory.readbyte(0x589)))
end)
function finish_probe()f:close();updates:close()end
'''
def main():
 expected=dict(name='match-clock-pause',load_slot=5,frames=300,captures=[65,95,300],segments=[dict(start=60,end=66,keys=['start']),dict(start=90,end=96,keys=['start'])])
 ref=BASE/'runs/match-clock-pause'
 if not (ref/'scenario.json').exists():
  p=BASE/'match-clock-pause.json';p.write_text(json.dumps(expected,indent=2)+'\n');run(p)
 spec=json.loads((ref/'scenario.json').read_text());assert spec==expected
 spec.update(name='runtime-pause-audio',hooks_lua=HOOK);p=BASE/'runtime-pause-audio.json';p.write_text(json.dumps(spec,indent=2)+'\n');run(p)
 out=BASE/'runs/runtime-pause-audio';raw=(out/'ram.bin').read_bytes();assert raw==(ref/'ram.bin').read_bytes()
 for p in ref.glob('frame-*.png'):assert p.read_bytes()==(out/p.name).read_bytes()
 requests=[json.loads(line) for line in (out/'sound-requests.jsonl').read_text().splitlines()]
 updates=[json.loads(line) for line in (out/'audio-updates.jsonl').read_text().splitlines()]
 paused_updates=sum(row['phase']==2 for row in updates)
 paused=[i+1 for i in range(300) if raw[i*2048+0x589]==2]
 assert paused==list(range(62,92));assert paused_updates==30;assert all(row['id'] not in (253,255) for row in requests)
 result=dict(same_ram=True,same_screenshots=True,paused_input_frames=paused,paused_audio_driver_updates=paused_updates,audio_driver_updates=len(updates),requests=requests,source=json.loads((out/'source.json').read_text()),sound_trace_sha256=hashlib.sha256((out/'sound-requests.jsonl').read_bytes()).hexdigest())
 (ROOT/'.local/basketball/engine/runtime-pause-audio.json').write_text(json.dumps(result,indent=2)+'\n');print(json.dumps(result))
if __name__=='__main__':main()
