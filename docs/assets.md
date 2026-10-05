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

- `catan/terrain-{wood,brick,wool,grain,ore,desert}-v1.webp`：六类六边形地形，原版资源颜色。
- `catan/resource-{wood,brick,wool,grain,ore}-v1.webp`：五种完整资源卡；`icon-{wood,brick,wool,grain,ore}-v1.webp` 为费用、交易和银行使用的圆形图标。
- `catan/dev-{0,1,2,3,4}-v1.webp`：依次为骑士、道路建设、丰收、垄断、胜利点。
- `catan/{settlement,city}-{blue,red,white,orange}-v1.webp`：四种基础玩家颜色的透明村庄与城市。五至六人扩充另有 `purple` 与 `green` 两色，保留同一棋子轮廓与明暗，由 `scripts/prepare_catan_player_colors.py` 生成。
- `catan/helpers/helper-{1..12}.webp`：官方 Helpers 十二位人物图；可由 `scripts/prepare_catan_helper_assets.py` 从固定版本规则书重现提取。
- `catan/port-{wood,brick,wool,grain,ore,any}-v1.webp` 与 `catan/robber-v1.webp`：港口船与强盗。

- `carcassonne/tile-{0..71}-v1.webp`：72 张基础版地块（256 × 256），与规则数据中的 `art` 对应，保留原版装饰差异；`meeple-{0..4}-v1.webp` 为蓝、绿、黑、红、黄五色随从。

- `sanguosha/v1/generals/{id}.webp` 与 `sanguosha/v1/cards/{kind}.webp`：25 张武将画像和 32 种卡面，准备方式见 `docs/sanguosha.md`。

将准备好的 WebP 文件放在临时目录，OSS 密钥分别放入已忽略的 `config/secrets/oss-access-key-id` 和 `config/secrets/oss-access-key-secret`。上传命令（值均为示例）：

```sh
python3 scripts/upload_assets.py .local/asset-pack \
  --bucket YOUR_BUCKET --endpoint YOUR_BUCKET.oss-REGION.aliyuncs.com \
  --prefix games/NEW_VERSION
```

每次使用新的版本目录；上传前检查文件完整性，上传后核对 CDN 文件，再修改部署配置。密钥、实际服务地址与图片均不要提交。素材使用长期不可变缓存，验证完成后可删除本地临时素材。

补充新游戏素材时，上传目录必须以部署中实际的 `ASSETS_BASE_URL` 路径为准，包含其中已有的版本目录。例如基地址为 `https://assets.example.com/games/version`，三国杀的上传前缀应为 `games/version/sanguosha/v1`。不要改动共用基地址来修复单个游戏，否则会影响其他游戏。验证时使用与线上相同的基地址打开游戏，确认图片元素已解码显示；只验证另一个可访问的图片目录不足以证明页面可用。

铁路新增地图使用 `rail/maps/v1/{europe,india,switzerland,nordiccountries,legendaryasia}/`：每个目录有 `map.webp`、`preview.webp` 和 `tickets/{id}.webp`。欧洲另有 `station-{blue,red,green,yellow,black}.webp`。共 247 个文件；列车牌与车厢沿用 `rail/` 现有素材。印度、北欧底图为 1125×1744，目的地卡为 161×250；其余底图为 1744×1125，目的地卡为 250×161，显示时保持比例。

准备方式：`scripts/prepare_rail_map_assets.py SOURCE_IMG_DIR OUTPUT_DIR`（依赖 Pillow、OpenCV、NumPy）。城市文字轮廓与白边从底图移除，中文由前端绘制；地理装饰文字保留。`scripts/rail_map_labels.json` 同时记录文字区域和显示位置。上传时将输出目录对应到部署素材基地址下的 `rail/maps/v1`，不可遗漏既有版本前缀。图片不写入 Git，密钥不写入脚本。


卡坦事件插画使用 `catan/events/{kind}-v1.webp`，包括 `beautiful_day`、`calm_seas`、`conflict`、`earthquake`、`epidemic`、`good_neighbors`、`helpful_neighbor`、`new_year`、`plentiful_year`、`robber_attacks`、`robber_flees`、`tournament`、`trade_advantage`；另有 `catan/events/back-v1.webp`。13张事件插画原生为203–208×172–177，牌背173×247，均保留原比例，不拉伸或放大。它们是官方插画和牌背，**不是带生产点数的完整36张实体牌面**；点数、中文事件文字须由核验后的游戏数据与界面独立呈现。

`scripts/prepare_catan_event_assets.py OUTPUT_DIR --rules-directory RULES_DIR` 从固定SHA256的2025 T&B规则书第4–6页提取；省略规则目录时在内存下载同一固定文件。依赖PyMuPDF、Pillow。PDF内部图片顺序与正文顺序不同，使用经逐项视觉核对的对象ID映射；检查来源哈希、对象所在页及原生尺寸，输出无损WebP与来源清单。14张合计704,114字节。可复现的对象/尺寸/路径/文件哈希见`docs/research/catan-event-art-sources.json`。上传时保持既有素材基地址，用上述新文件路径；更新画面需提升文件版本，避免覆盖不可变缓存。该批素材准备不代表事件扩展已开放或完整牌表已核验。
