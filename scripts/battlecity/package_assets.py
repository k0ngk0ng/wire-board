#!/usr/bin/env python3
"""Write committed manifest/sources/labels and README from the local pack.

Only metadata is written to games/battlecity/assets.  Commercial media stays
in .local and in the gitignored local fallback directory.
"""
from __future__ import annotations

import argparse
import hashlib
import json
import shutil
import subprocess
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
PACK = ROOT / ".local/battlecity-pack/battlecity/v1"
META = ROOT / "games/battlecity/assets"
PREFIX = "bga/v20261002-1/battlecity/v1/"
CDN = "https://cdn.ichenj.com/bga/v20261002-1"
ROM_SHA = "a869aead5b6957fc62002ff9636e048cc34baf0100d629b07dc51aa18f220c0c"
ARCHIVE_SHA = "eeffcf3d376770e4265264dfe04a649babd00c49355fc8a6bbbf69dfd66e3d90"
ARCHIVE = "/Users/jason/Documents/nes/roms/02/坦克大战(打坦克，Battle City).zip"
LICENSE = (
    "User-provided commercial media from Battle City (J).nes. The project does not "
    "claim redistribution rights; the deployer confirms authorization. Media is "
    "uploaded to OSS and is not committed to Git, Releases or images."
)


def sha(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def ffprobe_duration(path: Path) -> float:
    out = subprocess.run(
        ["ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "default=nw=1:nk=1", str(path)],
        check=True, capture_output=True, text=True,
    )
    return round(float(out.stdout.strip()), 3)


def package_atlas_v2() -> None:
    pack = ROOT / ".local/battlecity-pack/battlecity/v2"
    atlas = json.loads((pack / "atlas.json").read_text())
    assert atlas["version"] == 2 and len(atlas["frames"]) == 101
    manifest = {"version": "battlecity/v2", "prefix": "bga/v20261002-1/battlecity/v2/",
                "cdnBase": CDN, "license": LICENSE, "source": {"romSha256": ROM_SHA},
                "generator": "scripts/battlecity/prepare_assets.py",
                "generatorSha256": sha(ROOT / "scripts/battlecity/prepare_assets.py"),
                "entries": []}
    for filename, kind in (("atlas.webp", "atlas"), ("atlas.json", "atlas-meta")):
        path = pack / filename
        manifest["entries"].append({"file": filename, "urlPath": "/battlecity/v2/" + filename,
            "kind": kind, "sha256": sha(path), "bytes": path.stat().st_size})
    (META / "manifest-v2.json").write_text(json.dumps(manifest, ensure_ascii=False, indent=2) + "\n")
    shutil.copyfile(pack / "atlas.json", META / "atlas.json")
    # Coordinates and dimensions stay stable; verify the committed browser table.
    previous = json.loads((ROOT / ".local/battlecity-pack/battlecity/v1/atlas.json").read_text())
    assert {k: {f: v[f] for f in ("x", "y", "w", "h")} for k,v in atlas["frames"].items()} == {k: {f: v[f] for f in ("x", "y", "w", "h")} for k,v in previous["frames"].items()}
    print(json.dumps({"version": "battlecity/v2", "entries": 2, "bytes": sum(e["bytes"] for e in manifest["entries"])}))


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--atlas-v2", action="store_true", help="package corrected atlas only; retain v1 audio")
    args = parser.parse_args()
    if args.atlas_v2:
        package_atlas_v2()
        return
    atlas = json.loads((PACK / "atlas.json").read_text())
    audio = atlas.get("audio", {})
    if len(audio) != 11:
        raise SystemExit(f"expected 11 audio keys, got {len(audio)}")
    entries = []
    files = [PACK / "atlas.webp", PACK / "atlas.json"] + sorted((PACK / "audio").glob("*.mp3"))
    for path in files:
        rel = path.relative_to(PACK).as_posix()
        entry = {
            "file": rel,
            "urlPath": "/battlecity/v1/" + rel,
            "bytes": path.stat().st_size,
            "sha256": sha(path),
        }
        if rel == "atlas.webp":
            entry.update({"kind": "atlas", "width": atlas["width"], "height": atlas["height"], "frames": len(atlas["frames"])})
        elif rel == "atlas.json":
            entry.update({"kind": "atlas-meta"})
        else:
            key = path.stem
            record = audio.get(key)
            if not record:
                raise SystemExit(f"audio metadata missing for {key}")
            entry.update({"kind": "bgm" if key.startswith("bgm-") else "sfx", "duration": record["duration_s"], "event": key})
        entries.append(entry)

    manifest = {
        "version": "battlecity/v1",
        "prefix": PREFIX,
        "cdnBase": CDN,
        "generator": "scripts/battlecity/prepare_assets.py",
        "generatorSha256": sha(ROOT / "scripts/battlecity/prepare_assets.py"),
        "license": LICENSE,
        "entries": entries,
    }
    sources = {
        "source": "User-provided Battle City (J).nes extraction (NROM-128)",
        "archive": {"path": ARCHIVE, "sha256": ARCHIVE_SHA},
        "rom": {"path": ".local/battlecity/rom/Battle City (J).nes", "sha256": ROM_SHA, "bytes": 24592},
        "generator": {
            "atlas": "scripts/battlecity/prepare_assets.py",
            "atlasSha256": sha(ROOT / "scripts/battlecity/prepare_assets.py"),
            "audio": "scripts/battlecity/render_nsf_slot.py / render_nsf_music.py",
            "audioSha256": {
                "render_nsf_slot.py": sha(ROOT / "scripts/battlecity/render_nsf_slot.py"),
                "render_nsf_music.py": sha(ROOT / "scripts/battlecity/render_nsf_music.py"),
                "render_audio.py": sha(ROOT / "scripts/battlecity/render_audio.py"),
            },
            "palette": atlas.get("palette"),
        },
        "license": LICENSE,
        "audio": {
            "method": "Original $AA51 INIT / $AA7E PLAY sound engine in a 32KB NROM-mirrored NSF; queue snapshots copied into $0300-$03FF; no game code or synthesis.",
            "queueSnapshots": [
                "queue-frame365.bin", "queue-frame460.bin", "queue-frame504.bin",
                "queue-frame541.bin", "queue-frame602.bin", "queue-frame614.bin",
                "queue-frame638.bin", "queue-frame1245.bin",
            ],
            "mapping": {
                "bgm-start": "music queue at frame 365 (stage start)",
                "bgm-battle": "music queue at frame 460 (stage battle loop capture)",
                "sfx-fire": "slot 15 at frame 504",
                "sfx-brick": "slot 13 at frame 602",
                "sfx-steel": "slot 12 at frame 614",
                "sfx-explode": "slots 7+11 at frame 638, mixed",
                "sfx-powerup": "slot 17 at frame 541",
                "sfx-shield": "slot 11 at frame 638",
                "sfx-base": "slot 7 at frame 638",
                "sfx-win": "slot 25 at frame 1245",
                "sfx-lose": "slot 24 at frame 1245",
            },
        },
    }
    labels = {
        "teams": {"0": "上方队伍", "1": "下方队伍"},
        "classes": {
            c: {"name": n}
            for c, n in {
                "balanced": "均衡坦克", "basic": "基础坦克", "fast": "快速坦克",
                "power": "火力坦克", "armor": "装甲坦克",
            }.items()
        },
        "maps": {m["id"]: m["name"] for m in json.loads((ROOT / "games/battlecity/maps/catalog.json").read_text())["maps"]},
        "audio": {k: f"audio/{k}.mp3" for k in sorted(audio)},
    }
    META.mkdir(parents=True, exist_ok=True)
    (META / "manifest.json").write_text(json.dumps(manifest, ensure_ascii=False, indent=2) + "\n")
    (META / "sources.json").write_text(json.dumps(sources, ensure_ascii=False, indent=2) + "\n")
    (META / "labels.json").write_text(json.dumps(labels, ensure_ascii=False, indent=2) + "\n")
    shutil.copyfile(PACK / "atlas.json", META / "atlas.json")
    # The frame table is metadata, not media.  Committing it lets the browser
    # load the atlas image cross-origin without fetching atlas.json (the CDN's
    # CORS policy only names the production origin).
    web_frames = {
        name: {"x": f["x"], "y": f["y"], "w": f["w"], "h": f["h"]}
        for name, f in atlas["frames"].items()
    }
    (META.parent / "web" / "atlas.ts").write_text(
        "// Generated by scripts/battlecity/package_assets.py from the verified atlas.\n"
        "export type AtlasFrame = { x: number; y: number; w: number; h: number };\n"
        f"export const ATLAS_WIDTH = {atlas['width']};\n"
        f"export const ATLAS_HEIGHT = {atlas['height']};\n"
        "export const ATLAS_FRAMES: Record<string, AtlasFrame> = "
        + json.dumps(web_frames, separators=(",", ":"))
        + ";\n"
    )
    readme = f"""# 坦克大战素材

源 ROM 为用户提供的 `Battle City (J).nes`（SHA-256 `{ROM_SHA}`）。商业媒体只放 OSS 与本地回退，不提交 Git、Release 或镜像。

## 准备与本地回退

```sh
python3 -B scripts/battlecity/prepare_assets.py --masters .local/battlecity/audio-masters --install-local
python3 -B scripts/battlecity/package_assets.py
python3 -B scripts/battlecity/verify-assets.py --dist
```

图集帧名与 `CONTRACT.md` 第 11 节一致；音频直接从原 ROM 的 `$AA51/$AA7E` 声音引擎与捕获的队列快照渲染，不做合成替代。`sources.json` 记录 ROM/归档/生成器/队列快照和音效映射。

## 上传与回读

```sh
python3 -B scripts/battlecity/publish-assets.py
python3 -B scripts/battlecity/verify-assets.py --oss
```

不可变前缀 `{PREFIX}`，bucket `stoneage-assets`，CDN `{CDN}`；`x-oss-forbid-overwrite` 防止覆盖既有对象。前端优先 CDN，失败回退 `/battlecity/v1/`，最后回退 Canvas 矢量坦克/地形。

## 授权

{LICENSE} Battle City 及其名称、商标和商业美术归 Namco 及各自权利人。
"""
    (META / "README.md").write_text(readme)
    print(json.dumps({"entries": len(entries), "bytes": sum(e["bytes"] for e in entries), "assets": str(META)}, indent=2))


if __name__ == "__main__":
    main()
