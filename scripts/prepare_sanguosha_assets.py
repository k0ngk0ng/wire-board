#!/usr/bin/env python3
"""Prepare pinned classic artwork; no server addresses or credentials belong here."""
import argparse
import json
from pathlib import Path
import subprocess
import tempfile
import time
import urllib.request
from concurrent.futures import ThreadPoolExecutor

REVISION = 'e8768851bd8054db9fd1b63cd6f1feca813590d7'
BASE = f'https://raw.githubusercontent.com/Mogara/QSanguosha-v2/{REVISION}/'
GENERALS = 'caocao simayi xiahoudun zhangliao xuchu guojia zhenji liubei guanyu zhangfei zhugeliang zhaoyun machao huangyueying sunquan ganning lvmeng huanggai zhouyu daqiao luxun sunshangxiang huatuo lvbu diaochan'.split()
UNCHANGED = {'zhenji','zhugeliang','sunquan','sunshangxiang'}
CARDS = 'slash jink peach duel snatch dismantlement ex_nihilo amazing_grace god_salvation savage_assault archery_attack collateral nullification indulgence lightning crossbow double_sword qinggang_sword blade spear axe halberd kylin_bow ice_sword eight_diagram renwang_shield jueying dilu zhuahuangfeidian chitu dayuan zixing'.split()

def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('directory',type=Path)
    args=parser.parse_args()
    root=args.directory.resolve(); root.mkdir(parents=True,exist_ok=True)
    sources={f'generals/{g}.webp':f'image/fullskin/generals/full/{g if g in UNCHANGED else "nos_"+g}.png' for g in GENERALS}
    sources.update({f'cards/{c}.webp':f'image/big-card/{c}.png' for c in CARDS})
    def prepare(item):
        target,source=item; output=root/target; output.parent.mkdir(parents=True,exist_ok=True)
        if output.exists() and output.read_bytes()[:4]==b'RIFF':return target
        for attempt in range(3):
            try:
                with urllib.request.urlopen(BASE+source,timeout=25) as response: raw=response.read()
                break
            except (OSError, TimeoutError):
                if attempt==2:raise
                time.sleep(1+attempt)
        if not raw.startswith(b'\x89PNG\r\n\x1a\n'):raise ValueError(f'Invalid PNG: {source}')
        # Original bytes are removed even when conversion fails.
        with tempfile.NamedTemporaryFile(dir=root,suffix='.png') as tmp:
            tmp.write(raw);tmp.flush()
            subprocess.run(['cwebp','-quiet','-q','88',tmp.name,'-o',str(output)],check=True)
        return target
    with ThreadPoolExecutor(max_workers=3) as pool:
        for target in pool.map(prepare,sources.items()):print(target,flush=True)
    (root/'sources.json').write_text(json.dumps({'repository':'Mogara/QSanguosha-v2','revision':REVISION,'files':sources},ensure_ascii=False,indent=2))
    print(f'Prepared {len(sources)} WebP images.')
if __name__=='__main__':main()
