# 原生主、副武器图集 v5

v5 保留 14 辆坦克的完整原版弹体图集，以及地雷、锁定标记和离子炮素材，共 17 个不可变对象、1085 帧、115326 字节。媒体位于 `/fortressblue/v5/`，没有覆盖 v2、v3 或 v4。

每辆 `bulletXX.i16` 的 0–31 帧为主弹，32–63 帧为副弹，每组包含 8 个方向、每方向 4 帧。额外原帧也保留。图片直接使用原生方向，不旋转或镜像已带方向的像素图；矩形、锚点、RGBA 像素与原文件逐帧一致。`projectiles.json` 与 `projectile-effects.json` 保存帧表和原始文件 SHA-256，`manifest-v5.json` 绑定发布对象的大小及 SHA-256。

```sh
python3 -B scripts/fortressblue/prepare-projectile-assets.py
python3 -B scripts/fortressblue/verify-projectile-assets.py --dist
python3 -B scripts/fortressblue/verify-assets.py --dist
```

生成器需要仓库内保留的原包及解码依赖，已存在的媒体若内容不同会拒绝覆盖。标准验证器仅依赖已提交文件和 Python 标准库。不可变上传由 `publish-secondary-assets.py` 负责；原图版权和来源归属不因解码改变。

动画间隔属于平台呈现参数：弹体每 90 ms 一帧；锁定标记每 100 ms 一相位，原素材 7 帧，第 8 相位留空；离子命中每 70 ms 一帧、10 帧，离子光束每 60 ms 一帧、8 帧。这些毫秒值没有声称来自原版实测。源像素、原始帧组和游戏时间轴分别核验。

来源与玩法边界见 [二号弹审计](../../../docs/research/fortress-secondary/README.md)。逐像素结果位于 `.local/fortress-secondary/source-pixels.json`，OSS／CDN 读回结果位于 `.local/fortress-secondary/oss-publish.json`。
