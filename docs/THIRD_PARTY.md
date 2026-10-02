# 数据与素材来源

## 游戏规则数据

- Splendor 90 张发展卡及 10 位贵族数据，取自 [filipmlynarski/splendor-ai](https://github.com/filipmlynarski/splendor-ai) 的 `environment/cards.csv` 和 `environment/nobles.csv`。来源许可证：MIT，完整文本见 [splendor-data-LICENSE](splendor-data-LICENSE)。本项目仅使用 CSV 数据，自行实现 Go 规则引擎。
- Ticket to Ride 美国版城市坐标、路线与目的地数据，取自 [Rob217/TicketToRideAnalysis](https://github.com/Rob217/TicketToRideAnalysis) 的 `data/USA/city_locations.json`、`routes.csv`、`tickets.csv`。来源许可证：MIT，完整文本见 [rail-data-LICENSE](rail-data-LICENSE)。规则数据整合为 `internal/game/rail_data.json`；显示坐标与路段几何按 BGA 美国标准底图重新对齐。
- 数据下载日期：2026-10-02。原作者许可证的适用范围以原始文本为准。

## 视觉和声音

- 界面、大厅封面和未配置外部素材时的替代图案由本项目 SVG/CSS 实现，资源颜色对应基础版桌游含义。
- 可选外部美术来自 Board Game Arena 的 Splendor（发展卡、贵族、筹码）和 Ticket to Ride（列车牌、目的地、美国地图）资源。目的地卡裁切后叠加中文名称和基础版分值；美术不包含在代码仓库、Release 或镜像中，由部署者上传到独立素材目录。
- 图标来自 Lucide，ISC License（通过 npm 依赖安装，许可保存在该依赖包中）。
- 提示音通过 Web Audio 本地合成，无外部声音资源。
- 启用 `ASSETS_BASE_URL` 后，图片从所配置的素材服务器加载；游戏运行不请求 BGA。

Splendor / 璀璨宝石、Ticket to Ride / 铁路环游及 Board Game Arena 的名称、商标和商业美术归各自权利人。本项目是独立的私人联机实现，不代表这些产品的官方服务。上述商业美术不适用本项目代码或数据的开源许可证。
