#!/usr/bin/env python3
"""Prepare audited public-domain/CC0 decks without extracting archive paths.
Run at repository root; only named members are read into owned destinations.
"""
import hashlib,json,zipfile,tarfile,shutil
from pathlib import Path
root=Path(__file__).resolve().parents[2];research=root/'.local/poker-research/sources';source=root/'assets/poker/source';public=root/'web/public/poker';manifest=[]
(source/'licenses').mkdir(parents=True,exist_ok=True);(public/'licenses').mkdir(parents=True,exist_ok=True)
def write(skin,id,member,data,pack,url,license):
 ext=Path(member).suffix;p=source/skin/(str(id)+ext);p.parent.mkdir(parents=True,exist_ok=True);p.write_bytes(data)
 manifest.append(dict(skin=skin,id=id,source=p.relative_to(root).as_posix(),member=member,archive=pack,sourceURL=url,license=license,sourceSHA256=hashlib.sha256(data).hexdigest()))
def license(name,data):
 (source/'licenses'/name).write_bytes(data);(public/'licenses'/name).write_bytes(data)
ranks=['2','3','4','5','6','7','8','9','10','J','Q','K','A']
with zipfile.ZipFile(research/'byron.zip') as z:
 for id in range(54):
  name=ranks[id%13]+'CDHS'[id//13] if id<52 else 'Joker'+str(id-51)
  member='Vector-Playing-Cards-master/cards-svg/'+name+'.svg'
  write('english',id,member,z.read(member),'byron.zip','https://github.com/notpeter/Vector-Playing-Cards','Public domain / WTFPL')
 license('english-README.md',z.read('Vector-Playing-Cards-master/README.md'))
with tarfile.open(research/'nicu.tgz') as t:
 for id in range(54):
  name='white_'+'cdhs'[id//13]+'_'+ranks[id%13].lower() if id<52 else 'white_jk_'+('b' if id==52 else 'r')
  member='../svg/white/'+name+'.svg'
  write('illustrated',id,member,t.extractfile(member).read(),'nicu.tgz','https://nicubunu.ro/graphics:playingcards','Public domain / CC0')
 license('illustrated-LICENSE.txt',t.extractfile('../svg/LICENSE.txt').read());license('illustrated-README.txt',t.extractfile('../svg/README.txt').read())
with zipfile.ZipFile(research/'jumbo.zip') as z:
 for id in range(54):
  names=['2','3','4','5','6','7','8','9','10','Jack','Queen','King','Ace']
  name=['club','diamond','heart','spade'][id//13]+names[id%13] if id<52 else ('blackJoker' if id==52 else 'redJoker')
  vector=id>=52 or id%13>=9
  member='Decks/Vertical4/'+('svgs/' if vector else 'pngs/')+name+('.svg' if vector else '.png')
  write('fourcolor',id,member,z.read(member),'jumbo.zip','https://opengameart.org/content/jumbo-index-playing-cards','CC0-1.0')
 (source/'licenses'/'fourcolor-source.html').write_bytes((research/'oga-jumbo-index-playing-cards.html').read_bytes())
 license('fourcolor-source.txt',(research/'oga-jumbo-index-playing-cards.txt').read_bytes())
with zipfile.ZipFile(research/'kenney-casino-audio.zip') as z:
 directory=public/'v1/audio';directory.mkdir(parents=True,exist_ok=True)
 for name in ['card-shuffle','card-slide-1','card-place-1','card-fan-1']:
  data=z.read('Audio/'+name+'.ogg');(directory/(name+'.ogg')).write_bytes(data)
  manifest.append(dict(type='sound',name=name,sourceURL='https://kenney.nl/assets/casino-audio',license='CC0-1.0',sourceSHA256=hashlib.sha256(data).hexdigest(),output=(directory/(name+'.ogg')).relative_to(root).as_posix()))
 license('audio-License.txt',z.read('License.txt'))
# Original backs share card dimensions. No third-party brand or characters.
for name,color,accent in [('pine','#315e50','#d4bf84'),('claret','#813b45','#ead3a0'),('ink','#334f76','#d7d9c2')]:
 svg=f'''<svg xmlns="http://www.w3.org/2000/svg" width="240" height="336" viewBox="0 0 240 336"><defs><pattern id="p" width="24" height="24" patternUnits="userSpaceOnUse"><path d="M12 1L23 12 12 23 1 12Z" fill="none" stroke="{accent}" stroke-width=".8"/><circle cx="12" cy="12" r="2" fill="{accent}"/></pattern></defs><rect x="1" y="1" width="238" height="334" rx="14" fill="#fffaf0" stroke="#c5bca8"/><rect x="10" y="10" width="220" height="316" rx="10" fill="{color}"/><rect x="17" y="17" width="206" height="302" rx="6" fill="url(#p)"/><path d="M120 93L170 168 120 243 70 168Z" fill="{color}" stroke="{accent}" stroke-width="3"/><path d="M120 123L149 168 120 213 91 168Z" fill="none" stroke="{accent}" stroke-width="2"/><circle cx="120" cy="168" r="12" fill="{accent}"/></svg>'''
 p=source/'backs'/(name+'.svg');p.parent.mkdir(parents=True,exist_ok=True);p.write_text(svg)
 manifest.append(dict(type='back',name=name,source=p.relative_to(root).as_posix(),author='wire-board contributors',license='Project original',sourceSHA256=hashlib.sha256(svg.encode()).hexdigest()))
(root/'assets/poker/source-manifest.json').write_text(json.dumps(manifest,ensure_ascii=False,indent=2)+'\n')
print('Prepared 162 faces, 3 original backs, 4 CC0 sounds with source records.')
