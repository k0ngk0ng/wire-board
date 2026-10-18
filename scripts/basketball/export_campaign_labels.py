"""Verify source pointers/glyph transcriptions and export campaign menu labels.
Unicode transcriptions are read from original glyphs, not inferred item effects.
Chinese labels are editorial translations; they do not assert official naming.
"""
from pathlib import Path
import argparse,hashlib,json
ROOT=Path(__file__).resolve().parents[2]
JP=['ピカピカのシャツ','ロケットシャツ','ブルースのシャツ','コウテツシャツ','2だんめのシャツ','たいようのシャツ','ヌメヌメのシャツ','ダンカーのシャツ','うもうのシャツ','まぼろしのシャツ','ピカピカのパンツ','ロケットパンツ','ばんのうパンツ','うもうのパンツ','まっはパンツ','Tバックパンツ','ブルースのパンツ','しわしわパンツ','オニのパンツ','しみつきぱんつ','まぼろしのパンツ','ピカピカシューズ','ロケットシューズ','エアシューズ','ブルースのくつ','テクノシューズ','いっぽんあしのゲタ','ナリキンシューズ','うもうのくつ','こまげた','ダンシングシューズ','まぼろしのくつ']
ZH=['闪亮球衣','火箭球衣','布鲁斯球衣','钢铁球衣','二层球衣','太阳球衣','滑溜球衣','扣将球衣','羽毛球衣','幻影球衣','闪亮短裤','火箭短裤','万能短裤','羽毛短裤','马赫短裤','丁字短裤','布鲁斯短裤','皱皱短裤','鬼纹短裤','斑渍短裤','幻影短裤','闪亮球鞋','火箭球鞋','气垫球鞋','布鲁斯球鞋','科技球鞋','独齿木屐','富豪球鞋','羽毛球鞋','驹形木屐','舞蹈球鞋','幻影球鞋']
def main():
 parser=argparse.ArgumentParser();parser.add_argument('--check',action='store_true');args=parser.parse_args()
 rom=(ROOT/'.local/nes-basket/basketball.nes').read_bytes();sha=hashlib.sha256(rom).hexdigest();assert sha=='0fa28f60abb8d2ccbbeee90e79e3fa160a5cf32feb90f47ebf1a89168cff569b'
 source=ROOT/'.local/basketball/rea/campaign-audit/verified-selection.json';audit=json.loads(source.read_text());assert audit['romSHA256']==sha
 fixed=rom[16+7*16384:16+8*16384];bank=rom[16+3*16384:16+4*16384]
 known={int(k):v for k,v in json.loads((ROOT/'games/basketball/selection/names.json').read_text())['characterGlyphTranscriptions'].items()};known.update({0:' ',3:'2',36:'T'})
 def transcribe(encoded,label):
  values=[]
  for value in encoded:
   if value==255:break
   values.append(value)
  while values and values[-1]==0:values.pop()
  assert len(values)==len(label),(values,label)
  for value,letter in zip(values,label):
   assert value not in known or known[value]==letter,(value,known.get(value),letter,label)
   known[value]=letter
  return values
 items=[]
 for row,jp,zh in zip(audit['equipment'],JP,ZH,strict=True):
  i=row['id'];ptr=int.from_bytes(fixed[0x2c8+i*2:0x2ca+i*2],'little');assert ptr==int(row['namePointer'],16)
  raw=list(fixed[ptr-0xc000:ptr-0xc000+9]);assert raw==row['encodedGlyphs']
  items.append(dict(id=i,slot=0 if i<10 else 1 if i<21 else 2,name=zh,originalName=jp,sourcePointer=row['namePointer'],encodedGlyphs=transcribe(raw,jp)))
 tactics=[]
 for row,zh in zip(audit['tactics'],['积极进攻','盯人防守','花式配合','自由发挥','严密防守'],strict=True):
  ptr=int(row['sourcePointer'].split(':')[1],16);raw=list(bank[ptr-0x8000:ptr-0x8000+9]);assert raw==row['encodedGlyphs']
  tactics.append(dict(profile=row['profile'],name=zh,originalName=row['originalName'],sourcePointer=row['sourcePointer'],encodedGlyphs=transcribe(raw,row['originalName'])))
 result=dict(schemaVersion=1,romSHA256=sha,sourceSHA256=hashlib.sha256(source.read_bytes()).hexdigest(),translationAuthority='editorial-not-official',scope='original menu names and equipment slots only; no inferred effects or acquisition claims',items=items,tactics=tactics)
 dest=ROOT/'games/basketball/selection/campaign-labels.json';text=json.dumps(result,ensure_ascii=False,indent=2)+'\n'
 if args.check:assert dest.read_text()==text
 else:dest.write_text(text)
 print(json.dumps(dict(status='passed',items=len(items),tactics=len(tactics),glyphs=len(known),sha256=hashlib.sha256(text.encode()).hexdigest())))
if __name__=='__main__':main()
