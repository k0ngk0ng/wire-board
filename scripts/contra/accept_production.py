#!/usr/bin/env python3
"""Local native/Wasm binding acceptance; no production game registration.

Uses real Go builds and the embedded production constructor in an empty working
directory. Changed inputs and corrupted assets must fail before native compile.
"""
import argparse,hashlib,json,os,shutil,subprocess,sys,traceback
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2];sys.dont_write_bytecode=True;sys.path.insert(0,str(ROOT/'games/contra/build'))
from identity import go_env,verify_runtime_file,native_flags
def main():
 p=argparse.ArgumentParser();p.add_argument('--runtime',required=True);p.add_argument('--out',required=True);args=p.parse_args()
 out=(ROOT/args.out).resolve();assert out.is_relative_to(ROOT)and not out.exists();out.mkdir(parents=True);temp=out/'temporary';temp.mkdir();empty=temp/'empty';empty.mkdir()
 manifest_path=(ROOT/args.runtime).resolve();report={'passed':False,'scope':'Production initializer and local build binding only; no publication or complete bot campaign.','checks':[]}
 registry=(ROOT/'games/registry.go').read_bytes();main_source=(ROOT/'main.go').read_bytes()
 def run(command,cwd=ROOT,ok=True):
  env=go_env(ROOT)
  if command[0]=='go' and '-race' in command:env['CGO_ENABLED']='1'
  result=subprocess.run(command,cwd=cwd,env=env,text=True,capture_output=True,timeout=180)
  report['checks'].append({'command':command,'exitCode':result.returncode,'stdout':result.stdout,'stderr':result.stderr})
  assert (result.returncode==0)==ok,report['checks'][-1];return result
 try:
  manifest,identity,context=verify_runtime_file(ROOT,manifest_path);report['runtimeBuildID']=identity;report['rulesBuildID']=manifest['rulesBuildID'];report['visualsID']=manifest['visualsID']
  output=temp/'probe';run(['python3','-B','games/contra/build/native.py','--runtime',str(manifest_path),'--out',str(output),'--package','./games/contra/build/cmd/probe'])
  measured=json.loads(run([str(output)],empty).stdout);assert measured['rulesBuildID']==manifest['rulesBuildID']and measured['runtimeBuildID']==identity and measured['visualsID']==manifest['visualsID'];assert measured['frames']>=12
  report['offline']=measured;report['nativeBuild']=json.loads(output.with_suffix('.build.json').read_text())
  # The real composition test must execute with this artifact's identities;
  # ordinary unbound tests intentionally cannot validate successful startup.
  registered=run(['go','test','-race','-json','-ldflags='+native_flags(manifest['rulesBuildID'],identity),'./internal/production','-run','^TestContraProductionRegistration','-count=1'])
  events=[json.loads(line)for line in registered.stdout.splitlines()if line.startswith('{')]
  for name in ['TestContraProductionRegistrationRequiresIdentities','TestContraProductionRegistrationWithLinkedBuild']:
   actions={event.get('Action')for event in events if event.get('Test')==name}
   assert {'run','pass'}<=actions and not actions&{'skip','fail'},('production composition was not verified',name,actions)
  report['productionCompositionVerified']=True
  unbound=temp/'unbound';run(['go','build','-trimpath','-buildvcs=false','-o',str(unbound),'./games/contra/build/cmd/probe']);denied=run([str(unbound)],empty,False);assert 'requires linked'in denied.stderr
  for kind in ['rules','source','toolchain','visuals','corrupt-wasm','escape']:
   directory=temp/kind;directory.mkdir();changed=json.loads(manifest_path.read_bytes())
   if kind=='rules':changed['rulesBuildID']='0'*64
   elif kind=='source':changed['runtime']['sourceSHA256']='0'*64
   elif kind=='toolchain':changed['runtime']['goVersion']='go0.0.0'
   elif kind=='visuals':changed['visualsID']='0'*64
   elif kind=='escape':changed['wasm']['url']='../outside.wasm'
   else:
    item=changed['wasm'];destination=directory/item['url'].lstrip('/');destination.parent.mkdir(parents=True,exist_ok=True);destination.write_bytes(b'corrupt')
   candidate=directory/'manifest.json';candidate.write_text(json.dumps(changed))
   rejected=run(['python3','-B','games/contra/build/native.py','--runtime',str(candidate),'--out',str(directory/'binary'),'--package','./games/contra/build/cmd/probe'],ok=False)
   assert not(directory/'binary').exists()
  assert registry==(ROOT/'games/registry.go').read_bytes()and main_source==(ROOT/'main.go').read_bytes()
  report['registryUnchanged']=True;report['passed']=True
 except BaseException:report['error']=traceback.format_exc()
 finally:
  shutil.rmtree(temp);report['scriptSHA256']=hashlib.sha256(Path(__file__).read_bytes()).hexdigest();(out/'result.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n');print(json.dumps({'passed':report['passed'],'error':report.get('error'),'out':str(out)},ensure_ascii=False))
 return 0 if report['passed']else 1
if __name__=='__main__':raise SystemExit(main())
