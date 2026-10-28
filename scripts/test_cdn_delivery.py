"""Pre-implementation safety failures: incomplete/hash/mime/CORS mismatches,
symlinks and traversal in uploads; trim before verification; secret publication.

Contra failures declared before implementation: absent/mutated receipt, extra
or missing payloads, wrong identity/hash/size/MIME, path aliases and symlinks,
WAV incorrectly compressed or marked nonseekable, Range ignored (200), wrong
Content-Range/body/MIME/CORS/encoding, and trim using another receipt's digest.
All local HTTP probes are loopback-only; no upload or credentials are used.
"""
import unittest,tempfile,json,hashlib,gzip,shutil,threading
from http.server import BaseHTTPRequestHandler, HTTPServer
from pathlib import Path
import cdn_delivery as cdn
class Delivery(unittest.TestCase):
 def setUp(self):
  self.temp=tempfile.TemporaryDirectory();self.addCleanup(self.temp.cleanup);self.dist=Path(self.temp.name);(self.dist/'assets').mkdir();(self.dist/'assets/index-abcdefgh.js').write_bytes(b'export const ready=true;');(self.dist/'index.html').write_text('entry')
  self.base='https://cdn.ichenj.com/bga/web/'+'a'*40+'/'
  self.entry={'url':self.base+'assets/index-abcdefgh.js','bytes':24,'sha256':hashlib.sha256(b'export const ready=true;').hexdigest(),'mime':'text/javascript'}
 def test_checks_remote_bytes_mime_and_cors(self):
  body=b'export const ready=true;';headers={'Content-Type':'text/javascript','Access-Control-Allow-Origin':'*'}
  cdn.validate_response(self.entry,body,headers)
  for b,h in [(body+b'!',headers),(body,{'Content-Type':'text/html','Access-Control-Allow-Origin':'*'}),(body,{'Content-Type':'text/javascript'})]:
   with self.assertRaises(ValueError):cdn.validate_response(self.entry,b,h)
 def test_precompressed_scripts_are_verified_after_decoding(self):
  body=b'export const ready=true;';headers={'Content-Type':'text/javascript','Content-Encoding':'gzip','Access-Control-Allow-Origin':'*'}
  cdn.validate_response(self.entry,gzip.compress(body,mtime=0),headers)
  with self.assertRaises(ValueError):cdn.validate_response(self.entry,b'broken gzip',headers)
 def test_inventory_is_bound_to_files_and_rejects_symlinks(self):
  rows=cdn.inventory(self.dist,self.base);self.assertEqual(rows[0]['url'],self.entry['url']);self.assertEqual(rows[0]['sha256'],self.entry['sha256'])
  (self.dist/'assets/leak.js').symlink_to(self.dist/'index.html')
  with self.assertRaises(ValueError):cdn.inventory(self.dist,self.base)
 def test_trim_requires_exact_verified_inventory(self):
  rows=cdn.inventory(self.dist,self.base)
  with self.assertRaises(ValueError):cdn.trim(self.dist,{'passed':False,'files':rows})
  altered=[dict(rows[0],sha256='0'*64)]
  with self.assertRaises(ValueError):cdn.trim(self.dist,{'passed':True,'files':altered})
  cdn.trim(self.dist,{'passed':True,'files':rows});self.assertFalse((self.dist/'assets').exists());self.assertTrue((self.dist/'index.html').exists())

class ContraDelivery(unittest.TestCase):
 setUp=Delivery.setUp

 def runtime(self, dist=None):
  dist=dist or self.dist
  rules='c'*64
  manifest=json.dumps({'version':1,'rulesBuildID':rules},sort_keys=True).encode()
  runtime=cdn.sha(manifest)
  prefix='contra/runtime-'+runtime+'/'
  payloads={'manifest.json':(manifest,'application/json',False),
            'audio/tone.wav':(b'RIFF'+bytes(range(128)),'audio/wav',True),
            'graphics/page.idx':(b'indexed pixels','application/octet-stream',False),
            'graphics/colors.rgb':(b'palette','application/octet-stream',False)}
  files={}
  for name,(body,mime,seek) in payloads.items():
   path=dist/prefix/name;path.parent.mkdir(parents=True,exist_ok=True);path.write_bytes(body)
   files[name]={'bytes':len(body),'sha256':cdn.sha(body),'mime':mime,'requiresRange':seek}
  receipt={'prefix':prefix,'runtimeBuildID':runtime,'rulesBuildID':rules,'files':files,'uploaded':False}
  (dist/'contra-delivery.json').write_text(json.dumps(receipt))
  return receipt

 def test_inventory_includes_exact_runtime_payloads_without_publishing_receipt(self):
  receipt=self.runtime();rows=cdn.inventory(self.dist,self.base)
  runtime={r['path']:r for r in rows if r['path'].startswith('contra/')}
  self.assertEqual(set(runtime),{receipt['prefix']+name for name in receipt['files']})
  self.assertNotIn('contra-delivery.json',{r['path'] for r in rows})
  wave=runtime[receipt['prefix']+'audio/tone.wav']
  self.assertEqual(wave['mime'],'audio/wav');self.assertEqual(wave['encoding'],'')
  self.assertTrue(wave['requiresRange']);self.assertEqual(wave['wireBytes'],wave['bytes'])

 def test_inventory_rejects_invalid_runtime_receipts_and_payloads(self):
  cases=['missing-receipt','missing-root','extra','missing','hash','size','mime','range',
         'prefix','traversal','absolute','alias','symlink-file','symlink-directory',
         'symlink-root','symlink-receipt','wrong-runtime-id','wrong-rules-id']
  for case in cases:
   with self.subTest(case=case),tempfile.TemporaryDirectory(dir=self.dist) as name:
    dist=Path(name);(dist/'assets').mkdir();(dist/'assets/app.js').write_bytes(b'app')
    receipt=self.runtime(dist);receipt_path=dist/'contra-delivery.json'
    wave=dist/receipt['prefix']/'audio/tone.wav'
    if case=='missing-receipt':receipt_path.unlink()
    elif case=='missing-root':shutil.rmtree(dist/'contra')
    elif case=='extra':(dist/'contra/extra.json').write_text('{}')
    elif case=='missing':wave.unlink()
    elif case=='hash':wave.write_bytes(b'bad')
    elif case=='size':receipt['files']['audio/tone.wav']['bytes']+=1
    elif case=='mime':receipt['files']['audio/tone.wav']['mime']='text/html'
    elif case=='range':receipt['files']['audio/tone.wav']['requiresRange']=False
    elif case=='prefix':receipt['prefix']='contra/runtime-other/'
    elif case in ('traversal','absolute','alias'):
     target={'traversal':'../escape.wav','absolute':'/audio/tone.wav','alias':'audio//tone.wav'}[case]
     receipt['files'][target]=receipt['files'].pop('audio/tone.wav')
    elif case=='symlink-file':wave.unlink();wave.symlink_to(self.dist/'index.html')
    elif case=='symlink-directory':(dist/receipt['prefix']/'alias').symlink_to(dist/'assets',target_is_directory=True)
    elif case=='symlink-root':shutil.rmtree(dist/'contra');(dist/'contra').symlink_to(dist/'assets',target_is_directory=True)
    elif case=='symlink-receipt':receipt_path.unlink();receipt_path.symlink_to(self.dist/'index.html')
    elif case=='wrong-runtime-id':receipt['runtimeBuildID']='d'*64
    elif case=='wrong-rules-id':receipt['rulesBuildID']='d'*64
    if case not in ('missing-receipt','symlink-receipt'):receipt_path.write_text(json.dumps(receipt))
    with self.assertRaises(ValueError):cdn.inventory(dist,self.base)

 def test_trim_requires_same_receipt_and_preserves_server_metadata(self):
  receipt=self.runtime();path=self.dist/'contra-delivery.json';original=path.read_bytes()
  (self.dist/'game-load-manifest.json').write_text('{}')
  rows=cdn.inventory(self.dist,self.base)
  report={'passed':True,'files':rows}
  with self.assertRaises(ValueError):cdn.trim(self.dist,report)
  self.assertTrue((self.dist/'assets').exists());self.assertTrue((self.dist/receipt['prefix']).exists())
  report['contraDeliverySHA256']=cdn.sha(original)
  path.write_bytes(original+b'\n')
  with self.assertRaises(ValueError):cdn.trim(self.dist,report)
  self.assertTrue((self.dist/'assets').exists());self.assertTrue((self.dist/receipt['prefix']).exists())
  path.write_bytes(original);cdn.trim(self.dist,report)
  self.assertFalse((self.dist/'assets').exists());self.assertFalse((self.dist/receipt['prefix']).exists())
  for name in ['index.html','game-load-manifest.json','contra-delivery.json']:self.assertTrue((self.dist/name).exists())

class WaveRangeHTTP(unittest.TestCase):
 def test_real_range_response_and_failure_boundaries(self):
  payload=b'RIFF'+bytes(range(128));state={'mode':'good','requests':[]}
  class Handler(BaseHTTPRequestHandler):
   def log_message(self,*args):pass
   def do_GET(self):
    requested=self.headers.get('Range')
    state['requests'].append({'range':requested,'origin':self.headers.get('Origin'),'encoding':self.headers.get('Accept-Encoding')})
    body=payload;status=200;mime='audio/wav';cors='*';content_range=None;encoding=None
    if requested:
     body=payload[:64];status=206;content_range=f'bytes 0-63/{len(payload)}'
     if state['mode']=='status':status=200
     elif state['mode']=='range':content_range=f'bytes 1-64/{len(payload)}'
     elif state['mode']=='body':body=bytes([body[0]^1])+body[1:]
     elif state['mode']=='mime':mime='text/html'
     elif state['mode']=='cors':cors=None
     elif state['mode']=='encoding':encoding='gzip'
    elif state['mode']=='full-body':body+=b'wrong'
    self.send_response(status);self.send_header('Content-Type',mime);self.send_header('Content-Length',str(len(body)))
    if cors:self.send_header('Access-Control-Allow-Origin',cors)
    if content_range:self.send_header('Content-Range',content_range)
    if encoding:self.send_header('Content-Encoding',encoding)
    self.end_headers();self.wfile.write(body)
  server=HTTPServer(('127.0.0.1',0),Handler);thread=threading.Thread(target=server.serve_forever,daemon=True);thread.start()
  try:
   row={'url':f'http://127.0.0.1:{server.server_port}/tone.wav','bytes':len(payload),'sha256':cdn.sha(payload),'mime':'audio/wav','encoding':'','requiresRange':True}
   self.assertEqual(cdn.verify(row),row)
   self.assertEqual([r['range'] for r in state['requests']],[None,'bytes=0-63'])
   self.assertEqual(state['requests'][-1]['encoding'],'identity');self.assertEqual(state['requests'][-1]['origin'],'https://bga.ichenj.com')
   for mode in ['status','range','body','mime','cors','encoding','full-body']:
    with self.subTest(mode=mode):
     state.update(mode=mode,requests=[])
     with self.assertRaises(ValueError):cdn.verify(row)
     if mode=='full-body':self.assertEqual(len(state['requests']),1,'full checksum must pass before range probe')
  finally:server.shutdown();server.server_close();thread.join(timeout=2)

# Network failures declared before the retry implementation: transient request
# errors abort all assets; ambiguous PUT is repeated over an immutable object;
# permanent authorization/checksum errors retry or are accepted; retries never
# terminate; a failed Range probe skips the full content contract on retry.
class NetworkRecovery(unittest.TestCase):
 def test_transient_failures_are_bounded_and_permanent_errors_fail_closed(self):
  from unittest.mock import Mock,patch
  from urllib.error import URLError,HTTPError
  for error in [URLError(TimeoutError()),TimeoutError(),HTTPError('url',503,'busy',{},None)]:
   action=Mock(side_effect=[error,'ok'])
   with patch.object(cdn.time,'sleep') as sleep:
    self.assertEqual(cdn.retry_request(action,'asset'),'ok');self.assertEqual(action.call_count,2);sleep.assert_called_once_with(1)
  for error in [ValueError('hash mismatch'),HTTPError('url',403,'forbidden',{},None),HTTPError('url',404,'missing',{},None)]:
   action=Mock(side_effect=error)
   with patch.object(cdn.time,'sleep') as sleep:
    with self.assertRaises(type(error)):cdn.retry_request(action,'asset')
    self.assertEqual(action.call_count,1);sleep.assert_not_called()
  action=Mock(side_effect=URLError(TimeoutError()))
  with patch.object(cdn.time,'sleep') as sleep:
   with self.assertRaises(URLError):cdn.retry_request(action,'asset')
   self.assertEqual(action.call_count,4);self.assertEqual([c.args[0] for c in sleep.call_args_list],[1,2,4])
 def test_ambiguous_upload_rechecks_object_without_overwriting(self):
  import io
  from unittest.mock import patch
  from urllib.error import HTTPError,URLError
  body=b'asset';response=io.BytesIO(body);response.headers={}
  row={'url':'https://cdn.ichenj.com/bga/asset','path':'asset','encoding':'','sha256':cdn.sha(body),'mime':'application/octet-stream'}
  with tempfile.TemporaryDirectory() as temp:
   dist=Path(temp);(dist/'asset').write_bytes(body)
   with patch.object(cdn,'credentials',return_value=('test','test')),patch.object(cdn.time,'sleep'),patch.object(cdn,'urlopen',side_effect=[HTTPError('url',404,'missing',{},None),URLError(TimeoutError()),response]) as request:
    cdn.publish(dist,[row],'bucket','example.invalid')
    self.assertEqual([c.args[0].get_method() for c in request.call_args_list],['GET','PUT','GET'])
    self.assertEqual(request.call_args_list[1].args[0].get_header('X-oss-forbid-overwrite'),'true')
 def test_range_network_retry_still_checks_full_body_and_range(self):
  import io
  from unittest.mock import patch
  from urllib.error import URLError
  body=b'x'*100
  def response(data,partial=False):
   r=io.BytesIO(data);r.headers={'Content-Type':'audio/wav','Access-Control-Allow-Origin':'*'};r.status=206 if partial else 200
   if partial:r.headers['Content-Range']='bytes 0-63/100'
   return r
  row={'url':'https://example.invalid/tone.wav','bytes':100,'sha256':cdn.sha(body),'mime':'audio/wav','encoding':'','requiresRange':True}
  with patch.object(cdn.time,'sleep'),patch.object(cdn,'urlopen',side_effect=[response(body),URLError(TimeoutError()),response(body),response(body[:64],True)]) as request:
   self.assertEqual(cdn.verify(row),row);self.assertEqual(request.call_count,4)

# Before connection reuse: each full/Range read performs another TLS handshake;
# unread error bodies poison later responses; retries must not skip verification.
class TLSReuse(unittest.TestCase):
 def test_full_and_range_reads_reuse_a_verified_tls_connection(self):
  self.exercise(False)
 def test_transient_error_body_is_drained_before_reuse(self):
  self.exercise(True)
 def exercise(self,fail_once):
  import ssl,subprocess,os
  from unittest.mock import patch
  from http.server import ThreadingHTTPServer
  payload=b'x'*100;state={'connections':0,'failed':False,'requests':0}
  class Handler(BaseHTTPRequestHandler):
   protocol_version='HTTP/1.1'
   def log_message(self,*args):pass
   def do_GET(self):
    state['requests']+=1;status=200;body=payload
    if fail_once and not state['failed']:state['failed']=True;status=503;body=b'retry'
    elif self.headers.get('Range'):status=206;body=payload[:64]
    self.send_response(status);self.send_header('Content-Type','audio/wav');self.send_header('Access-Control-Allow-Origin','*');self.send_header('Content-Length',str(len(body)))
    if status==206:self.send_header('Content-Range','bytes 0-63/100')
    self.end_headers();self.wfile.write(body)
  class Server(ThreadingHTTPServer):
   daemon_threads=True
   def get_request(self):
    state['connections']+=1
    return super().get_request()
  with tempfile.TemporaryDirectory() as temp:
   cert=Path(temp)/'cert.pem';key=Path(temp)/'key.pem'
   subprocess.run(['openssl','req','-x509','-newkey','rsa:2048','-nodes','-days','1','-subj','/CN=localhost','-addext','subjectAltName=IP:127.0.0.1','-keyout',str(key),'-out',str(cert)],check=True,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
   server=Server(('127.0.0.1',0),Handler)
   context=ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER);context.load_cert_chain(cert,key);server.socket=context.wrap_socket(server.socket,server_side=True)
   trusted=ssl.create_default_context(cafile=str(cert))
   thread=threading.Thread(target=server.serve_forever,daemon=True);thread.start()
   row={'url':f'https://127.0.0.1:{server.server_port}/tone.wav','bytes':100,'sha256':cdn.sha(payload),'mime':'audio/wav','encoding':'','requiresRange':True}
   try:
    with patch('ssl._create_default_https_context',return_value=trusted),patch('urllib.request._opener',None),patch.dict(os.environ,{'NO_PROXY':'127.0.0.1,localhost'}),patch.object(cdn.time,'sleep'):
     for _ in range(3):self.assertEqual(cdn.verify(row),row)
    self.assertEqual(state['connections'],1,'full and range requests must not repeatedly negotiate TLS')
    self.assertEqual(state['requests'],7 if fail_once else 6)
   finally:
    if hasattr(cdn,'close_connections'):cdn.close_connections()
    server.shutdown();server.server_close();thread.join(timeout=2)

if __name__=='__main__':unittest.main()
