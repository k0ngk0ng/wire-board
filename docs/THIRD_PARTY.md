# 数据与素材来源

## 游戏规则数据

- Splendor 90 张发展卡及 10 位贵族数据，取自 [filipmlynarski/splendor-ai](https://github.com/filipmlynarski/splendor-ai) 的 `environment/cards.csv` 和 `environment/nobles.csv`。来源许可证：MIT，完整文本见 [splendor-data-LICENSE](splendor-data-LICENSE)。本项目仅使用 CSV 数据，自行实现 Go 规则引擎。
- Ticket to Ride 美国版城市坐标、路线与目的地数据，取自 [Rob217/TicketToRideAnalysis](https://github.com/Rob217/TicketToRideAnalysis) 的 `data/USA/city_locations.json`、`routes.csv`、`tickets.csv`。来源许可证：MIT，完整文本见 [rail-data-LICENSE](rail-data-LICENSE)。坐标转换为 SVG 画布坐标，整合为 `internal/game/rail_data.json`；未使用来源仓库内的商业地图图片。
- 数据下载日期：2026-10-02。原作者许可证的适用范围以原始文本为准。

## 视觉和声音

- 界面、宝石符号、卡牌装饰、火车、大厅封面、简化地图底纹均为本项目 SVG/CSS 实现，资源颜色对应基础版桌游含义。
- 图标来自 Lucide，ISC License（通过 npm 依赖安装，许可保存在该依赖包中）。
- 提示音通过 Web Audio 本地合成，无外部声音资源。
- 所有运行时资源由本服务提供，无外部字体、图片 CDN 或 BGA 依赖。

Splendor / 璀璨宝石、Ticket to Ride / 铁路环游及 Board Game Arena 的名称、商标和商业美术归各自权利人。本项目是独立的私人联机实现，不代表这些产品的官方服务，也没有下载或再分发 BGA 的商业美术素材。未来如替换为原版图片，应使用有适当许可的资源。
