"""Per-slot original Contra audio control oracle; see pre-implementation plan.

The source ROM is read-only. An explicitly declared one-time isolation clears
other slot IDs immediately after the real request. No PCM is exported.
"""
import argparse
import base64
from concurrent.futures import ThreadPoolExecutor, as_completed
import gzip
import hashlib
import json
import os
from pathlib import Path
import shutil
import struct
import subprocess
import time
import zipfile

ROOT = Path(__file__).resolve().parents[2]
BASE = ROOT / '.local/contra-audio-control-library'
EVIDENCE = ROOT / 'docs/research/nes-contra/evidence'
ROM_PATH = Path('/Users/jason/Documents/nes/roms/03/魂斗罗日版(完全版).zip')
ROM_SHA = '6ba53139fa88b8de1ae527c438bda6f1541d1ee7df26d63dec5164a32d166bfe'
EXE = Path('/opt/homebrew/Cellar/fceux/2.6.6_12/libexec/fceux')
LAUNCHER = Path('/opt/homebrew/bin/fceux')
WIDTH = 2048 + 5
FIRST_IDS = [2, 11, 22, 24, 26, 28, 36, 38, 40, 47, 51, 56, 70, 74, 78,
             82, 86, 90, 94, 98, 102, 106, 110, 114, 116, 117, 119, 123,
             127, 128, 129]


def sha(data):
    return hashlib.sha256(data).hexdigest()


def atomic_json(path, value):
    data = (json.dumps(value, ensure_ascii=False, indent=2) + '\n').encode()
    pending = path.with_suffix(path.suffix + '.tmp')
    pending.write_bytes(data)
    pending.replace(path)


def states_from_bytes(data):
    assert len(data) % WIDTH == 0, 'incomplete state record'
    return [data[n:n + WIDTH] for n in range(0, len(data), WIDTH)]


def event_shape(event):
    return {k: v for k, v in event.items() if k != 'tick'}


def verify_cycle(states, events, start, period):
    """Verify the equality boundary and the following full cycle, not a prefix."""
    assert start >= 1 and period > 0
    assert len(states) >= start + 2 * period
    assert states[start - 1] == states[start + period - 1]
    assert states[start:start + period] == states[start + period:start + 2 * period]
    first = [(e['tick'] - start, event_shape(e)) for e in events
             if start <= e['tick'] < start + period]
    second = [(e['tick'] - start - period, event_shape(e)) for e in events
              if start + period <= e['tick'] < start + 2 * period]
    assert first == second, 'cycle request mismatch'


def validate_result(result, states, events):
    assert result['ticks'] == len(states)
    slot, sid = result['slot'], result['id']
    assert all(-1 <= e['tick'] < len(states) for e in events)
    if slot is None:
        assert sid in (127, 128, 129) and result['status'] == 'direct-dpcm'
        assert len(states) == 1
        return
    assert result['initial']['slots'][slot] == sid, 'request rejected'
    active = [state[0x106 + slot] == sid for state in states]
    if result['status'] == 'released':
        tick = result['releaseTick']
        assert tick == len(states) - 1 and all(active[:tick]) and not active[tick]
    elif result['status'] == 'loop':
        assert result['releaseTick'] == -1 and all(active)
        verify_cycle(states, events, result['loopStartTick'], result['loopTicks'])
    else:
        assert result['status'] == 'unknown' and result['releaseTick'] is None
        assert all(active)


def verifier_self_check():
    """Acceptance checks written before the recorder implementation."""
    states = [b'A', b'B', b'C', b'B', b'C']
    events = [{'tick': 1, 'id': 2}, {'tick': 3, 'id': 2}]
    # The boundary state before the cycle must also match.
    states[0] = b'C'
    verify_cycle(states, events, 1, 2)
    rejected = 0
    for changed_states, changed_events in [
        (states[:-1], events),
        (states[:-1] + [b'X'], events),
        (states, events[:-1] + [{'tick': 3, 'id': 3}]),
        ([b'X'] + states[1:], events),
    ]:
        try:
            verify_cycle(changed_states, changed_events, 1, 2)
        except AssertionError:
            rejected += 1
    assert rejected == 4
    zero = bytes(WIDTH)
    unknown = {'id': 2, 'slot': 5, 'ticks': 1, 'initial': {'slots': [0] * 5 + [2]},
               'status': 'unknown', 'releaseTick': None}
    active = bytearray(zero)
    active[0x10b] = 2
    validate_result(unknown, [bytes(active)], [])
    try:
        validate_result(unknown | {'releaseTick': -1}, [bytes(active)], [])
    except AssertionError:
        rejected += 1
    assert rejected == 5
    return {'checksPassed': 7, 'mutationsRejected': rejected}


def read_definitions(rom):
    bank = rom[16 + 0x4000:16 + 0x8000]
    table = int.from_bytes(bank[0x717:0x719], 'little')
    assert table == 0x8920
    definitions = []
    for sid in range(1, 127):
        offset = table - 0x8000 + sid * 3
        count = ((bank[offset] >> 3) & 3) + 1
        entries = []
        for i in range(count):
            control, address = struct.unpack_from('<BH', bank, offset + i * 3)
            assert control & 7 < 6 and 0x8000 <= address < 0xC000
            entries.append({'index': i, 'control': control, 'slot': control & 7,
                            'sequenceAddress': address})
        definitions.append({'id': sid, 'entries': entries, 'slots': []})
    for sid in (127, 128, 129):
        definitions.append({'id': sid, 'entries': [], 'slots': [], 'kind': 'direct-dpcm'})
    return definitions


def carrier(rom, sid, context):
    prg = rom[16:131088]
    data = bytearray(prg[0x4000:0x8000] + prg[0x1C000:])
    init = bytes([0x20, 0x0B, 0xF9, 0x20, 0x19, 0x87, 0xA9, 0,
                  0x8D, 0xF0, 5, 0x8D, 0xF1, 5, 0x85, 0x25,
                  0xA9, context, 0x8D, 0x94, 1, 0x60])
    # The private counters stop changing after 120 quiet updates. Branches are
    # patched from labels; no original instruction or sequence byte is changed.
    play = bytearray([0xAD, 0xF1, 5, 0xD0, 0, 0xAD, 0xF0, 5,
                      0xC9, 120, 0xF0, 0, 0xEE, 0xF0, 5, 0x4C, 0, 0])
    request = len(play)
    play.extend([0xEE, 0xF1, 5, 0xA9, sid, 0x20, 0xBC, 0xF9, 0xEA])
    returned = 0xBF80 + len(play) - 1
    update = len(play)
    play.extend([0x20, 0xD5, 0x80, 0xEA, 0x60])
    after = 0xBF80 + len(play) - 2
    play[4] = update - 5
    play[11] = request - 12
    struct.pack_into('<H', play, 16, 0xBF80 + update)
    assert data[0x3F80:0x3F80 + len(play)] == bytes([255]) * len(play)
    assert data[0x3FE0:0x3FE0 + len(init)] == bytes([255]) * len(init)
    data[0x3F80:0x3F80 + len(play)] = play
    data[0x3FE0:0x3FE0 + len(init)] = init
    # Full bank identity outside the two explicitly declared padding intervals.
    untouched = bytearray(data)
    untouched[0x3F80:0x3F80 + len(play)] = bytes([255]) * len(play)
    untouched[0x3FE0:0x3FE0 + len(init)] = bytes([255]) * len(init)
    assert untouched == prg[0x4000:0x8000] + prg[0x1C000:]
    header = bytearray(128)
    header[:8] = b'NESM\x1a\x01\x01\x01'
    struct.pack_into('<HHH', header, 8, 0x8000, 0xBFE0, 0xBF80)
    struct.pack_into('<H', header, 110, 16639)
    return bytes(header + data), returned, after


LUA = r'''
local out=assert(os.getenv('NES_CONTROL_OUT'))
local sid=SOUND_ID;local target=TARGET_SLOT;local limit=MAX_TICKS
local states=assert(io.open(out..'/states.bin','wb'))
local events=assert(io.open(out..'/requests.jsonl','w'))
local tick=-1;local requested=false;local active=false;local done=false
local records={};local requests={};local seen={};local pending=nil;local wrapperCaller=nil
local function byte(a)return memory.readbyte(a)end
local function reg(n)return AND(memory.getregister(n),255)end
local function word(a)return byte(a)+256*byte(a+1)end
local function caller()
 local s=reg('s');return byte(0x100+AND(s+1,255))+256*byte(0x100+AND(s+2,255))-2
end
local function registers()
 return string.format('{"a":%d,"x":%d,"y":%d,"p":%d,"s":%d,"carry":%d}',reg('a'),reg('x'),reg('y'),reg('p'),reg('s'),AND(reg('p'),1))
end
local function codes()
 local t={};for i=0,5 do t[#t+1]=tostring(byte(0x106+i))end;return '['..table.concat(t,',')..']'
end
memory.registerexec(0xf9bc,1,function()wrapperCaller=caller()end)
memory.registerexec(0x87a2,1,function()
 local source=byte(0x120);local code=source<6 and byte(0x106+source)or -1
 local cleared=active and source==target and code==0
 local shape=string.format('"id":%d,"internal":%s,"sourceSlot":%d,"sourceCode":%d,"targetCode":%d,"afterClear":%s,"callerPC":%d,"wrapperCallerPC":%s',reg('a'),tostring(active),source,code,target>=0 and byte(0x106+target)or -1,tostring(cleared),caller(),wrapperCaller and tostring(wrapperCaller)or 'null')
 events:write(string.format('{"tick":%d,%s}\n',tick,shape))
 if active then requests[tick]=requests[tick]or {};table.insert(requests[tick],shape)end
 wrapperCaller=nil
end)
memory.registerexec(REQUEST_RETURN,1,function()
 assert(not requested)
 local before=memory.readbyterange(0,2048)
 local f=assert(io.open(out..'/initial-ram.bin','wb'));f:write(before);f:close()
 local original=codes();local ret=registers();local seq=target>=0 and byte(0x112+target)+256*byte(0x118+target)or 0
 local isolation={}
 for slot=0,5 do
  if slot~=target and target>=0 then
   isolation[#isolation+1]=string.format('{"address":%d,"before":%d,"after":0}',0x106+slot,byte(0x106+slot))
   memory.writebyte(0x106+slot,0)
  end
 end
 local f=assert(io.open(out..'/initial.json','w'))
 f:write(string.format('{"registers":%s,"slots":%s,"sequenceAddress":%d,"isolation":[%s]}\n',ret,original,seq,table.concat(isolation,',')));f:close()
 requested=true
end)
memory.registerexec(0x80d5,1,function()
 if requested and not done then tick=tick+1;active=true end
end)
local function finish(status,release,start,period)
 done=true;states:close();events:close()
 local f=assert(io.open(out..'/terminal.json','w'))
 f:write(string.format('{"status":"%s","releaseTick":%s,"loopStartTick":%s,"loopTicks":%s,"ticks":%d}\n',status,release or 'null',start or 'null',period or 'null',tick+1));f:close()
 local m=assert(io.open(out..'/complete.tmp','w'));m:write('complete');m:close();assert(os.rename(out..'/complete.tmp',out..'/complete'))
end
memory.registerexec(AFTER_UPDATE,1,function()
 if not active or done then return end
 active=false
 local state=memory.readbyterange(0,2048)..string.char(reg('a'),reg('x'),reg('y'),reg('p'),reg('s'))
 states:write(state);records[tick]=state
 if target<0 then finish('direct-dpcm',nil,nil,nil);return end
 if byte(0x106+target)~=sid then finish('released',tostring(tick),nil,nil);return end
 if pending then
  local other=tick-pending.period
  assert(records[tick]==records[other],'full RAM/register cycle mismatch at '..tick)
  assert(table.concat(requests[tick]or {},'\n')==table.concat(requests[other]or {},'\n'),'request cycle mismatch at '..tick)
  if tick==pending.last then finish('loop','-1',tostring(pending.start),tostring(pending.period));return end
 else
  local prev=seen[state]
  if prev then
   pending={start=prev+1,period=tick-prev,last=tick+(tick-prev)}
  else seen[state]=tick end
 end
 if tick+1>=limit then finish('unknown',nil,nil,nil)end
end)
emu.speedmode('turbo')
while not done do emu.frameadvance()end
while true do end
'''


def key_for(sid, slot, context):
    return f'{sid:02x}-slot{slot if slot is not None else "dpcm"}-context{context}'


def record_case(rom, sid, slot, context, label, limit):
    key = key_for(sid, slot, context)
    path = BASE / label / key
    nsf, returned, after = carrier(rom, sid, context)
    lua = (LUA.replace('SOUND_ID', str(sid)).replace('TARGET_SLOT', str(-1 if slot is None else slot))
           .replace('MAX_TICKS', str(limit)).replace('REQUEST_RETURN', hex(returned))
           .replace('AFTER_UPDATE', hex(after)))
    identity = {'id': sid, 'slot': slot, 'context0194': context, 'maxTicks': limit,
                'carrierSHA256': sha(nsf), 'luaSHA256': sha(lua.encode()), 'romSHA256': ROM_SHA,
                'executableSHA256': sha(EXE.read_bytes()), 'launcherSHA256': sha(LAUNCHER.read_bytes())}
    if path.exists():
        assert (path / 'complete').read_text() == 'complete', path
        assert json.loads((path / 'identity.json').read_text()) == identity, path
    else:
        path.mkdir(parents=True)
        atomic_json(path / 'identity.json', identity)
        (path / 'carrier.nsf').write_bytes(nsf)
        (path / 'capture.lua').write_text(lua)
        for name in ('fceux-home', 'runtime', 'tmp'):
            (path / name).mkdir()
        (path / 'fceux-home/fceux.cfg').write_text('SDL.OpenGL = 0\nSDL.VideoDriver = 2\nSDL.VideoVsync = 0\nSDL.NoConfig = 1\n')
        env = dict(os.environ, QT_QPA_PLATFORM='offscreen', SDL_AUDIODRIVER='dummy',
                   FCEUX_HOME=str(path / 'fceux-home'), FCEUX_CONFIG_DIR=str(path / 'fceux-home'),
                   XDG_RUNTIME_DIR=str(path / 'runtime'), TMPDIR=str(path / 'tmp'), NES_CONTROL_OUT=str(path))
        cmd = ['sandbox-exec', '-f', str(BASE / 'workspace.sb'), str(LAUNCHER), '--pal', '0',
               '--sound', '1', '--soundrate', '44100', '--soundq', '2', '--volume', '192',
               '--loadlua', str(path / 'capture.lua'), str(path / 'carrier.nsf')]
        with (path / 'emulator.log').open('w') as log:
            process = subprocess.Popen(cmd, env=env, stdout=log, stderr=subprocess.STDOUT)
            started = time.monotonic()
            try:
                while process.poll() is None and not (path / 'complete').exists():
                    if time.monotonic() - started > max(90, limit / 100):
                        raise TimeoutError(str(path))
                    if 'Lua thread bombed out' in (path / 'emulator.log').read_text():
                        raise RuntimeError((path / 'emulator.log').read_text()[-3000:])
                    time.sleep(.08)
                assert (path / 'complete').read_text() == 'complete', path
            finally:
                if process.poll() is None:
                    process.kill()
                process.wait()
    result = json.loads((path / 'terminal.json').read_text()) | identity
    result['initial'] = json.loads((path / 'initial.json').read_text())
    state_path = path / 'states.bin'
    states_bytes = state_path.read_bytes() if state_path.exists() else gzip.decompress((path / 'states.bin.gz').read_bytes())
    states = states_from_bytes(states_bytes)
    events_bytes = (path / 'requests.jsonl').read_bytes()
    events = [json.loads(row) for row in events_bytes.splitlines()]
    validate_result(result, states, events)
    result.update(caseKey=key, sourceCase=str(path.relative_to(ROOT)),
                  statesSHA256=sha(states_bytes), requestsSHA256=sha(events_bytes),
                  initialRAMSHA256=sha((path / 'initial-ram.bin').read_bytes()))
    origin = [e for e in events if e['tick'] == -1]
    assert len(origin) == 1 and origin[0]['id'] == sid and not origin[0]['internal']
    timeline = [e for e in events if e['internal'] and e['sourceSlot'] == slot and e['sourceCode'] == sid]
    result['endCallbacks'] = [e for e in events if e['afterClear']]
    if result['status'] == 'loop':
        start, period = result['loopStartTick'], result['loopTicks']
        result['prefixRequests'] = [e for e in timeline if e['tick'] < start]
        result['loopRequests'] = [e | {'tick': e['tick'] - start} for e in timeline
                                  if start <= e['tick'] < start + period]
    else:
        result['prefixRequests'], result['loopRequests'] = timeline, []
    result['requestCount'] = len(events)
    atomic_json(path / 'result.json', result)
    if state_path.exists():
        compressed = gzip.compress(states_bytes, compresslevel=6, mtime=0)
        (path / 'states.bin.gz').write_bytes(compressed)
        assert gzip.decompress((path / 'states.bin.gz').read_bytes()) == states_bytes
        state_path.unlink()
    for name in ('fceux-home', 'runtime', 'tmp'):
        if (path / name).exists():
            shutil.rmtree(path / name)
    return result


def compile_library(rom, results, requested, label):
    definitions = read_definitions(rom)
    by_case = {r['caseKey']: r for r in results}
    variants = []
    for definition in definitions:
        sid = definition['id']
        targets = sorted({e['slot'] for e in definition['entries']}) or [None]
        for slot in targets:
            result = by_case.get(key_for(sid, slot, 0))
            if result is None:
                continue
            if slot is not None:
                entry = [e for e in definition['entries'] if e['slot'] == slot][-1]
                assert result['initial']['sequenceAddress'] == entry['sequenceAddress'], (sid, slot)
            definition['slots'].append(result)
        definition['complete'] = len(definition['slots']) == len(targets) and all(
            r['status'] != 'unknown' for r in definition['slots'])
    for sid in (119, 122, 123, 126):
        variant = by_case.get(key_for(sid, 5, 1))
        if variant:
            variants.append(variant)
    complete = all(d['complete'] for d in definitions) and len(variants) == 4 and all(v['status'] != 'unknown' for v in variants)
    return {'version': 1, 'source_is_original_execution': True, 'romSHA256': ROM_SHA,
            'archiveSHA256': sha(ROM_PATH.read_bytes()), 'captureLabel': label,
            'scope': 'One-time isolated original per-slot lifecycle; default $0194=0, additional $77/$7A/$7B/$7E slot5 context=1; no PCM or mixed-gameplay claim.',
            'recordLayout': {'ramBytes': 2048, 'registerOrder': ['A', 'X', 'Y', 'P', 'S'], 'bytesPerTick': WIDTH},
            'requestFields': {'callerPC': 'Top stack return address minus 2; a tail JMP retains an outer JSR location, so this is not always the immediate request instruction.',
                              'wrapperCallerPC': 'Top stack return address minus 2 observed at $F9BC entry, when that wrapper was used.',
                              'sourceSlot': 'Original $0120 at $87A2 entry; meaningful for internal requests during $80D5.',
                              'sourceCode': 'Original $0106+sourceSlot at request entry; zero for the captured after-clear callbacks.'},
            'coordinate': 'tick 0 is the first $80D5 after the initial request; releaseTick is the first tick whose post-update slot ID differs; loopStart follows the first equal boundary state.',
            'interventions': 'Verified FF-padding init/play wrapper, frozen private warmup counters $05F0/$05F1, pause=0, declared $0194 input, other $0106+slot values cleared only once after request return; original code/data preserved.',
            'coverage': {'complete': complete, 'status': 'complete' if complete else 'partial',
                         'requestedIDs': requested, 'completedCases': len(results),
                         'completeDefinitions': sum(d['complete'] for d in definitions),
                         'unknownCases': [r['caseKey'] for r in results if r['status'] == 'unknown']},
            'definitions': definitions, 'contextVariants': variants,
            'outOfScopeRequests': {'0': 'Invalid table entry; not treated as reset.', '130..255': 'Original entry ignores these IDs (carry=1); outside valid library.'}}


def pack_original(library):
    cases = []
    for result in [r for d in library['definitions'] for r in d['slots']] + library['contextVariants']:
        path = ROOT / result['sourceCase']
        states = (path / 'states.bin.gz').read_bytes()
        requests = (path / 'requests.jsonl').read_bytes()
        initial = (path / 'initial-ram.bin').read_bytes()
        assert sha(gzip.decompress(states)) == result['statesSHA256']
        assert sha(requests) == result['requestsSHA256'] and sha(initial) == result['initialRAMSHA256']
        cases.append({'result': result, 'statesGzipBase64': base64.b64encode(states).decode(),
                      'requestsJSONL': requests.decode(), 'initialRAMHex': initial.hex()})
    packed = {'romSHA256': ROM_SHA, 'source_is_original_execution': True,
              'recordLayout': library['recordLayout'], 'scope': library['scope'], 'cases': cases}
    target = EVIDENCE / 'audio-control-library-original.json.gz'
    temporary = target.with_suffix('.tmp')
    temporary.write_bytes(gzip.compress(json.dumps(packed, separators=(',', ':')).encode(), mtime=0))
    temporary.replace(target)
    return {'file': str(target.relative_to(ROOT)), 'sha256': sha(target.read_bytes()), 'caseCount': len(cases)}


def verify_saved_library():
    library_path = EVIDENCE / 'audio-control-library.json'
    library = json.loads(library_path.read_text())
    bundle_path = EVIDENCE / 'audio-control-library-original.json.gz'
    bundle = json.loads(gzip.decompress(bundle_path.read_bytes()))
    assert bundle['romSHA256'] == library['romSHA256'] == ROM_SHA
    expected = {r['caseKey']: r for d in library['definitions'] for r in d['slots']}
    expected.update({r['caseKey']: r for r in library['contextVariants']})
    assert len(bundle['cases']) == len(expected) == library['coverage']['completedCases']
    ticks = 0
    for case in bundle['cases']:
        result = case['result']
        assert expected.pop(result['caseKey']) == result
        raw = gzip.decompress(base64.b64decode(case['statesGzipBase64']))
        event_bytes = case['requestsJSONL'].encode()
        events = [json.loads(row) for row in event_bytes.splitlines()]
        assert sha(raw) == result['statesSHA256'] and sha(event_bytes) == result['requestsSHA256']
        assert sha(bytes.fromhex(case['initialRAMHex'])) == result['initialRAMSHA256']
        validate_result(result, states_from_bytes(raw), events)
        timeline = [e for e in events if e['internal'] and e['sourceSlot'] == result['slot'] and e['sourceCode'] == result['id']]
        assert result['endCallbacks'] == [e for e in events if e['afterClear']]
        if result['status'] == 'loop':
            start, period = result['loopStartTick'], result['loopTicks']
            assert result['prefixRequests'] == [e for e in timeline if e['tick'] < start]
            assert result['loopRequests'] == [e | {'tick': e['tick'] - start} for e in timeline if start <= e['tick'] < start + period]
        else:
            assert result['prefixRequests'] == timeline and result['loopRequests'] == []
        ticks += result['ticks']
    assert not expected
    return {'verifiedCases': len(bundle['cases']), 'verifiedTicks': ticks,
            'librarySHA256': sha(library_path.read_bytes()), 'originalBundleSHA256': sha(bundle_path.read_bytes())}


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--ids', default='all')
    parser.add_argument('--label', default='v1')
    parser.add_argument('--jobs', type=int, default=3)
    parser.add_argument('--max-ticks', type=int, default=32768)
    parser.add_argument('--verify-only', action='store_true')
    parser.add_argument('--pack-only', action='store_true')
    parser.add_argument('--repeat-of', help='Compare independent new processes to this prior label; preserve the primary library.')
    args = parser.parse_args()
    assert args.label.replace('-', '').replace('_', '').isalnum()
    checks = verifier_self_check()
    if args.pack_only:
        library = json.loads((EVIDENCE / 'audio-control-library.json').read_text())
        print(json.dumps(pack_original(library)))
        return
    if args.verify_only:
        print(json.dumps(checks | verify_saved_library()))
        return
    if args.repeat_of:
        assert args.repeat_of != args.label
        assert args.repeat_of.replace('-', '').replace('_', '').isalnum()
    requested = list(range(1, 130)) if args.ids == 'all' else FIRST_IDS if args.ids == 'first' else [int(x, 0) for x in args.ids.split(',')]
    assert len(set(requested)) == len(requested) and all(1 <= x <= 129 for x in requested)
    with zipfile.ZipFile(ROM_PATH) as archive:
        rom = archive.read('Contra (J).nes')
    assert sha(rom) == ROM_SHA
    archive_sha = sha(ROM_PATH.read_bytes())
    BASE.mkdir(parents=True, exist_ok=True)
    (BASE / 'workspace.sb').write_text('(version 1)\n(allow default)\n(deny file-write*)\n' +
        f'(allow file-write* (subpath {json.dumps(str(ROOT))}) (literal "/dev/null") (literal "/dev/tty"))\n')
    cases = []
    for definition in read_definitions(rom):
        if definition['id'] in requested:
            for slot in sorted({e['slot'] for e in definition['entries']}) or [None]:
                cases.append((definition['id'], slot, 0))
    for sid in (119, 122, 123, 126):
        if sid in requested:
            cases.append((sid, 5, 1))
    cases.sort(key=lambda c: (requested.index(c[0]), c[1] if c[1] is not None else -1, c[2]))
    results = []
    library_path = EVIDENCE / ('audio-control-library.json' if not args.repeat_of else f'audio-control-library-{args.label}.json')
    preserve_complete = library_path.exists() and json.loads(library_path.read_text())['coverage']['complete']
    repeated = []
    with ThreadPoolExecutor(max_workers=args.jobs) as pool:
        pending = {pool.submit(record_case, rom, sid, slot, context, args.label, args.max_ticks): (sid, slot, context)
                   for sid, slot, context in cases}
        for future in as_completed(pending):
            result = future.result()
            if args.repeat_of:
                source = BASE / args.repeat_of / result['caseKey'] / 'result.json'
                prior = json.loads(source.read_text())
                assert {k: v for k, v in result.items() if k != 'sourceCase'} == {k: v for k, v in prior.items() if k != 'sourceCase'}, result['caseKey']
                repeated.append(result['caseKey'])
            results.append(result)
            results.sort(key=lambda r: (r['id'], r['slot'] if r['slot'] is not None else -1, r['context0194']))
            if not preserve_complete:
                atomic_json(library_path, compile_library(rom, results, requested, args.label))
            print(json.dumps({k: result[k] for k in ('caseKey', 'status', 'releaseTick', 'loopStartTick', 'loopTicks', 'ticks', 'requestCount')}), flush=True)
    assert sha(ROM_PATH.read_bytes()) == archive_sha
    library = compile_library(rom, results, requested, args.label)
    atomic_json(library_path, library)
    packed = pack_original(library) if not args.repeat_of else None
    atomic_json(EVIDENCE / f'audio-control-library-{args.label}-verification.json',
                {'scope': library['scope'], 'coverage': library['coverage'], 'verifier': checks,
                 'scriptSHA256': sha(Path(__file__).read_bytes()), 'outputSHA256': sha(library_path.read_bytes()),
                 'archiveUnchanged': True, 'originalBundle': packed, 'repeatOf': args.repeat_of,
                 'independentIdenticalCases': sorted(repeated),
                 'cases': [{'caseKey': r['caseKey'], 'statesSHA256': r['statesSHA256'],
                                                     'requestsSHA256': r['requestsSHA256']} for r in results]})


if __name__ == '__main__':
    main()
