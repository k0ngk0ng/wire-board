# 街霸方块素材

本目录只提交素材清单、来源和标签；商业媒体不进入 Git。

## 内容

- `portraits/<id>.webp`：8 名可选角色的立绘，来自用户提供的 `.local/spf2t` 提取；Ryu 使用参考对局截图补齐。
- `audio/bgm-{select,battle}.mp3`：从游戏音乐中选取的循环 BGM。
- `audio/sfx-<event>.mp3`：选角、倒计时、锁定、碎裂、连锁、强化、钻石、反击、顶出、胜负音效。
- `gems.webp` 暂未打包：前端使用内置矢量/渐变宝石回退；后续可按 `web/gems.ts` 的 4×5 图集布局补充。
- `manifest.json`：每个文件的字节数、SHA-256、尺寸/时长、来源和 CDN 路径。
- `sources.json`：ROM SHA-256、生成器 SHA-256、来源与授权状态。
- `labels.json`：角色 id、中文名、立绘路径和音效事件映射。

## 准备与本地回退

```sh
python3 -B scripts/spf2t/prepare-assets.py --install-local
```

输出到 `.local/spf2t-pack/spf2t/v1`，并在 `--install-local` 时复制到
`games/spf2t/assets/public/spf2t/v1/`。该目录被 `.gitignore` 忽略；前端构建会把
它复制到 `web/dist`，作为 CDN 失败时的本地回退。

## 校验

```sh
python3 -B scripts/spf2t/verify-assets.py --dist
python3 -B scripts/spf2t/verify-assets.py --oss
```

干净仓库没有商业媒体时，`--dist` 记录 `externalOnly: true` 并通过；如果存在任意
本地回退文件，则全部条目都必须匹配。

## 上传

部署的实际素材基地址为 `https://cdn.ichenj.com/bga/v20261002-1`。上传前缀为
`bga/v20261002-1/spf2t/v1`，不修改共享基地址，不覆盖旧对象：

```sh
python3 -B scripts/spf2t/publish-assets.py
```

脚本使用 `stoneage-assets` bucket、`stoneage-assets.oss-cn-shanghai.aliyuncs.com`
endpoint 和 `config/secrets/` 中的 OSS 凭据；上传后从 CDN 回读 SHA-256 与 MIME。

## 授权

素材由用户提供；项目不声明商业素材的再分发许可，部署者自行确认授权。
原始 ROM、WAV、PNG 和参考截图不提交 Git。封面为 `games/spf2t/assets/art.ts`
中的原创 SVG，不使用原版盒装封面、扫描图、Capcom Logo 或大段标题字。
