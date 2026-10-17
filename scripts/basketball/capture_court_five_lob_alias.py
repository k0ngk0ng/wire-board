"""Observe the actual writers of unowned auxiliary launch-height aliases."""
import gzip,hashlib,json
from capture_reference import BASE,ROOT,run

HOOK=r'''
local log=assert(io.open(out..'/alias-writes.jsonl','w'))
local entries=assert(io.open(out..'/lob-entries.jsonl','w'))
local columns=assert(io.open(out..'/completed-columns.jsonl','w'))
for _,address in ipairs({0x135,0x142}) do
 memory.registerwrite(address,1,function(a,size,value)
  log:write(string.format('{"frame":%d,"address":%d,"value":%d,"pc":%d,"bank":%d,"sp":%d,"a":%d,"x":%d,"y":%d,"court":%d,"column":%d,"phase":%d}\n',emu.framecount(),a,value,memory.getregister('pc'),memory.readbyte(0xbfff),memory.getregister('s'),memory.getregister('a'),memory.getregister('x'),memory.getregister('y'),memory.readbyte(0x58b),memory.readbyte(0x3a1),memory.readbyte(0x589)))
 end)
end
memory.registerexec(0xb07e,1,function()
 if memory.readbyte(0xbfff)~=6 then return end
 local i=memory.getregister('x')
 entries:write(string.format('{"frame":%d,"object":%d,"owner":%d,"action":%d,"heightLow":%d,"heightHigh":%d}\n',emu.framecount(),i,memory.readbyte(0x539+i),memory.readbyte(0x512+i),memory.readbyte(0x135),memory.readbyte(0x142)))
end)
memory.registerexec(0xbcfe,1,function()
 if memory.readbyte(0xbfff)~=1 then return end
 columns:write(string.format('{"frame":%d,"court":%d,"column":%d,"phase":%d,"low":%d,"high":%d}\n',emu.framecount(),memory.readbyte(0x58b),memory.readbyte(0x3a1),memory.readbyte(0x589),memory.readbyte(0x135),memory.readbyte(0x142)))
end)
function finish_probe()log:close();entries:close();columns:close()end
'''


def main():
    reference=BASE/'runs/court-5-reference';spec=json.loads((reference/'scenario.json').read_text())
    spec['name']='court-5-lob-alias-writes-v3';spec['hooks_lua']+='\n'+HOOK
    path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');run(path)
    out=BASE/'runs'/spec['name'];assert (out/'ram.bin').read_bytes()==(reference/'ram.bin').read_bytes()
    writes=[json.loads(line) for line in (out/'alias-writes.jsonl').read_text().splitlines()]
    entries=[json.loads(line) for line in (out/'lob-entries.jsonl').read_text().splitlines()]
    oracle=json.loads(gzip.decompress((ROOT/'games/basketball/engine/testdata/background-launch-alias.json.gz').read_bytes()));lookup={(r['court'],r['column'],r['phase']):r for r in oracle}
    columns=[json.loads(line) for line in (out/'completed-columns.jsonl').read_text().splitlines()]
    for row in columns:
        expected=lookup[row['court'],row['column'],0 if row['phase']&127==0 else 1]
        assert (row['low'],row['high'])==(expected['low'],expected['high']),(row,expected)
    result=dict(unchanged_gameplay_trace=True,scope='original Court5 reference with existing isolated court selection; read-only write hooks',writes=len(writes),matchedCompletedColumns=len(columns),writers=sorted(set((v['bank'],v['pc']) for v in writes)),lobEntries=len(entries),unownedEntries=sum(v['owner']>=128 for v in entries),ram_sha256=hashlib.sha256((out/'ram.bin').read_bytes()).hexdigest(),source=json.loads((out/'source.json').read_text()))
    dest=ROOT/'.local/basketball/rea/court-five/alias-writers.json';dest.write_text(json.dumps(result,indent=2)+'\n');print(json.dumps(result))


if __name__=='__main__':main()
