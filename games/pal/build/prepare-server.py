import argparse
import importlib.util
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[3]
spec = importlib.util.spec_from_file_location('pal_server_distribution', ROOT / 'games/pal/build/server-distribution.py')
distribution = importlib.util.module_from_spec(spec)
spec.loader.exec_module(distribution)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--out', type=Path)
    parser.add_argument('--release')
    args = parser.parse_args()
    release = distribution.retained_release()
    identity = args.release or release.get('current')
    if identity not in release['catalogs']:
        parser.error('no verified source-bound PAL server release is available')
    pin = release['catalogs'][identity]
    target = args.out or ROOT / '.local/pal/prepared-server' / identity / 'pal'
    report = distribution.prepare(pin, target)
    report['runtime'] = distribution.verify(target)
    print(json.dumps(report))


if __name__ == '__main__':
    main()
