# 数据与素材来源

## 游戏规则数据

- Splendor 90 张发展卡及 10 位贵族数据，取自 [filipmlynarski/splendor-ai](https://github.com/filipmlynarski/splendor-ai) 的 `environment/cards.csv` 和 `environment/nobles.csv`。来源许可证：MIT，完整文本见 [splendor-data-LICENSE](splendor-data-LICENSE)。本项目仅使用 CSV 数据，自行实现 Go 规则引擎。
- Ticket to Ride 美国版城市坐标、路线与目的地数据，取自 [Rob217/TicketToRideAnalysis](https://github.com/Rob217/TicketToRideAnalysis) 的 `data/USA/city_locations.json`、`routes.csv`、`tickets.csv`。来源许可证：MIT，完整文本见 [rail-data-LICENSE](rail-data-LICENSE)。规则数据整合为 `internal/game/rail_data.json`；显示坐标与路段几何按 BGA 美国标准底图重新对齐。
- CATAN 基础版规则由本项目自行实现；地图拓扑、随机地形、数字与港口由 Go 生成，不连接外部游戏服务。
- Carcassonne 基础版的 72 张地块数量与规则参照 [WikiCarpedia](https://wikicarpedia.com/car/Base_game) 和 [BGA 帮助](https://en.doc.boardgamearena.com/Gamehelpcarcassonne)。Go 地块拓扑、区域连接、农民与计分由本项目自行实现。
- 数据下载日期：2026-10-02。原作者许可证的适用范围以原始文本为准。

## 视觉和声音

- 界面、大厅封面和未配置外部素材时的替代图案由本项目 SVG/CSS 实现，资源颜色对应基础版桌游含义。
- 可选外部美术来自 Board Game Arena 的 Splendor（发展卡、贵族、筹码）、Ticket to Ride（列车牌、目的地、美国地图）及 CATAN（地形、资源卡、发展卡、港口与棋子）资源。目的地卡裁切后叠加中文名称和基础版分值；美术不包含在代码仓库、Release 或镜像中，由部署者上传到独立素材目录。
- 图标来自 Lucide，ISC License（通过 npm 依赖安装，许可保存在该依赖包中）。
- 提示音通过 Web Audio 本地合成，无外部声音资源。
- 启用 `ASSETS_BASE_URL` 后，图片从所配置的素材服务器加载；游戏运行不请求 BGA。

Splendor / 璀璨宝石、Ticket to Ride / 铁路环游、CATAN / 卡坦岛及 Board Game Arena 的名称、商标和商业美术归各自权利人。本项目是独立的私人联机实现，不代表这些产品的官方服务。上述商业美术不适用本项目代码或数据的开源许可证。

CATAN 素材参考 BGA 当前第六版视觉资源（2026-10-03），按地形、卡片和透明棋子裁切为 36 个 WebP，约 258 KiB。仅存于独立素材服务器。

Carcassonne / 卡卡颂的名称、商标和商业美术归各自权利人。可选地块与随从美术参考 BGA 第二版图集（2026-10-03），裁切为 72 张地块和 5 色随从，共 77 个 WebP（约 1.74 MiB），只存于独立素材服务器，不包含在仓库、Release 或镜像中。基础版不包含修道院长，图中的花园、牲畜等装饰没有额外规则。

三国杀经典武将画像和牌面参考 [Mogara/QSanguosha-v2](https://github.com/Mogara/QSanguosha-v2)，固定提交 `e8768851bd8054db9fd1b63cd6f1feca813590d7` 的 `image/fullskin/generals/full/`（优先怀旧 `nos_`）和 `image/big-card/`。共 25 张画像、32 种牌图，经 WebP 转换后只存于独立素材服务。图片的原有美术权利不因该参考项目的代码许可而改变。本项目独立实现 Go 规则引擎，没有复制该项目的 C++/Lua 引擎源码。经典版规则和卡牌表交叉参考游卡官方卡牌介绍及标准版卡表；详见 [三国杀实现说明](sanguosha.md)。
