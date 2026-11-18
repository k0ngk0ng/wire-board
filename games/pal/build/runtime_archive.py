"""Immutable native artifacts and hash-bound original/development provenance."""
import hashlib
import json
from pathlib import Path
import re
import shutil
import tempfile

WASM_MAGIC = b'\0asm\1\0\0\0'
STAMP_NAME = b'wire-board.pal.build'
ROOT = Path(__file__).resolve().parents[3]
PURPOSES = {'campaign': 1, 'font-reference': 2, 'rng-fixture': 3, 'battle-fixture': 4, 'ending-fixture': 5, 'game-over-fixture': 6}


def validate_stamp(stamp):
    if not isinstance(stamp, dict) or set(stamp) != {'format', 'profile', 'purpose', 'sourceCommit'}:
        raise ValueError('invalid PAL build stamp fields')
    if type(stamp['format']) is not int or stamp['format'] != 1:
        raise ValueError('unsupported PAL build stamp format')
    profile = stamp['profile']
    if not isinstance(profile, str) or not (profile == 'original-98' or re.fullmatch(r'development-health:[1-9][0-9]{3}', profile)):
        raise ValueError('invalid PAL rules profile')
    if stamp['purpose'] not in PURPOSES or not isinstance(stamp['sourceCommit'], str) or not re.fullmatch(r'[0-9a-f]{40}', stamp['sourceCommit']):
        raise ValueError('invalid PAL native provenance')
    return stamp


def _uleb(value):
    out = bytearray()
    while True:
        byte = value & 127
        value >>= 7
        out.append(byte | (128 if value else 0))
        if not value:
            return bytes(out)


def _read_uleb(data, pos):
    value = 0
    for index in range(5):
        if pos >= len(data):
            raise ValueError('truncated Wasm section length')
        byte = data[pos]
        pos += 1
        if index == 4 and byte > 15:
            raise ValueError('oversized Wasm section length')
        value |= (byte & 127) << (index * 7)
        if not byte & 128:
            return value, pos
    raise ValueError('invalid Wasm section length')


def read_stamp(data):
    if not data.startswith(WASM_MAGIC):
        raise ValueError('invalid Wasm header')
    pos, found = 8, None
    while pos < len(data):
        kind = data[pos]
        length, start = _read_uleb(data, pos + 1)
        end = start + length
        if end > len(data):
            raise ValueError('truncated Wasm section')
        if kind == 0:
            name_size, body = _read_uleb(data, start)
            if body + name_size > end:
                raise ValueError('truncated Wasm custom section name')
            if data[body:body+name_size] == STAMP_NAME:
                if found is not None or end-body-name_size > 4096:
                    raise ValueError('duplicate or oversized PAL build stamp')
                found = validate_stamp(json.loads(data[body+name_size:end]))
        pos = end
    return found


def stamp_wasm(data, stamp):
    validate_stamp(stamp)
    if read_stamp(data) is not None:
        raise ValueError('PAL binary already has a build stamp')
    body = _uleb(len(STAMP_NAME)) + STAMP_NAME + json.dumps(stamp, separators=(',', ':'), sort_keys=True).encode()
    return data + b'\0' + _uleb(len(body)) + body


def output_directory(root, value):
    root = Path(root).resolve()
    result = (root / value).resolve()
    if not result.is_relative_to(root):
        raise ValueError('PAL output must remain inside the workspace')
    return result


def _verify(program, metadata):
    data = program.read_bytes()
    doc = json.loads(metadata.read_text())
    digest = hashlib.sha256(data).hexdigest()
    if doc.get('runtimeSHA256') != digest or doc.get('runtimeBytes') != len(data):
        raise ValueError('native artifact digest/size does not match its build manifest')
    stamp = read_stamp(data)
    if stamp is None:
        stamp = json.loads((ROOT / 'games/pal/engine/legacy-runtimes.json').read_text()).get(digest)
        if stamp is None:
            raise ValueError('unclassified legacy core cannot become a retained runtime')
    hp = doc.get('testBattleHP')
    expected_profile = 'original-98' if hp is None else 'development-health:' + str(hp)
    purpose = 'game-over-fixture' if doc.get('gameOverFixture') else 'ending-fixture' if doc.get('endingFixture') else 'font-reference' if doc.get('referenceFont') else 'rng-fixture' if doc.get('rngFixture') is not None else 'battle-fixture' if doc.get('battleFixture') is not None else 'campaign'
    if stamp['profile'] != expected_profile or stamp['purpose'] != purpose or stamp['sourceCommit'] != doc.get('sourceCommit'):
        raise ValueError('native profile/purpose/source does not match its build manifest')
    return digest, data, stamp


def retain_runtime(root, program, metadata):
    root = Path(root).resolve()
    program, metadata = output_directory(root, program), output_directory(root, metadata)
    digest, data, stamp = _verify(program, metadata)
    store = output_directory(root, '.local/pal/runtime-versions')
    target = store / digest
    output_directory(root, target)
    if target.is_symlink():
        raise ValueError('retained native version must not alias another path')
    store.mkdir(parents=True, exist_ok=True)

    def existing():
        if target.is_symlink():
            raise ValueError('retained native version must be an immutable directory')
        try:
            old_digest, old_data, old_stamp = _verify(target / 'rules.wasm', target / 'build.json')
        except OSError as error:
            raise ValueError('incomplete retained native version') from error
        if old_digest != digest or old_data != data or old_stamp != stamp:
            raise ValueError('retained native version is corrupt; refusing to overwrite')
        return target

    if target.exists():
        return existing()
    stage = Path(tempfile.mkdtemp(prefix='.staging-', dir=store))
    try:
        (stage / 'rules.wasm').write_bytes(data)
        (stage / 'build.json').write_bytes(metadata.read_bytes())
        try:
            stage.rename(target)
        except OSError:
            if not target.exists():
                raise
            return existing()
        return target
    finally:
        if stage.exists():
            shutil.rmtree(stage)
