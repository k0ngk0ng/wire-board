import argparse
import hashlib
import json
from pathlib import Path
import shutil
import struct
import subprocess

from speedtools.bnk_data import BnkData
from speedtools.types import Compression
from speedtools.utils import get_path_case_insensitive

COLLISION_PATCHES = (24, 25, 29, 30, 31, 32, 33, 34, 35, 36, 37)
SKID_PATCHES = (41, 42, 45, 46, 47)
SKID_SURFACES = {
    41: "cobble",
    42: "asphalt",
    45: "dirt",
    46: "snow",
    47: "gravel",
}
XA_TABLE = ((0, 0), (240, 0), (460, -208), (392, -220))


def sha_hex(data):
    return hashlib.sha256(data).hexdigest()


def encode_json(value):
    return json.dumps(value, ensure_ascii=False, separators=(",", ":"), allow_nan=False).encode() + b"\n"


def decode_ea_xa(raw, num_samples):
    out = []
    s1 = 0
    s2 = 0
    pos = 0
    while pos + 15 <= len(raw) and len(out) < num_samples:
        header = raw[pos]
        pos += 1
        coef_index = (header >> 4) & 0x0F
        shift = header & 0x0F
        c1, c2 = XA_TABLE[coef_index] if coef_index < 4 else (0, 0)
        for i in range(14):
            byte_val = raw[pos + i]
            for nibble in ((byte_val >> 4) & 0x0F, byte_val & 0x0F):
                if nibble >= 8:
                    nibble -= 16
                sample = ((nibble << 12) >> shift) + ((s1 * c1 + s2 * c2 + 128) >> 8)
                if sample < -32768:
                    sample = -32768
                elif sample > 32767:
                    sample = 32767
                out.append(sample)
                s2 = s1
                s1 = sample
                if len(out) >= num_samples:
                    break
            if len(out) >= num_samples:
                break
        pos += 14
    return struct.pack("<" + str(len(out)) + "h", *out)


def decode_stream_pcm(stream):
    if stream.compression == Compression.PCM:
        return bytes(stream.audio_samples)
    if stream.compression == Compression.ADPCM:
        return decode_ea_xa(stream.audio_samples, stream.decompressed_samples)
    raise ValueError("Unsupported BNK compression: " + str(stream.compression))


def encode_lossless_flac(pcm, sample_rate, channels, target):
    subprocess.run(
        [
            "ffmpeg",
            "-nostdin",
            "-v",
            "error",
            "-f",
            "s16le",
            "-ar",
            str(sample_rate),
            "-ac",
            str(channels),
            "-i",
            "pipe:0",
            "-map_metadata",
            "-1",
            "-c:a",
            "flac",
            "-compression_level",
            "12",
            "-sample_fmt",
            "s16",
            "-metadata_header_padding",
            "0",
            "-fflags",
            "+bitexact",
            "-flags:a",
            "+bitexact",
            str(target),
        ],
        input=pcm,
        capture_output=True,
        check=True,
    )
    decoded = subprocess.run(
        ["ffmpeg", "-nostdin", "-v", "error", "-i", str(target), "-f", "s16le", "-acodec", "pcm_s16le", "pipe:1"],
        capture_output=True,
        check=True,
    ).stdout
    if decoded != pcm:
        raise ValueError("FLAC roundtrip altered original collision PCM: " + str(target))


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--game-root", type=Path, required=True)
    parser.add_argument("--out", type=Path, required=True)
    args = parser.parse_args()
    repository = Path(__file__).resolve().parents[2]
    output = args.out.resolve()
    if not output.is_relative_to(repository):
        parser.error("output must stay inside this repository")
    if output.exists():
        parser.error("output must be new; existing assets are never overwritten")
    stage = output.with_name(output.name + ".building")
    if stage.exists():
        parser.error("staging directory already exists")
    source_path = get_path_case_insensitive(args.game_root, args.game_root / "Data/Audio/Sfx/gen.bnk")
    source_bytes = source_path.read_bytes()
    bank = BnkData.from_file(source_path)
    by_patch = {idx: streams for idx, streams in bank.sound_streams}
    stage.mkdir(parents=True)
    try:
        folder = stage / "audio/sfx"
        folder.mkdir(parents=True)
        patches = {}
        for patch in COLLISION_PATCHES:
            streams = by_patch.get(patch)
            if not streams:
                raise ValueError("Missing gen.bnk collision patch: " + str(patch))
            stream = streams[0]
            pcm = decode_stream_pcm(stream)
            frames = len(pcm) // (2 * stream.num_channels)
            rel_path = f"audio/sfx/collision-{patch}.flac"
            target = stage / rel_path
            encode_lossless_flac(pcm, stream.sample_rate, stream.num_channels, target)
            flac_bytes = target.read_bytes()
            patches[str(patch)] = {
                "patch": patch,
                "path": rel_path,
                "compression": stream.compression.name,
                "sampleRate": stream.sample_rate,
                "channels": stream.num_channels,
                "bits": 16,
                "frames": frames,
                "duration": round(frames / stream.sample_rate, 6),
                "bytes": len(flac_bytes),
                "sha256": sha_hex(flac_bytes),
                "pcmSha256": sha_hex(pcm),
            }
        metadata = {
            "version": 1,
            "source": "Data/Audio/Sfx/gen.bnk",
            "sourceBytes": len(source_bytes),
            "sourceSha256": sha_hex(source_bytes),
            "generator": "scripts/racing/nfs-sfx.py",
            "cutoffDistanceSqScaled": 5625,
            "patches": patches,
        }
        (folder / "collision.json").write_bytes(encode_json(metadata))
        skid_patches = {}
        for patch in SKID_PATCHES:
            streams = by_patch.get(patch)
            if not streams:
                raise ValueError("Missing gen.bnk skid patch: " + str(patch))
            stream = streams[0]
            full_pcm = decode_stream_pcm(stream)
            frame_bytes = 2 * stream.num_channels
            total_frames = len(full_pcm) // frame_bytes
            loop_start = max(0, min(stream.loop_start, total_frames - 1))
            loop_end = max(loop_start, min(stream.loop_end, total_frames - 1))
            loop_pcm = full_pcm[loop_start * frame_bytes : (loop_end + 1) * frame_bytes]
            loop_frames = len(loop_pcm) // frame_bytes
            rel_path = f"audio/sfx/skid-{patch}.flac"
            target = stage / rel_path
            encode_lossless_flac(loop_pcm, stream.sample_rate, stream.num_channels, target)
            flac_bytes = target.read_bytes()
            skid_patches[str(patch)] = {
                "patch": patch,
                "surface": SKID_SURFACES[patch],
                "path": rel_path,
                "compression": stream.compression.name,
                "sampleRate": stream.sample_rate,
                "channels": stream.num_channels,
                "bits": 16,
                "loopStart": loop_start,
                "loopEnd": loop_end,
                "frames": loop_frames,
                "duration": round(loop_frames / stream.sample_rate, 6),
                "volume": stream.volume,
                "bytes": len(flac_bytes),
                "sha256": sha_hex(flac_bytes),
                "pcmSha256": sha_hex(loop_pcm),
            }
        skid_metadata = {
            "version": 1,
            "source": "Data/Audio/Sfx/gen.bnk",
            "sourceBytes": len(source_bytes),
            "sourceSha256": sha_hex(source_bytes),
            "generator": "scripts/racing/nfs-sfx.py",
            "patches": skid_patches,
        }
        (folder / "skid.json").write_bytes(encode_json(skid_metadata))
        assets = []
        for path in sorted(p for p in folder.iterdir() if p.is_file()):
            raw = path.read_bytes()
            assets.append(
                {
                    "path": path.relative_to(stage).as_posix(),
                    "bytes": len(raw),
                    "sha256": sha_hex(raw),
                    "mime": "audio/flac" if path.suffix == ".flac" else "application/json",
                }
            )
        manifest = {"version": 1, "source": metadata["source"], "assets": assets, "totalBytes": sum(a["bytes"] for a in assets)}
        (stage / "sfx-manifest.json").write_bytes(encode_json(manifest))
        stage.rename(output)
        print(json.dumps({"output": str(output.relative_to(repository)), "patches": len(patches), "skidPatches": len(skid_patches), "bytes": manifest["totalBytes"]}))
    except BaseException:
        shutil.rmtree(stage)
        raise


if __name__ == "__main__":
    main()
