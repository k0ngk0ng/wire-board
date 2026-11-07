"""Small scene previews from already verified independent renderer captures."""
from pathlib import Path
import hashlib, json, subprocess

ROOT=Path(__file__).resolve().parents[2]
ids=[0,2,4,6,8,10,12,14]
sources=[ROOT/'games/yuyuhakusho/verification/runtime-e2e/native/two-ai/0600.png']+[
 ROOT/f'games/yuyuhakusho/verification/additional-arena-e2e/native/render/{arena:x}/duel/0600.png'for arena in ids[1:]]
target=ROOT/'games/yuyuhakusho/assets/arena-previews.png'
command=['ffmpeg','-v','error','-y']
for source in sources:
 assert source.is_file();command+=['-i',str(source)]
filters=[f'[{i}:v]crop=320:176:0:32,scale=160:88:flags=neighbor[v{i}]'for i in range(8)]
filters.append(''.join(f'[v{i}]'for i in range(8))+'hstack=inputs=8[out]')
command+=['-filter_complex',';'.join(filters),'-map','[out]','-frames:v','1','-update','1',str(target)]
subprocess.run(command,cwd=ROOT,check=True)
manifest={'width':1280,'height':88,'bytes':target.stat().st_size,'sha256':hashlib.sha256(target.read_bytes()).hexdigest(),'scenes':[{'arena':arena,'source':str(source.relative_to(ROOT)),'sha256':hashlib.sha256(source.read_bytes()).hexdigest()}for arena,source in zip(ids,sources)],'method':'Lossless PNG atlas; crop original HUD from existing verified independent gameplay renders, then nearest-neighbor half-size. No emulator execution.'}
target.with_suffix('.json').write_text(json.dumps(manifest,indent=2)+'\n')
print(json.dumps({'scenes':8,'bytes':target.stat().st_size}))
