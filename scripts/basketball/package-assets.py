#!/usr/bin/env python3
"""Build a deterministic manifest-selected public payload. No upload or runtime edits."""
import argparse,gzip,hashlib,io,json,re,tarfile
from pathlib import Path,PurePosixPath

ROOT=Path(__file__).resolve().parents[2]
ROM_SHA='0fa28f60abb8d2ccbbeee90e79e3fa160a5cf32feb90f47ebf1a89168cff569b'
MIME={'.png':'image/png','.json':'application/json','.mp3':'audio/mpeg','.wav':'audio/wav'}
PRIVATE={'verification','limitations','license','tool','processing','captures','runs','source_scripts'}
DATA_JSON={'court-map.json','initial-ring.json','stream-columns.json'}
sha=lambda data:hashlib.sha256(data).hexdigest()
def encoded(value):return (json.dumps(value,ensure_ascii=False,sort_keys=True,separators=(',',':'))+'\n').encode()
def require(ok,message):
 if not ok:raise ValueError(message)
MUSIC76_TIMING=('sample_rate','loop_start_sample','loop_end_sample','loop_samples','decoded_samples')
def approved_music_file(value):
 if value=='music-076-intro-loop.mp3':return True
 match=re.fullmatch(r'music-(\d{3})(?:-loop)?\.mp3',value) if isinstance(value,str) else None
 return match is not None and int(match[1])!=76
def music76_timing(entry):
 values={key:entry.get(key) for key in MUSIC76_TIMING}
 require(all(type(v) is int and 0<v<=9007199254740991 for v in values.values()),'music76 requires positive integer sample metadata')
 require(values['sample_rate']==44100,'music76 sample rate differs')
 require(values['loop_start_sample']<values['loop_end_sample']==values['decoded_samples'],'music76 decoded/end sample range differs')
 require(values['loop_samples']==values['loop_end_sample']-values['loop_start_sample'],'music76 period sample count differs')
 return values
def safe_relative(value):
 require(isinstance(value,str) and bool(value),'empty/non-string asset path')
 require(re.fullmatch(r'[A-Za-z0-9_.\-/]+',value) is not None,'unsafe asset path')
 require(not value.startswith('/') and all(p not in ('','.','..') for p in value.split('/')),'unsafe asset path')
 return PurePosixPath(value)
def read_under(base,relative):
 rel=safe_relative(relative);base=base.resolve();p=base
 for part in rel.parts:
  p=p/part;require(not p.is_symlink(),'nested asset symlink is not permitted')
 require(p.resolve().is_relative_to(base) and p.is_file(),'missing or escaped asset')
 return p.read_bytes()
def checked_source_base(root,base):
 require(base.is_relative_to(root),'source base outside declared inputs')
 current=root
 for part in base.relative_to(root).parts:
  current=current/part
  if current.is_symlink():
   shared=root/'.local/basketball'
   require(current==shared and root.parent.name=='.local' and current.resolve()==(root.parent.parent/'.local/basketball').resolve(),'unexpected source directory symlink')
 return base
def public_path(value):
 rel=safe_relative(value);parts=rel.parts;require(parts[:2]==('basketball','v1') and len(parts)>=4,'unexpected public namespace')
 suffix=rel.suffix
 if parts[2]=='graphics':require(len(parts)>=5 and parts[3] in ('court','courts','static-atlas','static-objects','raster','results','postresult') and suffix in ('.png','.json'),'unexpected graphics payload')
 elif parts[2]=='selection':require(suffix in ('.png','.json'),'unexpected selection payload')
 elif parts[2]=='audio':
  require(len(parts)==4 and (re.fullmatch(r'sound-\d{3}\.wav',parts[-1]) or approved_music_file(parts[-1]) or parts[-1]=='audio-playback.json'),'unexpected audio payload')
 else:raise ValueError('unexpected public asset group')
 if suffix=='.json':require(rel.name in DATA_JSON|{'manifest.json','atlas.json','audio-playback.json'},'research JSON is not public payload')
 return str(rel)
def inside_output(root,path,new=False):
 root=root.resolve();p=Path(path)
 if not p.is_absolute():p=root/p
 require(p.is_relative_to(root) and p!=root,'output must be inside worktree')
 current=root
 for part in p.relative_to(root).parts:
  require(part not in ('..','.'),'unsafe output path');current=current/part
  require(not current.is_symlink(),'output symlink is not permitted')
 require(p.resolve().is_relative_to(root),'output escapes worktree')
 if new:require(not p.exists(),'output already exists; choose a new output or use --verify')
 return p
def walk(value):
 if isinstance(value,dict):
  yield value
  for child in value.values():yield from walk(child)
 elif isinstance(value,list):
  for child in value:yield from walk(child)
def check_identity(value,required=False):
 found=[]
 for node in walk(value):
  for k,v in node.items():
   if k in ('rom_sha256','source_rom_sha256','romSHA256'):
    require(v==ROM_SHA,'mixed or unexpected source ROM identity');found.append(v)
 require(not required or bool(found),'manifest has no source ROM identity')
def sanitize(value):
 if isinstance(value,list):return [sanitize(v) for v in value]
 if not isinstance(value,dict):return value
 out={}
 for k,v in value.items():
  if k=='source':
   if isinstance(v,dict) and 'rom_sha256' in v:out[k]={'rom_sha256':v['rom_sha256']}
  elif k in PRIVATE or k.lower().startswith('source') or k in ('listening_accepted','accepted_for_release'):continue
  elif k=='assets' and isinstance(v,dict):out[k]={name:sanitize(record) for name,record in v.items() if name!='pixel-verification.json'}
  else:out[k]=sanitize(v)
 return out
class Collector:
 def __init__(self,root):self.root=root.resolve();self.files={};self.provenance={}
 def document(self,base,name,identity=False):
  checked_source_base(self.root,base);raw=read_under(base,name);doc=json.loads(raw);check_identity(doc,identity)
  logical=str(base.relative_to(self.root)/name)
  self.provenance[logical]={'input':logical,'sha256':sha(raw),'metadata':{k:v for k,v in doc.items() if k in PRIVATE or k.startswith('source') or k in ('accepted_for_release','listening_accepted','rom_sha256')}}
  return doc,raw
 def add(self,url,data):
  path=public_path(url);require(Path(path).suffix in MIME,'forbidden public file type')
  if path.endswith('.wav'):require(re.fullmatch(r'basketball/v1/audio/sound-\d{3}\.wav',path) is not None,'music WAV/master forbidden')
  if path in self.files:require(self.files[path]==data,'duplicate path differs')
  self.files[path]=data
 def manifest(self,base,name,mount,expected=None):
  doc,raw=self.document(base,name,True)
  if expected:require(sha(raw)==expected,'nested manifest hash differs')
  for node in walk(doc):
   for field in ('spriteAtlas','sourceImage','sourceManifest'):
    if field in node and field+'SHA256' in node:
     ref=safe_relative(node[field]);refbase=self.root/str(ref.parent);checked_source_base(self.root,refbase)
     require(sha(read_under(refbase,ref.name))==node[field+'SHA256'],'asset provenance reference differs')
  public=sanitize(doc);assets={};refs=[]
  for node in walk(public):
   entries=node.get('assets')
   if isinstance(entries,dict):
    for f,v in entries.items():assets[f]=v.get('sha256');refs.append(f)
   if 'file' in node:
    f=node['file'];refs.append(f)
    if 'sha256' in node:assets[f]=node['sha256']
   if isinstance(node.get('preview'),str):refs.append(node['preview'])
  for f in sorted(set(refs)):
   rel=safe_relative(f);require(rel.suffix=='.png' or (rel.suffix=='.json' and rel.name in DATA_JSON),'non-runtime manifest asset')
   require(re.fullmatch('[0-9a-f]{64}',str(assets.get(f,''))) is not None,'asset reference lacks hash')
   data=read_under(base,f);require(sha(data)==assets[f],'asset hash differs: '+f)
   if rel.suffix=='.png':require(data.startswith(b'\x89PNG\r\n\x1a\n'),'PNG signature differs')
   else:json.loads(data)
   self.add(mount+'/'+f,data)
  for node in walk(public):
   if 'manifest' in node:
    f=node['manifest'];rel=safe_relative(f);require(rel.suffix=='.json','nested manifest is not JSON')
    digest=node.get('sha256');require(re.fullmatch('[0-9a-f]{64}',str(digest)) is not None,'nested manifest lacks hash')
    child=self.manifest(base/str(rel.parent),rel.name,mount+'/'+str(rel.parent),digest)
    node['sha256']=sha(child)
  result=encoded(public);self.add(mount+'/'+name,result);return result
 def audio(self):
  assets=self.root/'games/basketball/assets';base=self.root/'.local/basketball/audio/export'
  catalog,catalog_raw=self.document(assets,'audio-catalog.json',True);playback,_=self.document(assets,'audio-playback.json')
  source,_=self.document(base,'manifest.json',True)
  if 'catalog_sha256' in source:require(source['catalog_sha256']==sha(catalog_raw),'audio source catalog hash differs')
  known={e['id']:e for e in source['sounds']};effects={}
  for entry in catalog['effects']:
   original=known.get(entry['id']);require(original is not None and original['wav']==entry['wav'],'catalog effect lacks original hash')
   f=entry['wav'];require(isinstance(entry['id'],int) and 0<=entry['id']<256 and f==f'sound-{entry["id"]:03d}.wav','effect ID/file mismatch');effects[str(entry['id'])]={'wav':f,'sha256':original['sha256'][f]}
  for key,entry in playback.get('effects',{}).items():
   require(key.isdecimal() and str(int(key))==key and 0<=int(key)<256 and entry['wav']==f'sound-{int(key):03d}.wav','effect override ID/file mismatch')
   source_doc,_=self.document(base,entry['source_manifest']);check_identity(source_doc,True)
   require(any(n.get('id')==int(key) and n.get('wav')==entry['wav'] and n.get('sha256')==entry['sha256'] for n in walk(source_doc)),'effect source reference differs')
   effects[key]={'wav':entry['wav'],'sha256':entry['sha256']}
  music={}
  for key,entry in playback.get('music',{}).items():
   f=entry.get('file');require(entry.get('mime_type')=='audio/mpeg' and approved_music_file(f),'runtime music requires explicit MP3 rendition')
   require(key.isdecimal() and str(int(key))==key and 0<=int(key)<256 and f in (("music-076-intro-loop.mp3",) if key=='76' else (f'music-{int(key):03d}.mp3',f'music-{int(key):03d}-loop.mp3')),'music ID/file mismatch')
   digest=entry.get('distribution_sha256');require(re.fullmatch('[0-9a-f]{64}',str(digest)) is not None,'music lacks delivery hash')
   source_doc,_=self.document(base,entry['source_manifest']);check_identity(source_doc)
   delivery,_=self.document(base,entry['encoding_manifest'])
   require(any(n.get('id')==int(key) and n.get('file')==f and n.get('sha256')==digest for n in walk(delivery)),'MP3 encoding reference differs')
   timing={}
   if key=='76':
    require(entry.get('loop') is True,'music76 requires suffix looping')
    timing=music76_timing(entry)
    for doc in (source_doc,delivery):
     check_identity(doc,True)
     require(doc.get('status')=='offline-asset-verified' and doc.get('id')==76,'music76 source is not verified delivery')
     require(music76_timing(doc)==timing,'music76 source timing differs')
     require(doc.get('file')==f and doc.get('sha256')==digest,'music76 source delivery hash differs')
     require(doc.get('master')==entry.get('wav')=='music-076-intro-loop.wav' and doc.get('master_sha256')==entry.get('sha256'),'music76 source master differs')
    require(sha(read_under(base,entry['wav']))==entry['sha256'],'music76 master bytes differ')
   data=read_under(base,f);require(sha(data)==digest,'MP3 bytes differ')
   require(data.startswith(b'ID3') or (len(data)>1 and data[0]==255 and data[1]&224==224),'MP3 signature differs')
   self.add('basketball/v1/audio/'+f,data)
   music[key]={'file':f,'mime_type':'audio/mpeg','distribution_sha256':digest,'loop':entry.get('loop') is True,**timing}
  for entry in effects.values():
   f=entry['wav'];require(re.fullmatch(r'sound-\d{3}\.wav',f) is not None,'unapproved effect name')
   data=read_under(base,f);require(sha(data)==entry['sha256'],'effect bytes differ')
   require(data[:4]==b'RIFF' and data[8:12]==b'WAVE','effect WAV signature differs')
   self.add('basketball/v1/audio/'+f,data)
  self.add('basketball/v1/audio/audio-playback.json',encoded({'schema':1,'source':{'rom_sha256':ROM_SHA},'music':music,'effects':effects}))
def collect(root):
 c=Collector(root)
 for group in ['court','courts','static-atlas','static-objects','raster','results']:
  c.manifest(root/'.local/basketball'/group,'atlas.json' if group.startswith('static-') else 'manifest.json','basketball/v1/graphics/'+group)
 c.manifest(root/'.local/postresult-assets','manifest.json','basketball/v1/graphics/postresult')
 c.manifest(root/'.local/basketball/selection-assets','manifest.json','basketball/v1/selection')
 c.audio();return c
def archive(files):
 raw=io.BytesIO()
 with tarfile.open(fileobj=raw,mode='w',format=tarfile.USTAR_FORMAT) as tar:
  for path,data in sorted(files.items()):
   info=tarfile.TarInfo(path);info.size=len(data);info.mode=0o644;info.mtime=0;info.uid=info.gid=0;info.uname=info.gname='';tar.addfile(info,io.BytesIO(data))
 return gzip.compress(raw.getvalue(),compresslevel=9,mtime=0)
def build_package(root,output):
 root=Path(root).resolve();output=inside_output(root,output,True);c=collect(root)
 entries=[{'urlPath':'/'+p,'output':'public/'+p,'sha256':sha(b),'bytes':len(b),'mime':MIME[Path(p).suffix]} for p,b in sorted(c.files.items())]
 identity=sha(encoded({'sourceROM':ROM_SHA,'entries':entries}));prefix='bga/basketball/v1-'+identity
 provenance=encoded({'sourceROM':ROM_SHA,'inputs':sorted(c.provenance.values(),key=lambda v:v['input']),'scope':'source records retained separately; package integrity is not release/listening acceptance'})
 packed=archive(c.files)
 manifest={'schema':1,'game':'basketball','sourceROM':ROM_SHA,'contentSHA256':identity,'ossBucket':'stoneage-assets','ossPrefix':prefix,'assetsBaseURL':'https://cdn.ichenj.com/'+prefix,'entries':entries,'archiveSHA256':sha(packed),'provenanceSHA256':sha(provenance),'acceptanceInherited':False}
 output.mkdir(parents=True)
 for p,b in c.files.items():target=output/'public'/p;target.parent.mkdir(parents=True,exist_ok=True);target.write_bytes(b)
 (output/'provenance.json').write_bytes(provenance);(output/'asset-package.tar.gz').write_bytes(packed);(output/'package-manifest.json').write_bytes(encoded(manifest))
 verify_package(output,root);return manifest
def verify_package(package,root=ROOT):
 p=inside_output(Path(root),package);m=json.loads(read_under(p,'package-manifest.json'));require(m.get('sourceROM')==ROM_SHA and m.get('schema')==1 and m.get('game')=='basketball','package identity differs')
 entries=m['entries'];require(entries==sorted(entries,key=lambda e:e['urlPath']),'package order differs');files={}
 for entry in entries:
  url=entry['urlPath'];require(url.startswith('/basketball/v1/'),'unexpected public URL namespace');rel=public_path(url[1:]);require(entry['output']=='public/'+rel,'local fallback path differs')
  require(rel not in files,'duplicate package path');data=read_under(p,entry['output']);require(sha(data)==entry['sha256'] and len(data)==entry['bytes'],'packaged bytes differ')
  require(MIME.get(Path(rel).suffix)==entry['mime'],'packaged MIME differs')
  if rel.endswith('.wav'):require(re.fullmatch(r'basketball/v1/audio/sound-\d{3}\.wav',rel) is not None,'music master in package')
  files[rel]=data
 actual={str(f.relative_to(p/'public')) for f in (p/'public').rglob('*') if f.is_file() or f.is_symlink()};require(actual==set(files),'unlisted public payload')
 identity=sha(encoded({'sourceROM':ROM_SHA,'entries':entries}));require(identity==m['contentSHA256'],'content identity differs')
 prefix='bga/basketball/v1-'+identity;require(m['ossBucket']=='stoneage-assets' and m['ossPrefix']==prefix and m['assetsBaseURL']=='https://cdn.ichenj.com/'+prefix,'OSS target differs')
 require(sha(read_under(p,'provenance.json'))==m['provenanceSHA256'],'provenance differs')
 require(sha(read_under(p,'asset-package.tar.gz'))==m['archiveSHA256'] and sha(archive(files))==m['archiveSHA256'],'archive differs')
 return m
def main():
 ap=argparse.ArgumentParser(description=__doc__);group=ap.add_mutually_exclusive_group(required=True);group.add_argument('--output',type=Path);group.add_argument('--verify',type=Path);args=ap.parse_args()
 try:
  m=build_package(ROOT,args.output) if args.output else verify_package(args.verify)
  print(json.dumps({'status':'verified','assets':len(m['entries']),'bytes':sum(e['bytes'] for e in m['entries']),'contentSHA256':m['contentSHA256'],'assetsBaseURL':m['assetsBaseURL'],'uploaded':False}))
 except (ValueError,OSError,KeyError,json.JSONDecodeError) as exc:ap.exit(1,'Asset package failed: '+str(exc)+'\n')
if __name__=='__main__':main()
