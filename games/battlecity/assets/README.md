# 坦克大战素材

源 ROM 为用户提供的 `Battle City (J).nes`（SHA-256 `a869aead5b6957fc62002ff9636e048cc34baf0100d629b07dc51aa18f220c0c`）。商业媒体只放 OSS 与本地回退，不提交 Git、Release 或镜像。

## 准备与本地回退

```sh
python3 -B scripts/battlecity/prepare_assets.py --install-local
python3 -B scripts/battlecity/package_assets.py --atlas-v2
python3 -B scripts/battlecity/verify-assets.py --dist
```

图集帧名与 `CONTRACT.md` 第 11 节一致；音频直接从原 ROM 的 `$AA51/$AA7E` 声音引擎与捕获的队列快照渲染，不做合成替代。`sources.json` 记录 ROM/归档/生成器/队列快照和音效映射。

## 上传与回读

```sh
python3 -B scripts/battlecity/publish-assets.py --atlas-v2
python3 -B scripts/battlecity/verify-assets.py --oss
```

不可变前缀 `bga/v20261002-1/battlecity/v1/`，bucket `stoneage-assets`，CDN `https://cdn.ichenj.com/bga/v20261002-1`；`x-oss-forbid-overwrite` 防止覆盖既有对象。v1 为历史图集与现有声音前缀；当前图集使用下述 v2，前端优先 CDN，失败回退相同版本本地路径，最后回退 Canvas 矢量坦克/地形。

## 授权

User-provided commercial media from Battle City (J).nes. The project does not claim redistribution rights; the deployer confirms authorization. Media is uploaded to OSS and is not committed to Git, Releases or images. Battle City 及其名称、商标和商业美术归 Namco 及各自权利人。

## 方向图集修正（v2）

五车型、四方向各有原生 CHR 图案，不能把左右／上下视作同一张图的镜像。v2 使用原 ROM `$DFEB–$DFFF` 与 `$DA73/$DA7B` 的完整取帧方式；40 次受控原生渲染、双方配色共 80 帧逐像素比对通过。具体证据见 `docs/research/battlecity-directions/RESULTS.md`。

当前运行时图集使用 `/battlecity/v2/atlas.webp`，失败后按相同 v2 路径加载本地回退。声音继续使用 v1；既有 v1 对象不得覆盖。`manifest.json`／`sources.json` 保留 v1 历史来源，`manifest-v2.json` 与当前 `atlas.json` 记录修正图集。

```sh
python3 -B scripts/battlecity/prepare_assets.py --install-local
python3 -B scripts/battlecity/package_assets.py --atlas-v2
python3 -B scripts/battlecity/publish-assets.py --atlas-v2
```

生成／像素验收脚本需要 Pillow，可复用工作区 `.local/board-art-env/bin/python`。发布仍需执行不可变上传与回读后才能启用 v2。
