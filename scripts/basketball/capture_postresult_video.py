"""Controlled original whole-console post-result observations, not earned wins.

Failure/source inventory precedes this implementation in
POSTRESULT_ART_VERIFICATION.md. Only first BA40 entry changes declared bytes;
all original clocks, text, actors, PPU/NMI and inputs then run normally.
"""
import argparse, hashlib, json
from pathlib import Path
from capture_reference import ROOT,BASE,run

OUT=ROOT/'.local/postresult-video'

def injection(kind):
    changes={0x166:20,0x167:10,0x168:0,0x169:0}
    if kind=='ending':changes.update({0x58c:28,0x6d2:127,0x6d3:127})
    values=','.join(f'{{{a},{v}}}' for a,v in changes.items())
    return r'''
local injected=false
local function ram_snapshot()return memory.readbyterange(0,2048)end
assert(type(ram_snapshot())=='string' and #ram_snapshot()==2048)
memory.registerexec(0xba40,1,function()
 if injected or memory.readbyte(0xbfff)~=4 then return end
 assert(memory.readbyte(0x588)==1 and memory.readbyte(0x589)==6)
 injected=true;writefile('injection-before.ram',ram_snapshot())
 local changes={CHANGES};local rows={}
 for _,c in ipairs(changes) do
  rows[#rows+1]=string.format('{"address":%d,"before":%d,"after":%d}',c[1],memory.readbyte(c[1]),c[2])
  memory.writebyte(c[1],c[2])
 end
 writefile('injection-after.ram',ram_snapshot())
 writefile('injection.json',string.format('{"pc":%d,"bank":4,"frame":%d,"cycles":%d,"changes":[%s],"natural_earned_victory":false}',memory.getregister('pc'),emu.framecount(),debugger.getcyclescount(),table.concat(rows,',')))
end)
'''.replace('CHANGES',values)

OBSERVE=r'''
local log=assert(io.open(out..'/events.jsonl','w'))
local raw=assert(io.open(out..'/boundaries.ram','wb'));local ordinal=0
local function row(kind,pc,addr,value,withraw)
 local index=-1
 if withraw then index=ordinal;raw:write(ram_snapshot());ordinal=ordinal+1 end
 log:write(string.format('{"kind":"%s","pc":%d,"frame":%d,"cycles":%d,"scene":%d,"phase":%d,"videoCounter":%d,"fade":[%d,%d],"message":%d,"timer":%d,"rewardTimer":%d,"address":%d,"value":%d,"raw":%d}\n',kind,pc or memory.getregister('pc'),emu.framecount(),debugger.getcyclescount(),memory.readbyte(0x588),memory.readbyte(0x589),memory.readbyte(0x4c),memory.readbyte(0x301),memory.readbyte(0x302),memory.readbyte(0x6f7),memory.readbyte(0x6d7)+256*memory.readbyte(0x6d8),memory.readbyte(0x3c3),addr or 0,value or 0,index))
end
local seenWait=false
local probes={
 [0xca54]='schedule',[0xcaa4]='scene1',[0xcae3]='scene7',[0xcae9]='scheduleWait',
 [0xc021]='fadeBegin',[0xfcf8]='fadeEnd',[0xc024]='backgroundBegin',[0xdcc2]='backgroundEnd',
 [0xc036]='messageTrampoline',[0xbe6e]='messageBegin',[0xbec2]='messageEnd',
 [0xfd0a]='textBegin',[0xfd16]='textEnd',
 [0xb995]='endingBegin',[0xbf35]='endingActorsBegin',[0xbf3f]='endingEnd',
 [0xbc96]='resultActorsBegin',[0xbca0]='resultEnd',[0xbd8e]='rewardBegin',[0xbe6d]='rewardEnd',
 [0xf79d]='reset'
}
local full={textBegin=true,textEnd=true,messageBegin=true,messageEnd=true,endingBegin=true,endingEnd=true,resultActorsBegin=true,resultEnd=true,rewardBegin=true,rewardEnd=true}
local bank4={messageBegin=true,messageEnd=true,resultActorsBegin=true,resultEnd=true,rewardBegin=true,rewardEnd=true}
local bank3={endingBegin=true,endingActorsBegin=true,endingEnd=true}
for pc,kind in pairs(probes) do
 memory.registerexec(pc,1,function()
  local bank=memory.readbyte(0xbfff)
  if bank4[kind] and bank~=4 or bank3[kind] and bank~=3 then return end
  if kind=='schedule' then seenWait=false end
  if kind=='scheduleWait' then if seenWait then return end;seenWait=true end
  row(kind,pc,nil,nil,full[kind])
 end)
end
memory.registerexec(0xfbcc,1,function()row('sound',0xfbcc,nil,memory.getregister('a'),false)end)
for _,addr in ipairs({0x30b,0x32e,0x351,0x374})do
 memory.registerwrite(addr,1,function(a,s,v)row('queueHeader',nil,a,v,false)end)
end
local snapshot_frames=SNAPSHOT_FRAMES
local irq_frames=IRQ_FRAMES
local oam=string.rep(string.char(255),256);local ctrl=memory.readbyte(0x56)
local function ppubytes(a,n)local parts={};for p=a,a+n-1 do parts[#parts+1]=string.char(ppu.readbyte(p))end;return table.concat(parts)end
memory.registerwrite(0x4014,1,function(a,s,v)oam=memory.readbyterange(v*256,256)end)
for _,addr in ipairs({0x2000,0x2001,0x2005,0x2006,0x8000,0x8001,0xa000,0xc000,0xc001,0xe000,0xe001})do
 memory.registerwrite(addr,1,function(a,s,v)
  if a==0x2000 then ctrl=v end
  if irq_frames[emu.framecount()] then row('deviceWrite',nil,a,v,false)end
 end)
end
local chr_snapshots=0
for _,irqpc in ipairs({0xf2d4,0xf277}) do memory.registerexec(irqpc,1,function()
 if irq_frames[emu.framecount()] then
  row(irqpc==0xf2d4 and 'irqBefore' or 'interruptReturn',irqpc,nil,nil,false)
  local stem=string.format('irq-%05d-%03d',emu.framecount(),chr_snapshots);chr_snapshots=chr_snapshots+1
  writefile(stem..'-ppu.bin',ppubytes(0,0x4000))
 end
end) end
emu.registerbefore(function()if irq_frames[emu.framecount()] then row('beforeFrame')end end)
emu.registerafter(function()
 local frame=emu.framecount()
 if irq_frames[frame] then row('afterFrame')end
 if snapshot_frames[frame] then
  local stem=string.format('ppu-%05d',frame)
  writefile(stem..'-ppu.bin',ppubytes(0,0x4000));writefile(stem..'-oam.bin',oam);writefile(stem..'-ctrl.bin',string.char(ctrl))
 end
end)
function finish_probe()log:close();raw:close();writefile('observation-complete.json',string.format('{"boundaries":%d,"irq_snapshots":%d}',ordinal,chr_snapshots))end
'''

def main():
    p=argparse.ArgumentParser();p.add_argument('kind',choices=['seed','reward','ending']);p.add_argument('--observe',action='store_true');p.add_argument('--mirroring',action='store_true');p.add_argument('--raster-states',action='store_true');p.add_argument('--start-frame',type=int);p.add_argument('--frames',type=int);p.add_argument('--repeat',action='store_true');args=p.parse_args()
    if sum([args.observe,args.mirroring,args.raster_states])>1:p.error('Choose one observation mode')
    OUT.mkdir(exist_ok=True)
    if args.kind=='seed':
        spec=json.loads((BASE/'runs/match-clock-complete/scenario.json').read_text())
        spec.update(name='postresult-natural-seed',frames=22050,captures=[22050],save_slot=91)
        path=OUT/'seed-scenario.json';path.write_text(json.dumps(spec,indent=2)+'\n');run(path)
        got=(BASE/'runs/postresult-natural-seed/ram.bin').read_bytes()
        assert got==(BASE/'runs/match-clock-complete/ram.bin').read_bytes()[:len(got)]
        print(json.dumps(dict(natural_replay_prefix_equal=True,saved_state_sha256=hashlib.sha256((BASE/'states/slot-91.fcs').read_bytes()).hexdigest())))
        return
    frames=args.frames or (1100 if args.kind=='reward' else 15000)
    captures=sorted(set([1,30,60,120,240,420,480,540,600,660,720,900,frames]+([1200,1800,3000,4800,6000,9000,10500,12000,13500,14000,14500] if args.kind=='ending' else [])))
    captures=[f for f in captures if f<=frames]
    name='postresult-'+args.kind+('-raster-states' if args.raster_states else '-mirroring' if args.mirroring else '-observed' if args.observe else '-reference')+('-repeat' if args.repeat else '')
    hook=injection(args.kind)
    if args.mirroring:
        hook+='''
local mirror=assert(io.open(out..'/mirroring.jsonl','w'));local previous=nil
memory.registerwrite(0xa000,1,function(a,s,v)
 if v==previous then return end;previous=v
 mirror:write(string.format('{"frame":%d,"cycles":%d,"pc":%d,"value":%d,"scene":%d,"phase":%d}\\n',emu.framecount(),debugger.getcyclescount(),memory.getregister('pc'),v,memory.readbyte(0x588),memory.readbyte(0x589)))
end)
function finish_probe()mirror:close()end
'''
    if args.observe or args.raster_states:
        import csv
        reference=BASE/f'runs/postresult-{args.kind}-reference'
        with (reference/'frames.csv').open() as f:mapping={int(r['frame']):int(r['emulator_frame']) for r in csv.DictReader(f)}
        selected={mapping[n] for n in captures};windows={f+d for f in selected for d in [-1,0,1]}
        table=lambda values:'{'+','.join(f'[{n}]=true' for n in sorted(values))+'}'
        if args.observe:hook+='\n'+OBSERVE.replace('SNAPSHOT_FRAMES',table(selected)).replace('IRQ_FRAMES',table(windows))
        if args.raster_states:
            hook+='\nlocal raster_selected='+table(selected)+'''
local clocks=assert(io.open(out..'/raster-clocks.jsonl','w'))
emu.registerbefore(function()
 local frame=emu.framecount()
 if not raster_selected[frame] then return end
 local path=out..string.format('/before-%05d.fcs',frame)
 local state=savestate.create(path);savestate.save(state);savestate.persist(state)
 clocks:write(string.format('{"frame":%d,"cycles":%d}\\n',frame,debugger.getcyclescount()))
end)
function finish_probe()clocks:close()end
'''
    spec=dict(name=name,load_slot=91,frames=frames,captures=captures,segments=[] if args.start_frame is None else [dict(start=args.start_frame,end=args.start_frame+1,keys=['start'])],hooks_lua=hook)
    path=OUT/(name+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');run(path)
    target=BASE/'runs'/name
    injection_data=json.loads((target/'injection.json').read_text());assert injection_data['pc']==0xba40
    before=(target/'injection-before.ram').read_bytes();after=(target/'injection-after.ram').read_bytes();changes={c['address']:c['after'] for c in injection_data['changes']}
    assert len(before)==len(after)==2048
    assert all(after[i]==changes.get(i,before[i]) for i in range(2048))
    if args.observe or args.repeat or args.mirroring or args.raster_states:
        reference=BASE/f'runs/postresult-{args.kind}-reference'
        for file in ['ram.bin','injection.json','injection-before.ram','injection-after.ram']+[f'frame-{n:05d}.png' for n in captures]:assert (target/file).read_bytes()==(reference/file).read_bytes(),file
    raw=(target/'ram.bin').read_bytes();transitions=[];prior=None
    for i in range(frames):
        ram=raw[i*2048:(i+1)*2048];state=(ram[0x588],ram[0x589]&127)
        if state!=prior:transitions.append(dict(frame=i+1,scene=state[0],phase=state[1],entered=bool(ram[0x589]&128)));prior=state
    report=dict(kind=args.kind,observed=args.observe,frames=frames,injection=injection_data,scene_transitions=transitions,matched_controlled_reference=args.observe or args.repeat or args.mirroring or args.raster_states,rom_sha256=hashlib.sha256((BASE/'basketball.nes').read_bytes()).hexdigest(),saved_state_sha256=hashlib.sha256((BASE/'states/slot-91.fcs').read_bytes()).hexdigest(),scope='controlled original whole-console result after explicit score/progress overrides, not earned win or natural fourteen-match campaign')
    (target/'postresult-report.json').write_text(json.dumps(report,indent=2)+'\n');print(json.dumps(report))
if __name__=='__main__':main()
