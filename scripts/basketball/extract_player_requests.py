"""Verify E18B at its dispatch boundary and retain human B world-action traces."""
import csv,gzip,hashlib,json,sys
from pathlib import Path
from extract_world_actions import ROOT,decode_world_action_state
sys.path.insert(0,str(ROOT/'.local/basketball/python'))
from py65.devices.mpu6502 import MPU
rom=(ROOT/'.local/nes-basket/basketball.nes').read_bytes()
assert hashlib.sha256(rom).hexdigest()=='0fa28f60abb8d2ccbbeee90e79e3fa160a5cf32feb90f47ebf1a89168cff569b'
prg=rom[16:16+131072];cpu=MPU();cpu.memory[0x8000:0xc000]=prg[6*16384:7*16384];cpu.memory[0xc000:]=prg[-16384:]
allowed=[];masks=[]
for requested in range(106):
 pointer=cpu.WordAt(0xe208+2*requested);masks.append(list(cpu.memory[pointer:pointer+14]));row=[]
 for current in range(106):
  accepted=None
  for entered in (0,128):
   cpu.pc=0xe18b;cpu.x=0;cpu.memory[0x512]=current|entered;cpu.memory[0x51f]=requested
   for step in range(200):
    if cpu.pc==0xe1ed:break
    cpu.step()
   assert cpu.pc==0xe1ed
   result=cpu.memory[0x51f]==requested|128
   assert result == (masks[-1][current//8] & (128>>(current%8)) == 0)
   assert cpu.memory[0x512] == (requested if result else current|entered)
   if entered==0:accepted=result
   else:assert accepted==result
  if accepted:row.append(current)
 allowed.append(row)
out=ROOT/'games/basketball/engine/testdata'
(out/'player-request-acceptance.json').write_text(json.dumps(allowed,separators=(',',':'))+'\n')
run=ROOT/'.local/nes-basket/runs/world-request-control-b';raw=(run/'world-actions-ram.bin').read_bytes();index=list(csv.DictReader((run/'world-actions.csv').open()));records=[]
for i,row in enumerate(index):
 b=raw[i*4096:i*4096+2048];a=raw[i*4096+2048:(i+1)*4096]
 records.append(dict(frame=int(row['frame']),before=decode_world_action_state(b),after=decode_world_action_state(a),context=dict(courtID=b[0x58b],controls=list(b[0x539:0x53d]),characters=list(b[0x3e3:0x3e7]),teams=list(b[0x586:0x588]),runGrades=list(b[0x615:0x619]),jumpGrades=list(b[0x619:0x61d]),shotSkills=list(b[0x61d:0x621]),recoveryGrades=list(b[0x621:0x625]),equipment=b[0x6e4],otherObjectArcFlags=list(b[0x6e9:0x6f0]),enhancementEnabled=b[0x58d]<128,enhancements=[b[0x6e5+3*b[0x3e3+j]] for j in range(4)])))
payload=(json.dumps(records,separators=(',',':'))+'\n').encode();(out/'world-request-control-b.json.gz').write_bytes(gzip.compress(payload,mtime=0))
report=dict(rom_sha256=hashlib.sha256(rom).hexdigest(),acceptance_executions=106*106*2,acceptance_boundary='E18B..E1ED before indirect handler',human_world_records=len(records),human_trace_sha256=hashlib.sha256(raw).hexdigest(),source=json.loads((run/'source.json').read_text()))
(out/'player-request-source.json').write_text(json.dumps(report,indent=2)+'\n')
# Keep the editable table export separate from its instruction-execution oracle.
p=ROOT/'.local/basketball/rea/player-request-masks.json';p.write_text(json.dumps(masks))
print('request executions',report['acceptance_executions'],'human world records',len(records))
