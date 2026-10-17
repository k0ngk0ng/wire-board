"""Find a Court2 chain using controller input, then replay that exact input.

The research controller operates only player 1's legal directions/A/B. It is
not game AI and is not shipped. Original CPU players, ball state, sound state,
random state and IRQ timing remain untouched after initial roster/court setup.
"""
import collections
import hashlib
import json
import sys
from capture_reference import ROOT,BASE,run
from capture_audio_world_order import CONFIG,HOOK

POLICY = r'''
local inputlog=assert(io.open(out..'/controller-input.jsonl','w'))
local inputstep=0
local released_until=0
local launch_started=nil
local keys={'A','B','up','down','left','right','start','select'}
local original_set=joypad.set
joypad.set=function(player,pad)
 if player~=1 then return original_set(player,pad) end
 inputstep=inputstep+1
 if memory.readbyte(0x58b)==2 and memory.readbyte(0x588)==1 and AND(memory.readbyte(0x589),127)==1 then
  pad={}
  local x=memory.readbyte(0x75)+256*memory.readbyte(0x82)
  local depth=memory.readbyte(0x9c)
  local holder=memory.readbyte(0x53f)
  local action=AND(memory.readbyte(0x512),127)
  if inputstep>=released_until then
   if holder==0 then
    if launch_started then
     local age=inputstep-launch_started
     local cycle=memory.readbyte(0x572)
     if action==0x1a and (cycle==2 or cycle==3) then
      released_until=inputstep+60;launch_started=nil
     elseif age<3 then pad.A=true;pad.B=true
     elseif age>=9 and age<90 then pad.B=true
     elseif age>=90 then released_until=inputstep+180;launch_started=nil end
    elseif x<214 then pad.right=true
    elseif x>238 then pad.left=true
    elseif depth<69 then pad.up=true
    elseif depth>70 then pad.down=true
    elseif memory.readbyte(0x46b)>=128 then pad.right=true
    elseif memory.readbyte(0x5f7)<128 then launch_started=inputstep;pad.A=true;pad.B=true end
   else
    launch_started=nil
    local target=6;if holder<4 then target=holder end
    local tx=memory.readbyte(0x75+target)+256*memory.readbyte(0x82+target)
    local td=memory.readbyte(0x9c+target)
    if x<tx-4 then pad.right=true elseif x>tx+4 then pad.left=true end
    if depth<td-3 then pad.up=true elseif depth>td+3 then pad.down=true end
    if inputstep%36<18 then pad.A=true end
    if holder==6 and math.abs(x-tx)<14 and math.abs(depth-td)<10 and memory.readbyte(0xbc)>12 and inputstep%60<12 then pad.A=true;pad.B=true end
   end
  end
 end
 original_set(player,pad)
 local selected={};for _,key in ipairs(keys) do if pad[key] then selected[#selected+1]='"'..key..'"' end end
 inputlog:write(string.format('{"step":%d,"keys":[%s]}\n',inputstep,table.concat(selected,',')))
end
local prior_finish=finish_probe
function finish_probe()prior_finish();inputlog:close()end
'''


def execute(spec):
    out=BASE/'runs'/spec['name'];path=ROOT/'.local/basketball/audio/world-order'/(spec['name']+'.json')
    path.parent.mkdir(parents=True,exist_ok=True);path.write_text(json.dumps(spec,indent=2)+'\n')
    if not ((out/'capture-complete.json').exists() and json.loads((out/'scenario.json').read_text())==spec):run(path)
    return out


def main():
    name=sys.argv[1] if len(sys.argv)>1 else 'audio-court2-controller-chain-v4'
    frames=int(sys.argv[2]) if len(sys.argv)>2 else 18000
    base=json.loads((BASE/'opening-from-menu.json').read_text());base.pop('save_slot',None)
    base.update(frames=frames,captures=[1450,2000,5000,10000,frames])
    config=CONFIG.replace('TEAM0','3').replace('TEAM1','3')
    observed=execute(dict(base,name=name+'-observed',hooks_lua=config+'\n'+HOOK+'\n'+POLICY))
    inputs=[json.loads(line) for line in (observed/'controller-input.jsonl').read_text().splitlines()]
    assert [row['step'] for row in inputs]==list(range(1,frames+1))
    segments=[]
    for row in inputs:
        if segments and row['keys']==segments[-1]['keys'] and row['step']==segments[-1]['end']+1:
            segments[-1]['end']=row['step']
        else:segments.append(dict(start=row['step'],end=row['step'],keys=row['keys']))
    reference=execute(dict(base,name=name+'-replay',segments=segments,hooks_lua=config))
    assert (reference/'ram.bin').read_bytes()==(observed/'ram.bin').read_bytes(),'controller replay diverged'
    for n in base['captures']:assert (reference/f'frame-{n:05d}.png').read_bytes()==(observed/f'frame-{n:05d}.png').read_bytes()
    rows=[json.loads(line) for line in (observed/'audio-world-order.jsonl').open()]
    chains=[json.loads(line) for line in (observed/'turn-chains.jsonl').open()]
    report={'scope':'configured Court2 and team3 roster; normal player-1 controller inputs replayed without the search policy; no later state injection',
        'frames':frames,'input_segments':len(segments),'ram_and_screenshots_match_static_input_replay':True,
        'counts':dict(collections.Counter(row['kind'] for row in rows)),'chains':chains,
        'input_trace':str((observed/'controller-input.jsonl').relative_to(ROOT)),
        'input_sha256':hashlib.sha256((observed/'controller-input.jsonl').read_bytes()).hexdigest(),
        'ram_sha256':hashlib.sha256((observed/'ram.bin').read_bytes()).hexdigest(),
        'events_sha256':hashlib.sha256((observed/'audio-world-order.jsonl').read_bytes()).hexdigest(),
        'chain_ram_sha256':hashlib.sha256((observed/'turn-chains.ram').read_bytes()).hexdigest(),
        'source':json.loads((observed/'source.json').read_text())}
    (ROOT/'.local/basketball/audio/world-order'/(name+'-summary.json')).write_text(json.dumps(report,indent=2)+'\n')
    print(json.dumps({key:report[key] for key in ['frames','input_segments','ram_and_screenshots_match_static_input_replay','counts']}))


if __name__=='__main__':main()
