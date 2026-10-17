"""Observe original contact candidates and the later automatic-catch phase."""
import hashlib
import json
from capture_reference import ROOT,BASE,run

HOOK=r'''
local contacts=assert(io.open(out.."/contacts.jsonl","w"))
local catches=assert(io.open(out.."/catches.jsonl","w"))
local pendingContact=nil;local pendingCatch=nil
local function word(a,b) return memory.readbyte(a)+256*memory.readbyte(b) end
local function wide(a,b,c) return word(a,b)+65536*memory.readbyte(c) end
local function array(a,count)
 local values={};for i=0,count-1 do values[#values+1]=tostring(memory.readbyte(a+i)) end
 return '['..table.concat(values,',')..']'
end
local function object(i)
 local action=AND(memory.readbyte(0x512+i),127)
 local kind=memory.readbyte(0x3d6+i)
 local cursor=memory.readbyte(0x444+i)
 local shape=0
 if action>=0x60 then
  local variant=memory.readbyte(0x57a+i)
  local frames=word(0x8d10+variant*2,0x8d11+variant*2)
  shape=word(0x912f,0x9130)+8*memory.readbyte(frames+cursor)
 else
  local actions=word(0x8cf2+kind*2,0x8cf3+kind*2)
  local group=memory.readbyte(actions+action)
  local frames=word(0x8fa5+group*2,0x8fa6+group*2)
  shape=word(frames+cursor*2,frames+cursor*2+1)
 end
 local flags=0x9321;if kind==8 then flags=0x938b elseif kind>8 then flags=0x93a3 end
 local receive=word(0x9301+kind*2,0x9302+kind*2)
 return string.format('{"action":%d,"kind":%d,"variant":%d,"cursor":%d,"pose":%d,"disabled":%d,"owner":%d,"attackFlags":%d,"receiveFlags":%d,"shapeAddress":%d,"shape":%s,"x":%d,"depth":%d,"height":%d,"attachment":%d,"air":%d,"request":%d}',memory.readbyte(0x512+i),kind,memory.readbyte(0x57a+i),cursor,memory.readbyte(0x5b+i),memory.readbyte(0x3f4+i),memory.readbyte(0x539+i),memory.readbyte(flags+action),memory.readbyte(receive+action),shape,array(shape,8),wide(0x68+i,0x75+i,0x82+i),word(0x8f+i,0x9c+i),wide(0xa9+i,0xb6+i,0xc3+i),memory.readbyte(0x642+i),memory.readbyte(0x5f7+i),memory.readbyte(0x51f+i))
end
local function state()
 local objects={};for i=0,6 do objects[#objects+1]=object(i) end
 return string.format('{"objects":[%s],"rotation":%d,"candidates":%s,"holder":%d,"excludedCatcher":%d,"previousHolder":%d,"lastTouch":%d,"climbers":%s,"hoopOwners":%s,"ballGravity":%d,"ballVX":%d}',table.concat(objects,','),memory.readbyte(0x633),array(0x3c,7),memory.readbyte(0x53f),memory.readbyte(0x561),memory.readbyte(0x546),memory.readbyte(0x6a8),array(0x53d,2),array(0x540,6),word(0x4cc,0x4d9),word(0x47e,0x48b))
end
memory.registerexec(0x802f,1,function()
 if memory.readbyte(0xbfff)~=5 then return end
 assert(not pendingContact);pendingContact=string.format('{"frame":%d,"before":%s,',emu.framecount(),state())
end)
memory.registerexec(0x805c,1,function()
 if memory.readbyte(0xbfff)~=5 or not pendingContact then return end
 contacts:write(pendingContact..'"after":'..state()..'}\n');pendingContact=nil
end)
memory.registerexec(0x84c3,1,function()
 if memory.readbyte(0xbfff)~=5 then return end
 assert(not pendingCatch);pendingCatch=string.format('{"frame":%d,"before":%s,',emu.framecount(),state())
end)
memory.registerexec(0x801d,1,function()
 if memory.readbyte(0xbfff)~=5 or not pendingCatch then return end
 catches:write(pendingCatch..'"after":'..state()..'}\n');pendingCatch=nil
end)
function finish_probe()
 if pendingContact then writefile('incomplete-contact.json',pendingContact..'"incomplete":true}') end
 if pendingCatch then writefile('incomplete-catch.json',pendingCatch..'"incomplete":true}') end
 contacts:close();catches:close()
end
'''

def main():
    records={'contacts':[],'catches':[]};sources=[]
    for name in ['hoop-control-cpu-long','human-possession-baseline','ball-control-pass','control-a','control-b','control-run-brake','control-running-jump']:
        reference=BASE/'runs'/name
        spec=json.loads((reference/'scenario.json').read_text());spec.pop('save_slot',None);spec.update(name='contact-'+name,hooks_lua=HOOK)
        path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');run(path)
        observed=BASE/'runs'/spec['name'];assert (observed/'ram.bin').read_bytes()==(reference/'ram.bin').read_bytes()
        for kind in records:
            for line in (observed/(kind+'.jsonl')).read_text().splitlines():
                row=json.loads(line)
                if kind=='contacts':
                    assert row['before']['objects']==row['after']['objects'],'Contact search mutated object state'
                    row['after']={k:row['after'][k] for k in ('rotation','candidates')}
                else:
                    before,after=row['before'],row['after']
                    row['before']={k:before[k] for k in ('holder','excludedCatcher','previousHolder','lastTouch','climbers','hoopOwners')}
                    row['before'].update(candidate=before['candidates'][6],ballAction=before['objects'][6]['action'],actors=[dict(action=o['action'],control=o['owner'],flags=o['receiveFlags'],attachment=o['attachment'],air=o['air']) for o in before['objects'][:4]])
                    row['after']=dict(holder=after['holder'],lastTouch=after['lastTouch'],ballAction=after['objects'][6]['action'],actions=[o['action'] for o in after['objects'][:4]])
                records[kind].append(dict(scenario=name,**row))
        sources.append(dict(scenario=name,**json.loads((observed/'source.json').read_text())))
    for kind,rows in records.items():
        p=ROOT/f'games/basketball/engine/testdata/{kind}.json';p.write_text(json.dumps(rows,separators=(',',':'))+'\n')
        p.with_name(kind+'-source.json').write_text(json.dumps(dict(source_is_original_execution=True,records=len(rows),sha256=hashlib.sha256(p.read_bytes()).hexdigest(),sources=sources),indent=2)+'\n')
        print(kind,len(rows))
    profiles={}
    for row in records['contacts']:
        for o in row['before']['objects']:
            action=o['action']&127
            key=(o['kind'],action,o['variant'] if action>=96 else 0,o['cursor'])
            value=dict(kind=key[0],action=key[1],variant=key[2],cursor=key[3],shape=o['shape'],attackFlags=o['attackFlags'],receiveFlags=o['receiveFlags'])
            assert o['shapeAddress']>=0x8000
            if key in profiles:assert profiles[key]==value,'Dynamic contact profile requires investigation'
            profiles[key]=value
    rows=[profiles[key] for key in sorted(profiles)]
    p=ROOT/'games/basketball/engine/testdata/contact-profiles.json';p.write_text(json.dumps(rows,indent=2)+'\n')
    p.with_name('contact-profiles-source.json').write_text(json.dumps(dict(source_is_original_geometry_data=True,records=len(rows),sha256=hashlib.sha256(p.read_bytes()).hexdigest(),sources=sources),indent=2)+'\n')
    print('contact-profiles',len(rows))

if __name__=='__main__':main()
