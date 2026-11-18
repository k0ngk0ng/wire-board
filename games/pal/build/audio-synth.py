import hashlib
import json
import os
from pathlib import Path
import platform
import shutil
import subprocess
import tarfile
import urllib.request

ROOT = Path(__file__).resolve().parents[3]
LOCAL = ROOT / '.local/pal'
SOURCE = LOCAL / 'audio-synthesis-source/TiMidity++-2.15.0.tar.bz2'
BUILD = LOCAL / 'audio-synth-build'
URL = 'https://downloads.sourceforge.net/project/timidity/TiMidity++/TiMidity++-2.15.0/TiMidity++-2.15.0.tar.bz2'
SOURCE_SHA = '161fc0395af16b51f7117ad007c3e434c825a308fa29ad44b626ee8f9bb1c8f5'
BEFORE = 'int_rand(-1);'
AFTER = 'int_rand(-12345);'
FLAGS = ['--disable-dependency-tracking', '--disable-network', '--disable-dynamic', '--disable-audio', '--without-x']
CFLAGS = '-O2 -DSTDC_HEADERS -ffp-contract=off'


def sha(data):
    return hashlib.sha256(data).hexdigest()


def prepare():
    if not BUILD.resolve().is_relative_to(ROOT) or BUILD.is_symlink() or (BUILD.exists() and any(item.is_symlink() and not item.resolve().is_relative_to(BUILD) for item in BUILD.rglob('*'))):
        raise ValueError('synth build must stay inside its workspace directory without escaping symbolic links')
    compiler = shutil.which('cc')
    if not compiler:
        raise ValueError('missing C compiler for deterministic MIDI renderer')
    recipe = {'version': 2, 'sourceURL': URL, 'sourceSHA256': SOURCE_SHA, 'sourceSize': 1624751, 'license': 'GPL-2.0-or-later', 'seed': 12345, 'explicitConfigurationOnly': True, 'patch': {'file': 'timidity/timidity.c', 'from': BEFORE, 'to': AFTER, 'replaceFunctionWithNoOp': 'timidity_pre_load_configuration'}, 'configure': FLAGS, 'cflags': CFLAGS, 'compilerSHA256': sha(Path(compiler).resolve().read_bytes()), 'platform': platform.system(), 'machine': platform.machine()}
    manifest = BUILD / 'build.json'
    binary = BUILD / 'TiMidity++-2.15.0/timidity/timidity'
    if manifest.is_file():
        existing = json.loads(manifest.read_text())
        patched = BUILD / 'TiMidity++-2.15.0/timidity/timidity.c'
        if existing['recipe'] != recipe or not binary.is_file() or sha(binary.read_bytes()) != existing['binarySHA256'] or not patched.is_file() or sha(patched.read_bytes()) != existing['patchedSourceSHA256'] or not SOURCE.is_file() or sha(SOURCE.read_bytes()) != SOURCE_SHA:
            raise ValueError('cached deterministic synthesizer identity changed; inspect its dedicated build directory before rebuilding')
        return existing
    if BUILD.exists():
        raise ValueError('incomplete synthesizer build exists; preserve its logs and remove that dedicated build directory before retrying')
    SOURCE.parent.mkdir(parents=True, exist_ok=True)
    data = SOURCE.read_bytes() if SOURCE.exists() else urllib.request.urlopen(URL, timeout=45).read()
    if len(data) != recipe['sourceSize'] or sha(data) != SOURCE_SHA:
        raise ValueError('TiMidity++ source archive integrity mismatch')
    if not SOURCE.exists():
        SOURCE.write_bytes(data)
    BUILD.mkdir()
    with tarfile.open(SOURCE) as archive:
        for member in archive.getmembers():
            target = BUILD / member.name
            inside = member.name.startswith('TiMidity++-2.15.0/') or member.name == 'TiMidity++-2.15.0' and member.isdir()
            if not target.resolve().is_relative_to(BUILD) or not inside or not (member.isdir() or member.isfile()):
                raise ValueError('unsafe synthesizer source archive member')
            if member.isdir():
                target.mkdir(parents=True, exist_ok=True)
            else:
                target.parent.mkdir(parents=True, exist_ok=True)
                target.write_bytes(archive.extractfile(member).read())
                target.chmod(member.mode & 0o777)
    source = BUILD / 'TiMidity++-2.15.0'
    target = source / 'timidity/timidity.c'
    text = target.read_text()
    if text.count(BEFORE) != 1:
        raise ValueError('original synthesizer random seed initializer changed')
    text = text.replace(BEFORE, AFTER)
    start = text.index('MAIN_INTERFACE int timidity_pre_load_configuration(void)\n{')
    end = text.index('MAIN_INTERFACE int timidity_post_load_configuration(void)', start)
    target.write_text(text[:start] + 'MAIN_INTERFACE int timidity_pre_load_configuration(void)\n{\n    return 0;\n}\n\n' + text[end:])
    configure = ['./configure', *FLAGS]
    if platform.system() == 'Darwin' and platform.machine() == 'arm64':
        configure.append('--build=arm-apple-darwin')
    environment = dict(os.environ, CC=compiler, CFLAGS=CFLAGS, TMPDIR=str(ROOT / '.local/tmp'))
    for name, command in [('configure', configure), ('make', ['make', '-j4'])]:
        result = subprocess.run(command, cwd=source, env=environment, capture_output=True, text=True)
        (BUILD / (name + '.log')).write_text(result.stdout + result.stderr)
        if result.returncode:
            raise ValueError(name + ' failed; see ' + str(BUILD / (name + '.log')))
    result = {'recipe': recipe, 'binary': str(binary.relative_to(ROOT)), 'binarySHA256': sha(binary.read_bytes()), 'patchedSourceSHA256': sha(target.read_bytes())}
    manifest.write_text(json.dumps(result, indent=2) + '\n')
    return result


if __name__ == '__main__':
    print(json.dumps(prepare()))
