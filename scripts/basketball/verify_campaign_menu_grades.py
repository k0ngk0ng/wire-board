"""Original Scene2 equipped-grade calls; controlled input matrix, not matches.
Execute unchanged bank3 8CEE through 8FC1/AD8F, stop8D26 before glyph rendering.
Prospective platform preview is a composition of original Equip + these grades;
source inventory hover itself only displays the currently equipped state.
"""
import argparse,base64,gzip,hashlib,inspect,itertools,json
from pathlib import Path
from verify_campaign_component import base,PRG,Memory,MPU,progress,ROOT,ROM
OUT=ROOT/'games/basketball/engine/testdata'
def call(r):
 m=Memory(PRG,r);m[0x8000:0xc000]=PRG[3*16384:4*16384]
 cpu=MPU(memory=m,pc=0x8cee);cpu.sp=255;cpu.p=0x20;cpu.stPushWord(0x6fff)
 random_calls=0
 for tick in range(5000):
  if cpu.pc==0x8d26:
   after=bytearray(m[:2048]);assert progress(after)==progress(r);assert after[0x48:0x4c]==r[0x48:0x4c] and random_calls==0
   return {name:after[address] for name,address in [('Run',0x615),('Jump',0x619),('Shot',0x61d),('Recovery',0x621),('Attack',0x625)]}
  if cpu.pc==0xf9c3:random_calls+=1
  cpu.step()
 raise RuntimeError(('source grade call did not finish',hex(cpu.pc)))
def main():
 parser=argparse.ArgumentParser();parser.add_argument('--check',action='store_true');args=parser.parse_args()
 assert hashlib.sha256(ROM).hexdigest()=='0fa28f60abb8d2ccbbeee90e79e3fa160a5cf32feb90f47ebf1a89168cff569b'
 rows=[];origin=base()
 for mode,character,shirt,pants,shoes in itertools.product(range(2),range(4),[-1,*range(10)],[-1,*range(10,21)],[-1,*range(21,32)]):
  r=bytearray(origin);r[0x588]=2;r[0x589]=155;r[0x58d]=mode;r[0x586:0x588]=bytes([0,1]);r[0x6d7]=0;r[0x6d9]=character
  slots=[shirt,pants,shoes];r[0x6e4+character*3:0x6e7+character*3]=bytes(x&255 for x in slots)
  rows.append(dict(mode=mode,character=character,slots=slots,grades=call(r)))
 assert len(rows)==12672
 payload=json.dumps(rows,separators=(',',':')).encode();packed=gzip.compress(payload,mtime=0)
 metadata=dict(records=len(rows),controlledBaseRAM=base64.b64encode(origin).decode(),controlledBaseRAMSHA256=hashlib.sha256(origin).hexdigest(),cpuImplementationSHA256=hashlib.sha256(Path(inspect.getfile(MPU)).read_bytes()).hexdigest(),mapperSHA256=hashlib.sha256((ROOT/'scripts/basketball/verify_pass_floor_original.py').read_bytes()).hexdigest(),controlledOverrides='Scene2,phase155,teams[0,1],selectedSide0,selectedCharacter/mode/owned slot values from each row; SP255,P20,bank3 mapped at8000; untouched original instructions',romSHA256=hashlib.sha256(ROM).hexdigest(),fixtureJSONSHA256=hashlib.sha256(payload).hexdigest(),entry='bank3:8CEE',stop='bank3:8D26 before glyph rendering',scope='isolated original instructions, modes0/1, all four own characters and1584 legal empty-or-equipped slot combinations; not naturally earned inventories or complete matches',noCallsSkipped=True,randomCalls=0,progressUnchanged=True,retainedREA='games/basketball/OPENING_VERIFICATION.md and .local/basketball/rea/opening/attributes/batch.json',previewScope='source displays equipped state; hypothetical post-confirmation projection composes original equip rule with this program')
 fixture=OUT/'campaign-menu-grades.json.gz';meta=OUT/'campaign-menu-grades-source.json'
 if args.check:assert fixture.read_bytes()==packed;assert json.loads(meta.read_text())==metadata
 else:fixture.write_bytes(packed);meta.write_text(json.dumps(metadata,indent=2)+'\n')
 print(json.dumps(dict(status='passed',records=len(rows),fixtureJSONSHA256=metadata['fixtureJSONSHA256'])))
if __name__=='__main__':main()
