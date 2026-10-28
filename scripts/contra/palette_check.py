"""Run palette-only Go tests without unrelated concurrent test-first files."""
import argparse,os,subprocess
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2]


def main():
 p=argparse.ArgumentParser();p.add_argument('--race',action='store_true');a=p.parse_args()
 base=ROOT/'.local/contra-palette';temporary=base/'runtime/tmp';temporary.mkdir(parents=True,exist_ok=True)
 cache=ROOT/'.local/go-build';modules=ROOT/'.local/go-mod';cache.mkdir(parents=True,exist_ok=True);modules.mkdir(parents=True,exist_ok=True)
 env={**os.environ,'GOCACHE':str(cache),'GOMODCACHE':str(modules),'GOTMPDIR':str(temporary),'TMPDIR':str(temporary),'GOTOOLCHAIN':'local'}
 directory=ROOT/'games/contra/engine'
 paths=subprocess.check_output(['rg','--files',str(directory),'-g','*.go','-g','!*_test.go'],text=True).splitlines()
 files=sorted(Path(s).name for s in paths if Path(s).parent==directory)
 command=['go','test',*(['-race']if a.race else []),'-count=1',*files,'palette_test.go']
 raise SystemExit(subprocess.run(command,cwd=directory,env=env).returncode)


if __name__=='__main__':main()
