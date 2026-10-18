"""Before-parser exact source-pixel acceptance; no mismatch exclusions."""
import json,unittest
from pathlib import Path
from PIL import Image,ImageChops
ROOT=Path(__file__).resolve().parents[2]
OUT=ROOT/'.local/postresult-fetch'

class PostresultFetchTests(unittest.TestCase):
 def test_all_observed_pixels(self):
  from postresult_fetch import derive_frame,render_frame
  OUT.mkdir(exist_ok=True);results=[]
  for kind,frame in [('reward',540),('reward',660),('ending',600),('ending',1800),('ending',6000),('ending',10500),('ending',12000),('ending',13500)]:
   metadata=derive_frame(kind,frame);image=render_frame(metadata).convert('RGB')
   expected=Image.open(ROOT/f'.local/nes-basket/runs/postresult-{kind}-observed/frame-{frame:05d}.png').convert('RGB')
   differences=ImageChops.difference(image,expected);mismatches=[(x,y) for y in range(240) for x in range(256) if differences.getpixel((x,y))!=(0,0,0)]
   stem=f'{kind}-{frame:05d}';image.save(OUT/f'{stem}.png');differences.save(OUT/f'{stem}-diff.png');(OUT/f'{stem}.json').write_text(json.dumps(metadata,separators=(',',':'))+'\n')
   results.append(dict(kind=kind,frame=frame,pixels=61440,different=len(mismatches),bounds=differences.getbbox(),first=mismatches[:20]))
  (OUT/'result.json').write_text(json.dumps(results,indent=2)+'\n')
  self.assertEqual(sum(r['different'] for r in results),0,results)
if __name__=='__main__':unittest.main()
