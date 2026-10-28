"""Record real scroll-register writes, background maps and DMA for scene QA.

Failure cases fixed before recording/rendering: assuming RAM scroll is already
visible, losing the 240-pixel wrap/nametable switch, attribute quadrants reversed,
universal color confused with palette-local zero, behind-background sprites
drawn above opaque terrain, stale DMA, and clipping flags ignored.
"""
import base64
import gzip
import hashlib
import json
import os
from pathlib import Path
import shutil
import sys
import zipfile

ROOT = Path(__file__).resolve().parents[2]
os.environ.setdefault('CONTRA_REFERENCE_ROOT', str(ROOT/'.local/contra-background'))
from capture_reference import BASE, run

HOOK = r'''
local display=assert(io.open(out..'/background.jsonl','w'))
local dma=assert(io.open(out..'/background-dma.bin','wb'))
local oam=string.rep(string.char(0xf4),256)
local control=0;local mask=0;local phase=0;local scrollX=0;local scrollY=0
local writes=0;local seq=0;local dmaFrame=-1
local selected=SELECTED
memory.registerwrite(0x4014,1,function(a,s,v) oam=memory.readbyterange(v*256,256);dmaFrame=emu.framecount() end)
memory.registerwrite(0x2000,1,function(a,s,v) control=v end)
memory.registerwrite(0x2001,1,function(a,s,v) mask=v end)
memory.registerread(0x2002,1,function() phase=0 end)
memory.registerwrite(0x2005,1,function(a,s,v)
 if phase==0 then scrollX=v else scrollY=v end
 phase=1-phase;writes=writes+1
end)
emu.registerafter(function()
 seq=seq+1
 if selected[seq] then
  dma:write(oam)
  display:write(string.format('{"frame":%d,"emulatorFrame":%d,"dmaFrame":%d,"control":%d,"mask":%d,"scrollX":%d,"scrollY":%d,"scrollWrites":%d}\n',seq,emu.framecount(),dmaFrame,control,mask,scrollX,scrollY,writes))
  writefile(string.format('background-%05d-ppu.bin',seq),bytes(0,0x4000,ppu.readbyte))
 end
 writes=0
end)
local colors={}
for i=0,63 do local r,g,b,a=gui.parsecolor(string.format('P%02x',i));colors[#colors+1]=string.char(r,g,b) end
writefile('background-colors.rgb',table.concat(colors))
function finish_probe()display:close();dma:close()end
'''


def main():
    from PIL import Image
    name=sys.argv[1] if len(sys.argv)>1 else 'background-v1'
    source=ROOT/'.local/contra/states/coop-play.fcs'
    (BASE/'states').mkdir(parents=True,exist_ok=True)
    target=BASE/'states/coop-play.fcs'
    if target.exists():assert source.read_bytes()==target.read_bytes()
    else:shutil.copyfile(source,target)
    captures=[1,2,30,60,90,120,160,200,240,320,400,480,600]
    spec={'name':name,'load_state':'coop-play','frames':600,'captures':captures,
          'segments':[{'start':1,'end':600,'player':p,'keys':['right','B']} for p in (1,2)] +
                     [{'start':n,'end':n+1,'player':p,'keys':['A']} for p in (1,2) for n in range(60,600,90)],
          'hooks_lua':HOOK.replace('SELECTED','{'+','.join(f'[{n}]=true' for n in captures)+'}')}
    path=BASE/(name+'.json');path.write_text(json.dumps(spec,indent=2)+'\n')
    observed=run(path)
    rows=[json.loads(line) for line in (observed/'background.jsonl').read_text().splitlines()]
    oam=(observed/'background-dma.bin').read_bytes();assert len(oam)==256*len(rows)
    colors=(observed/'background-colors.rgb').read_bytes();assert len(colors)==192
    with zipfile.ZipFile('/Users/jason/Documents/nes/roms/03/魂斗罗日版(完全版).zip') as archive:rom=archive.read('Contra (J).nes')
    rom_sha=hashlib.sha256(rom).hexdigest();assert rom_sha=='6ba53139fa88b8de1ae527c438bda6f1541d1ee7df26d63dec5164a32d166bfe'
    chr_bytes=rom[16+131072:];pages={};samples=[]
    for i,row in enumerate(rows):
        assert row['dmaFrame']==row['emulatorFrame'] and row['scrollWrites']==2,row
        ppu=(observed/f'background-{row["frame"]:05d}-ppu.bin').read_bytes()
        # This scene uses vertical mirroring: the two vertical nametables repeat.
        assert ppu[0x2000:0x2800]==ppu[0x2800:0x3000]
        mapping=[]
        for slot in range(8):
            data=ppu[slot*1024:(slot+1)*1024]
            matches=[n for n in range(128) if data==chr_bytes[n*1024:(n+1)*1024]]
            assert len(matches)==1
            key=f'page-{matches[0]:03d}';pages[key]=data.hex();mapping.append(key)
        image=Image.open(observed/f'frame-{row["frame"]:05d}.png').convert('RGB');assert image.size==(256,240)
        samples.append({**row,'mapping':mapping,'palette':ppu[0x3f00:0x3f20].hex(),
                        'screens':[{'tiles':ppu[base:base+960].hex(),'attributes':ppu[base+960:base+1024].hex()} for base in (0x2000,0x2400)],
                        'oam':oam[i*256:(i+1)*256].hex(),'expectedRGB':base64.b64encode(image.tobytes()).decode()})
    fixture={'source':json.loads((observed/'source.json').read_text()),'rgb':colors.hex(),'pages':pages,'samples':samples}
    packed=gzip.compress(json.dumps(fixture,separators=(',',':')).encode(),mtime=0)
    out=ROOT/'games/contra/tests/testdata';out.mkdir(parents=True,exist_ok=True)
    fixture_path=out/'background-original.json.gz'
    if fixture_path.exists():
        previous=json.loads(gzip.decompress(fixture_path.read_bytes()))
        assert all(previous[k]==fixture[k] for k in ('samples','pages','rgb')), 'new capture differs from retained scene oracle'
        packed=fixture_path.read_bytes()
    else:
        fixture_path.write_bytes(packed)
    source_record={'source_is_original_execution':True,'romSHA256':rom_sha,
         'fixtureSHA256':hashlib.sha256(packed).hexdigest(),'scenario':spec,'source':fixture['source'],
         'observedScrollWrites':'Exactly one X/Y pair and matching DMA per observed frame.',
         'scope':'First-stage natural two-player frames; no full-game or other-stage claim.'}
    source_path=out/'background-original-source.json'
    if not source_path.exists():source_path.write_text(json.dumps(source_record,indent=2)+'\n')
    (observed/'background-source.json').write_text(json.dumps(source_record,indent=2)+'\n')
    print(json.dumps({'frames':len(samples),'pages':len(pages),'scrollX':sorted({s['scrollX'] for s in samples}),'fixtureSHA256':hashlib.sha256(packed).hexdigest()}))


if __name__=='__main__':main()
