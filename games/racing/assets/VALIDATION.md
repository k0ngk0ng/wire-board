# 赛车素材验收

制作前列出的失败场景：

- 许可缺失、下载包或原文件摘要不匹配，CC BY 漂移音效未保留作者、许可和修改说明。
- SVG 裁切错位、车身被切断或包含邻车；六个席位和五种车身未完整覆盖，图片朝向与客户端不一致。
- 图集坐标越界、透明边距丢失、场景元素缺少，图片解码失败或低分辨率放大。
- 音频无法解码、空音、削波、循环首尾产生突跳，倒计时和发车等事件无法区分。
- 本地回退与正式素材不一致，外站原始 HTML 或密钥被复制进 public。
- OSS 目标错误、覆盖已有不可变对象、CDN 内容或 MIME 与本地产物不符。

先编写 `scripts/racing/verify-assets.py`，再制作导出脚本和素材。校验记录写入 `.local/racing/verification/`，包含源文件和成品摘要、图集覆盖及音频解码／峰值／循环边界检查；`--dist` 验证实际前端回退，`--oss` 回读实际 CDN。图集、封面及缩略图另保留接触表作视觉检查。运行方式：

```sh
source scripts/env.sh
.local/board-art-env/bin/python scripts/racing/prepare-assets.py
python3 scripts/racing/verify-assets.py --decode-audio
python3 scripts/racing/verify-assets.py --dist --oss
```

这些检查只覆盖赛车素材。实际输入、相机、车辆辨识、声音事件和统一静音仍由赛车浏览器验收覆盖。
