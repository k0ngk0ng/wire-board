# 天气负面效果与补给验收

对应许愿 `ce1065407da0539e6ceaf9781f14432c`，原文保存在 `.local/starfort-weather-v2/wish.json`。失败场景和计算口径见 [修订说明](WEATHER_V2.md)，当前数值见 [完整数据](DATA.md)。

## 验收范围

| 范围 | 可重复执行的证据 |
| --- | --- |
| 新局版本 9、版本 8 不迁移、版本与规则绑定 | `TestStarfortWeatherV2Version`，历史规则回归 |
| 三种天气对四类塔的影响及豁免 | `TestStarfortWeatherV2TowerStats`、`TestStarfortWeatherV2FrostMovement` |
| 三秒倒退／睡眠、十五秒保护、睡眠生命与护盾免伤且仍接受减速 | `TestStarfortWeatherV2Status`、`TestStarfortWeatherV2SleepDamageAndStatus` |
| 寒潮 MISS、同塔同敌人十秒保护、冰塔豁免 | `TestStarfortWeatherV2Miss` |
| 四名守卫、范围内先攻击首名守卫、巨像 7～10／精英 4～6 漏怪、守卫累计漏怪 | `TestStarfortWeatherV2BossGuards`、`TestStarfortWeatherV2Leak` |
| 矿场／瞭望塔新区间、补给成长治疗和满血转医疗包 | `TestStarfortWeatherV2ConvoyAndPositions` |
| 医疗包共享、观战／暂停权限、免费治疗、一次消耗及 SQLite 重启恢复 | `TestStarfortWeatherV2MedicalHTTP` |
| 正常资源 1～4 人 20 波、简单／困难及无尽超过 20 波、真人托管与接管 | 既有 `TestStarfortWeatherNaturalCampaign`、`TestStarfortWeatherNaturalModes`、`TestStarfortWeatherPadsAndMixedAutoplay` 使用当前生产新局规则 |
| 桌面／320px 触屏、全屏、顶部 10% 天气、不拦截输入、实际 Canvas ZZZ、减少动态效果、医疗包及刷新 | `accept_weather_v2_ui.py` 的 JSON、绘制坐标和截图 |

规则验收调用公开模块、动作、计时与存档边界；HTTP 验收使用真实服务和 SQLite。固定位置、随机数及视觉场景属于明确标记的边界夹具，不代表自然完整对局。手机验收使用 Chromium 原生触屏仿真，不代表手机真机或 Safari 验证。

## 复验命令

在仓库根目录执行：

```sh
. scripts/env.sh
go test -count=1 -timeout=20m ./games/starfort/server ./games/starfort/tests
go test -race -count=1 -timeout=5m ./games/starfort/server
go test -race -count=1 -timeout=5m -run '^TestStarfortWeatherV2' ./games/starfort/tests
npm --prefix web test -- starfort
npm --prefix web run build
python3 -B games/starfort/tests/accept_weather_v2_ui.py
```

规则重放与结果位于 `.local/starfort/rules/<测试名>/replay.jsonl`、`summary.json`，HTTP 结果沿用星垒 HTTP 验收目录。此次运行日志、先行失败基线、浏览器 `result.json`、ZZZ 绘制坐标、视区测量和截图统一在 `.local/starfort-weather-v2/`。发布收据另绑定正式提交、同一次 CI 构建、本地 CDN 回读、版本及服务器状态。
