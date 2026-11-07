#!/usr/bin/env python3
"""Build a local native candidate with the already-built runtime's exact IDs.

No registration, upload, release, commit or deployment is performed. The main
application remains unchanged until its composition root explicitly registers
NewProductionModule. --package can select the offline initialization probe.
"""
import argparse,json,subprocess,sys
from pathlib import Path
sys.dont_write_bytecode=True
from identity import go,go_env,native_flags,native_inputs,sha,verify_runtime_file
ROOT=Path(__file__).resolve().parents[3]
def main():
 p=argparse.ArgumentParser();p.add_argument('--runtime',required=True);p.add_argument('--out',required=True);p.add_argument('--package',default='.');p.add_argument('--goos');p.add_argument('--goarch');args=p.parse_args()
 if bool(args.goos)!=bool(args.goarch):p.error('--goos and --goarch must be supplied together')
 output=(ROOT/args.out).resolve();assert output.is_relative_to(ROOT);output.parent.mkdir(parents=True,exist_ok=True)
 manifest,runtime_id,before=verify_runtime_file(ROOT,ROOT/args.runtime)
 target=(args.goos,args.goarch)if args.goos and args.goarch else None
 env=go_env(ROOT,target);flags=native_flags(manifest['rulesBuildID'],runtime_id)
 inputs=native_inputs(ROOT,args.package,env)
 subprocess.run(['go','build','-trimpath','-buildvcs=false','-ldflags='+flags,'-o',str(output),args.package],cwd=ROOT,env=env,check=True)
 _,after_id,after=verify_runtime_file(ROOT,ROOT/args.runtime)
 if after_id!=runtime_id or before!=after or inputs!=native_inputs(ROOT,args.package,env):output.unlink();raise ValueError('source or artifact changed during native compile')
 receipt={'rulesBuildID':manifest['rulesBuildID'],'runtimeBuildID':runtime_id,'visualsID':manifest['visualsID'],'goVersion':manifest['runtime']['goVersion'],'flags':flags,'package':args.package,'target':json.loads(go(ROOT,['env','-json','GOOS','GOARCH'],env)),'sha256':sha(output.read_bytes()),'bytes':output.stat().st_size,'runtimeManifestSHA256':runtime_id,'inputs':inputs}
 output.with_suffix(output.suffix+'.build.json').write_text(json.dumps(receipt,indent=2)+'\n');print(json.dumps(receipt))
if __name__=='__main__':main()
