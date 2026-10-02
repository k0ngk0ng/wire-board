# 可选外部素材

在部署服务器私有 `.env` 中设置 `ASSETS_BASE_URL=https://assets.example.com/games/version`。不设置时显示内置替代图形。该地址必须为 HTTPS，不能含查询参数。修改后重新执行更新脚本。

素材目录包含：

- `splendor/cards.webp`：发展卡和牌背，6 × 6 图集。
- `splendor/nobles.webp`：贵族，5 × 3 图集。
- `splendor/tokens.webp`：六种筹码，6 × 1 图集。
- `rail/map-unlabeled-v4.webp`：1744 × 1125 美国地图，移除底图城市文字及白边、保留纸张和地形纹理；中文标签由界面单独绘制，关闭时不留白色底块。对应内置路线中心坐标。
- `rail/wagon-{blue,red,green,yellow,black}-v2.webp`：五种玩家颜色的单节透明车厢（116 × 55），以路线中心为锚点并随路线旋转，覆盖已占领的印刷格位；缺失时使用内置棋子。
- `rail/train-cards.webp`：九种列车牌，9 × 1 图集。
- `rail/tickets/1.webp` 至 `30.webp`：目的地地图；中文名称和基础版分数由界面叠加。

将准备好的 WebP 文件放在临时目录，OSS 密钥分别放入已忽略的 `config/secrets/oss-access-key-id` 和 `config/secrets/oss-access-key-secret`。上传命令（值均为示例）：

```sh
python3 scripts/upload_assets.py .local/asset-pack \
  --bucket YOUR_BUCKET --endpoint YOUR_BUCKET.oss-REGION.aliyuncs.com \
  --prefix games/NEW_VERSION
```

每次使用新的版本目录；上传前检查文件完整性，上传后核对 CDN 文件，再修改部署配置。密钥、实际服务地址与图片均不要提交。素材使用长期不可变缓存，验证完成后可删除本地临时素材。
