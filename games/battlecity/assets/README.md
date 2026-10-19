# 坦克大战素材

源 ROM 为用户提供的 `Battle City (J).nes`（SHA-256 `a869aead5b6957fc62002ff9636e048cc34baf0100d629b07dc51aa18f220c0c`）。商业媒体只放 OSS 与本地回退，不提交 Git、Release 或镜像。

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

不可变前缀 `bga/v20261002-1/battlecity/v1/`，bucket `stoneage-assets`，CDN `https://cdn.ichenj.com/bga/v20261002-1`；`x-oss-forbid-overwrite` 防止覆盖既有对象。前端优先 CDN，失败回退 `/battlecity/v1/`，最后回退 Canvas 矢量坦克/地形。

## 授权

User-provided commercial media from Battle City (J).nes. The project does not claim redistribution rights; the deployer confirms authorization. Media is uploaded to OSS and is not committed to Git, Releases or images. Battle City 及其名称、商标和商业美术归 Namco 及各自权利人。
