# 天气与商人版本验收

对应许愿：`3d0d9560e2e5c3b84ce7d4302e1cc412`。需求原文保存在 `.local/starfort-weather/wish.json`，计算口径与实现前失败场景见 [修订说明](WEATHER.md)，当前数值见 [完整数据](DATA.md)。此次同时调整管理后台的许愿列表。

## 覆盖范围

| 要求 | 可核验的证据 |
| --- | --- |
| 新局版本 8，版本 1～7 不迁移，查看与恢复不重抽 | `TestStarfortWeatherVersionAndForecast`、历史规则全量回归、HTTP／SQLite 恢复 |
| 四种天气，波前提示，战斗效果限定顶部 10%，不挡操作，支持减少动态效果 | `accept_weather_ui.py` 的桌面／320px 几何、原生操作与动画检查及截图 |
| 售塔确认、取消不扣塔、确认只出售一次 | 同一浏览器验收的桌面鼠标／手机触屏场景 |
| 精英扣血 4～6、星陨巨像 8～10，按生命比例计算；商店巨像免伤 | `TestStarfortWeatherLeak`、`TestStarfortWeatherMerchantGateAndWard` |
| 每路六点、一个默认高地、沿道路分布，扩图保留原点位 | `TestCooperativeExpansionWithCurrentSkills`、`TestStarfortWeatherPadsAndMixedAutoplay/even-road-pads`；塔位在对应道路间隔内避让交叉口和已有塔位 |
| 10／20／30 级后的升级费用，报价与实际扣费一致 | `TestStarfortWeatherUpgradeCosts` |
| 全任务共用 25% 生成概率，商人第 10 波起出现，位置／生命／费用／奖励正确，未承接不计次 | `TestStarfortWeatherTaskLottery`、任务承接与恢复检查 |
| 任务仅受同路普通敌人的一次攻击，跨步经过不漏判，承接路线生命 ×1.1 | `TestStarfortWeatherTaskDamageRefund`、`TestStarfortWeatherTaskRouteAndHealth` |
| 矿场满 40 点改为移动团队塔，保留塔属性；瞭望塔临时光环及交替永久成长 | `TestStarfortWeatherBeaconAndMineRewards` |
| 护送到达前不领奖、不过波；成功过波才开店，最终波先开放商店再结算 | `TestStarfortWeatherEscortMustArrive`、`TestStarfortWeatherShopHTTP`、`TestStarfortWeatherFinalMerchantShop` |
| 商店四类商品、全队独立额度、本人塔限制、暂停／观战拒绝、重启不重复购买 | `TestStarfortWeatherShopHTTP` 与浏览器购买／重载检查 |
| 冰封天气时长、冷却、恢复与逐塔保护 | `TestStarfortWeatherFreezeAndMeteor`、`TestStarfortWeatherCraterAndFreezeProtection` |
| 威力炮成长、阻滞炮十级工作量、重型填坑时间与填坑额外伤害 | `TestStarfortWeatherCombatGrowth`、`TestStarfortWeatherCraterAndFreezeProtection` |
| 箭雨成长、独立击退、薄雾加成、残废箭剩余生命伤害与逐塔免疫 | `TestStarfortWeatherCombatGrowth`、`TestStarfortWeatherHeartAndGuards`、`TestStarfortWeatherFogHeart` |
| 法师护甲倍率、星辉加成、流星雨范围、重型额外伤害、沉默及逐塔保护 | `TestStarfortWeatherArcaneArmor`、`TestStarfortWeatherFreezeAndMeteor`、`TestStarfortWeatherMeteorBoundsAndRestore` |
| 动态技能说明、静止鼠标逐帧稳定、手机固定阅读、全屏可操作 | 浏览器逐帧记录 `skill-stability-1366.json`、`skill-stability-320.json` 与截图 |
| 正常资源 1／2／3／4 人完整战役、简单／困难、无尽，以及真人与显式托管接管 | `TestStarfortWeatherNaturalCampaign`、`TestStarfortWeatherNaturalModes`、`TestStarfortWeatherPadsAndMixedAutoplay` |
| 许愿管理标准列表、前 80 字符摘要、详情浮层保留全文／图片／问答／备注及审核流程 | `scripts/accept_wishes.py`，桌面与 320px 布局、焦点恢复及审核状态流转 |

## 重复执行

在仓库根目录执行，所有缓存与产物留在工作区内：

```sh
. scripts/env.sh
go test -count=1 -timeout=25m ./games/starfort/server ./games/starfort/tests
go test -race -count=1 -timeout=8m ./games/starfort/server
go test -race -count=1 -timeout=5m -run '^TestStarfortWeather(ShopHTTP|TaskDamageRefund|BeaconAndMineRewards|TaskRouteAndHealth|CraterAndFreezeProtection|MerchantGateAndWard|FinalMerchantShop|PadsAndMixedAutoplay|MeteorBoundsAndRestore)$' ./games/starfort/tests
npm --prefix web test -- starfort
npm --prefix web run build
python3 -B games/starfort/tests/accept_weather_ui.py --skip-build
python3 -B scripts/accept_wishes.py
python3 -B scripts/check_game_boundaries.py
go vet ./games/starfort/server ./games/starfort/tests ./scripts/fixtures/wishes
```

模块验收逐步通过生产 `Module`、计时器与序列化存档；HTTP 验收使用真实会话与 SQLite。固定种子、位置和波次均明确标记为边界夹具，不作为自然完整对局证据。浏览器使用生产前端与真实 HTTP 服务，手机为 Chromium 触屏仿真，不代表实体手机或 Safari。

## 当前记录

完整服务端／规则／历史存档回归通过；1／2／3／4 人普通战役、简单／困难与 22 波无尽自然对局通过。星垒浏览器 76 项、许愿管理浏览器 73 项、前端 66 项，以及服务端和新增边界竞态检查均通过。结果与源码摘要以 `.local/starfort-weather/validation.json` 为准；模块重放在 `.local/starfort/rules/TestStarfortWeather*/`，HTTP 记录在 `.local/starfort/http/TestStarfortWeather*.json`。

本文件记录本地验收结果；正式版本、部署与线上许愿状态以对应发布记录为准。
