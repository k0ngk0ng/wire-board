#!/usr/bin/env python3
"""Prepare the SPF2T media pack from the user-provided extraction.

Commercial media is written only to .local and, with --install-local, to the
gitignored games/spf2t/assets/public/spf2t/v1 directory. The committed metadata
contains paths, hashes and labels only.
"""
import argparse
import hashlib
import json
import shutil
import subprocess
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
SOURCE = ROOT / ".local/spf2t"
PACK = ROOT / ".local/spf2t-pack/spf2t/v1"
LOCAL = ROOT / "games/spf2t/assets/public/spf2t/v1"
META = ROOT / "games/spf2t/assets"
CDN_BASE = "https://cdn.ichenj.com/bga/v20261002-1"
PREFIX = "bga/v20261002-1/spf2t/v1"

# Character-select order and the extracted portrait files. The first slot in
# the extraction is blank; the reference Ryu crop fills it.
PORTRAITS = {
    "chunli": "portrait_01_bf6ce7985a.png",
    "morrigan": "portrait_02_25d33dd5eb.png",
    "sakura": "portrait_03_125bfe4448.png",
    "felicia": "portrait_04_c346c80dc6.png",
    "ken": "portrait_05_8096cbe794.png",
    "donovan": "portrait_06_f27bcf5238.png",
    "hsienko": "portrait_07_cfc3ea5916.png",
}
CHARACTER_NAMES = {
    "morrigan": "莫莉卡",
    "chunli": "春丽",
    "ryu": "隆",
    "ken": "肯",
    "hsienko": "泪泪",
    "donovan": "多诺万",
    "felicia": "菲莉西亚",
    "sakura": "樱",
}
RYU_CROP = ROOT / ".local/spf2t/reference/evidence/p1_board_3400_4x.png"

SFX = {
    "select": "sfx_code_00b1.wav",
    "countdown": "sfx_code_00a7.wav",
    "lock": "sfx_code_00b0.wav",
    "crash": "sfx_code_00b4.wav",
    "chain": "sfx_code_00b5.wav",
    "power": "sfx_code_00b7.wav",
    "diamond": "sfx_code_00b8.wav",
    "garbage": "sfx_code_00b9.wav",
    "topout": "sfx_code_00ba.wav",
    "win": "sfx_code_00a8.wav",
    "lose": "sfx_code_00a9.wav",
}
BGM = {
    "bgm-select": "bgm_code_0001.wav",
    "bgm-battle": "bgm_code_0002.wav",
}


def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def run(*args):
    subprocess.run(args, check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)


def probe(path):
    result = subprocess.run(
        ["ffprobe", "-v", "error", "-select_streams", "v:0", "-show_entries", "stream=width,height", "-of", "csv=p=0", str(path)],
        check=True, text=True, capture_output=True,
    )
    width, height = result.stdout.strip().split(",")[:2]
    return int(width), int(height)


def duration(path):
    result = subprocess.run(
        ["ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "default=nk=1:nw=1", str(path)],
        check=True, text=True, capture_output=True,
    )
    return round(float(result.stdout.strip()), 3)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--install-local", action="store_true")
    args = parser.parse_args()
    if not SOURCE.exists():
        raise SystemExit("missing .local/spf2t")
    for tool in ("ffmpeg", "ffprobe", "cwebp"):
        if not shutil.which(tool):
            raise SystemExit(f"missing {tool}")
    shutil.rmtree(PACK.parent, ignore_errors=True)
    (PACK / "portraits").mkdir(parents=True)
    (PACK / "audio").mkdir(parents=True)
    entries = []

    def add(relative, source, kind, extra):
        path = PACK / relative
        w = path.read_bytes()
        entry = {
            "file": relative,
            "urlPath": "/spf2t/v1/" + relative,
            "bytes": len(w),
            "sha256": hashlib.sha256(w).hexdigest(),
            "kind": kind,
            "source": source,
            **extra,
        }
        entries.append(entry)

    portrait_dir = SOURCE / "graphics/characters/portraits"
    for character, filename in PORTRAITS.items():
        output = PACK / "portraits" / f"{character}.webp"
        run("cwebp", "-lossless", "-alpha_q", "100", "-quiet", str(portrait_dir / filename), "-o", str(output))
        w, h = probe(output)
        add(f"portraits/{character}.webp", f"graphics/characters/portraits/{filename}", "portrait", {"character": character, "width": w, "height": h})

    ryu = RYU_CROP
    if ryu.exists():
        tmp = PACK / "portraits" / "ryu.png"
        run("ffmpeg", "-y", "-i", str(ryu), "-vf", "crop=480:420:65:115,scale=128:112", "-frames:v", "1", str(tmp))
        output = PACK / "portraits" / "ryu.webp"
        run("cwebp", "-lossless", "-alpha_q", "100", "-quiet", str(tmp), "-o", str(output))
        tmp.unlink()
        w, h = probe(output)
        add("portraits/ryu.webp", str(ryu.relative_to(ROOT)), "portrait", {"character": "ryu", "width": w, "height": h})
    else:
        print("warning: Ryu reference crop missing; Ryu will use the initial fallback")

    for name, filename in BGM.items():
        output = PACK / "audio" / f"{name}.mp3"
        run("ffmpeg", "-y", "-i", str(SOURCE / "audio/music" / filename), "-codec:a", "libmp3lame", "-b:a", "128k", "-ar", "44100", "-ac", "2", str(output))
        add(f"audio/{name}.mp3", f"audio/music/{filename}", "bgm", {"duration": duration(output)})

    for name, filename in SFX.items():
        output = PACK / "audio" / f"sfx-{name}.mp3"
        run("ffmpeg", "-y", "-i", str(SOURCE / "audio/sfx" / filename), "-codec:a", "libmp3lame", "-b:a", "96k", "-ar", "44100", "-ac", "1", str(output))
        add(f"audio/sfx-{name}.mp3", f"audio/sfx/{filename}", "sfx", {"event": name, "duration": duration(output)})

    roms = json.loads((SOURCE / "metadata/MANIFEST.json").read_text()).get("roms_sha256", {})
    generator = Path(__file__).resolve()
    sources = {
        "source": "User-provided .local/spf2t extraction of Super Puzzle Fighter II Turbo (Europe 960529, CPS2, spf2t)",
        "generator": str(generator.relative_to(ROOT)),
        "generatorSha256": sha(generator),
        "romsSha256": roms,
        "license": "User-provided commercial media. The project does not claim redistribution rights; the deployer confirms authorization. Media is uploaded to OSS and is not committed to Git.",
        "characters": {cid: {"name": name} for cid, name in CHARACTER_NAMES.items()},
        "audio": {"bgm": BGM, "sfx": SFX},
    }
    manifest = {
        "version": "spf2t/v1",
        "prefix": PREFIX + "/",
        "cdnBase": CDN_BASE,
        "generator": str(generator.relative_to(ROOT)),
        "generatorSha256": sha(generator),
        "license": sources["license"],
        "entries": entries,
    }
    labels = {
        "characters": {
            cid: {"name": name, "portrait": f"portraits/{cid}.webp"}
            for cid, name in CHARACTER_NAMES.items()
        },
        "audio": {
            "bgm": {"select": "audio/bgm-select.mp3", "battle": "audio/bgm-battle.mp3"},
            "sfx": {name: f"audio/sfx-{name}.mp3" for name in SFX},
        },
    }
    META.mkdir(parents=True, exist_ok=True)
    (META / "manifest.json").write_text(json.dumps(manifest, ensure_ascii=False, indent=2) + "\n")
    (META / "sources.json").write_text(json.dumps(sources, ensure_ascii=False, indent=2) + "\n")
    (META / "labels.json").write_text(json.dumps(labels, ensure_ascii=False, indent=2) + "\n")
    if args.install_local:
        shutil.rmtree(LOCAL.parent, ignore_errors=True)
        LOCAL.parent.mkdir(parents=True, exist_ok=True)
        shutil.copytree(PACK, LOCAL)
    total = sum(entry["bytes"] for entry in entries)
    print(f"prepared {len(entries)} objects, {total} bytes; pack={PACK.relative_to(ROOT)}")


if __name__ == "__main__":
    main()
