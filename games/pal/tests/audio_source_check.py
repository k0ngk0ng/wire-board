#!/usr/bin/env python3
"""Check original sound/MIDI sources; this does not verify playback.

Failure cases: wrong source bytes, truncated/nonmonotonic MKF indexes, invalid
RIFF/PCM parameters, incomplete sample data, or malformed MIDI track bounds.
Run from the repository root with python3 -B; output stays in the workspace.
"""
from __future__ import annotations

import hashlib
import io
import json
from pathlib import Path
import struct
import wave


ROOT = Path(__file__).resolve().parents[3]
SOURCE = ROOT / ".local/pal/original-98"
MANIFEST = ROOT / "games/pal/assets/source-manifest.json"
OUTPUT = ROOT / ".local/pal/verification/audio-sources.json"


def sha(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def read_source(name: str, files: dict[str, dict]) -> bytes:
    data = (SOURCE / name).read_bytes()
    expected = files[name]
    assert len(data) == expected["size"], (name, "source size")
    assert sha(data) == expected["sha256"], (name, "source digest")
    return data


def check_midi(name: str, data: bytes) -> dict:
    assert data[:4] == b"MThd" and len(data) >= 14, (name, "MIDI header")
    header_length = struct.unpack_from(">I", data, 4)[0]
    assert header_length >= 6 and 8 + header_length <= len(data), name
    version, tracks, division = struct.unpack_from(">HHH", data, 8)
    assert version in (0, 1, 2) and tracks > 0 and division != 0, name
    at = 8 + header_length
    for _ in range(tracks):
        assert data[at:at+4] == b"MTrk" and at + 8 <= len(data), name
        size = struct.unpack_from(">I", data, at + 4)[0]
        at += 8 + size
        assert at <= len(data), (name, "truncated MIDI track")
    # The source's extracted MKF tracks retain an optional alignment byte.
    # Keep it in the source digest but exclude it from MIDI track bounds.
    padding = len(data) - at
    assert padding == 0 or (padding == 1 and at % 2 == 1
                            and len(data) % 2 == 0), (name, "MIDI tail")
    return {"path": name, "bytes": len(data), "sha256": sha(data),
            "format": version, "tracks": tracks, "division": division,
            "midiBytes": at, "archivePaddingBytes": padding,
            "archivePaddingByte": data[-1] if padding else None}


def main() -> None:
    source_manifest = json.loads(MANIFEST.read_text())
    files = {f["path"]: f for f in source_manifest["files"]}
    archive = read_source("sounds.mkf", files)
    assert len(archive) >= 8, "truncated sound MKF"
    table_bytes = struct.unpack_from("<I", archive)[0]
    assert table_bytes >= 8 and table_bytes % 4 == 0, "sound MKF index"
    offsets = struct.unpack_from("<" + "I" * (table_bytes // 4), archive)
    assert offsets[0] == table_bytes and offsets[-1] == len(archive), "MKF bounds"
    assert all(a <= b for a, b in zip(offsets, offsets[1:])), "MKF order"
    sounds = []
    empty = []
    for sound, (first, last) in enumerate(zip(offsets, offsets[1:])):
        chunk = archive[first:last]
        if not chunk:
            empty.append(sound)
            continue
        assert chunk[:4] == b"RIFF" and chunk[8:12] == b"WAVE", sound
        riff_bytes = struct.unpack_from("<I", chunk, 4)[0] + 8
        padding = len(chunk) - riff_bytes
        # Some original MKF entries align an odd RIFF payload with one byte
        # outside the RIFF size. Its value is unspecified and is not PCM.
        assert padding == 0 or (padding == 1 and riff_bytes % 2 == 1
                                and len(chunk) % 2 == 0), sound
        with wave.open(io.BytesIO(chunk), "rb") as audio:
            channels, width, rate, frames = (audio.getnchannels(),
                audio.getsampwidth(), audio.getframerate(), audio.getnframes())
            assert audio.getcomptype() == "NONE", (sound, "non-PCM source")
            assert channels > 0 and width in (1, 2, 3, 4) and rate > 0, sound
            samples = audio.readframes(frames)
            assert len(samples) == frames * channels * width, (sound, "PCM length")
            assert not audio.readframes(1), (sound, "extra PCM samples")
        sounds.append({"id": sound, "bytes": len(chunk), "sha256": sha(chunk),
            "riffBytes": riff_bytes, "archivePaddingBytes": padding,
            "archivePaddingByte": chunk[-1] if padding else None,
            "channels": channels, "bits": width * 8, "sampleRate": rate,
            "sampleFrames": frames, "pcmSHA256": sha(samples),
            "pcmEncoding": "unsigned-8" if width == 1 else "signed-little-endian"})
    music = [check_midi(name, read_source(name, files))
             for name in sorted(files) if name.startswith("musics/") and name.endswith(".mid")]
    assert sounds and music, "missing original audio"
    report = {"passed": True,
        "scope": "manifest-bound original RIFF/PCM sounds and MIDI headers/track bounds; no playback, synthesis, conversion or CDN acceptance",
        "sourceManifestSHA256": sha(MANIFEST.read_bytes()),
        "soundArchiveSHA256": sha(archive), "soundSlots": len(offsets) - 1,
        "soundCount": len(sounds), "emptySoundSlots": empty,
        "sounds": sounds, "midiCount": len(music), "midi": music,
        "playbackVerified": False, "losslessConversionVerified": False}
    OUTPUT.parent.mkdir(parents=True, exist_ok=True)
    OUTPUT.write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n")
    print(json.dumps({"passed": True, "sounds": len(sounds), "midi": len(music),
        "output": str(OUTPUT.relative_to(ROOT)), "playbackVerified": False}))


if __name__ == "__main__":
    main()
