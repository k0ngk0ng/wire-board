# 专精与全战场天气验收

对应许愿 `146b22bbe3f10154f1d661d9690ffd1a`。失败场景、成长循环与睡眠计算口径见 [修订说明](WEATHER_V3.md)，当前数据入口见 [完整数据](DATA.md)。

| 范围 | 可重复执行的证据 |
| --- | --- |
| 新局版本 10、历史版本 8／9 隔离、版本绑定及只读恢复 | `TestStarfortWeatherV3Version`，历史规则全量回归 |
| 威力炮范围伤害、阻滞炮 5 倍数等级、概率和冷却边界 | `TestStarfortWeatherV3AreaGrowth` |
| 箭雨范围、伤害、额外目标、独立击退与精英／首领减半 | `TestStarfortWeatherV3AreaGrowth`、`TestStarfortWeatherV3Knockback` |
| 冰封成长、寒潮增益、重型完全生效、减速与时长成长 | `TestStarfortWeatherV3FreezeAndSlow` |
| 残废剩余生命伤害、逐塔保护、薄雾与法师护甲倍率、流星雨 | `TestStarfortWeatherV3HeartAndMagic` |
| 星辉睡眠三秒、十秒保护、半伤、减速／冰封／沉默翻倍 | `TestStarfortWeatherV3Sleep` |
| 四任务共用 40% 概率、精确阈值、商人波数及读取不重抽 | `TestStarfortWeatherV3TaskLottery` |
| 实际操作及 SQLite 重启后保留新规则、专精成长和攻击优先级 | `TestStarfortWeatherV3HTTP` |
| 正常资源 1～4 人 20 波、简单／困难及无尽 22 波、混合真人托管／接管 | `TestStarfortWeatherNaturalCampaign`、`TestStarfortWeatherNaturalModes`、`TestStarfortWeatherPadsAndMixedAutoplay` |
| 桌面／320px 触屏、全屏、全天气覆盖、不拦截输入、实际塔身与技能说明、逐帧悬停稳定性、Canvas ZZZ 及减少动态效果 | `accept_weather_v3_ui.py` 的结果、坐标、逐帧记录和截图 |

固定位置、随机数及高生命敌人属于边界夹具，不替代自然对局。浏览器验收使用 Chromium 原生触屏仿真，不代表实体手机或 Safari。

## 复验命令

在仓库根目录执行：

```sh
. scripts/env.sh
go test -count=1 -timeout=25m ./games/starfort/server ./games/starfort/tests
go test -race -count=1 -timeout=6m ./games/starfort/server
go test -race -count=1 -timeout=6m -run '^TestStarfortWeatherV3' ./games/starfort/tests
npm --prefix web test -- starfort
npm --prefix web run build
python3 -B games/starfort/tests/accept_weather_v3_ui.py --skip-build
python3 -B scripts/check_game_boundaries.py
```

本次证据位于 `.local/starfort-weather-v3/`，包含实现前失败基线、规则／竞态与前端检查日志、浏览器 `result.json`、逐帧坐标和截图。公开模块的动作重放和最终状态位于 `.local/starfort/rules/<测试名>/`。发布收据另绑定正式提交、同次 CI 构建、本地 CDN 回读、版本、镜像及服务器检查。

## 本次本地结果

- 新规则、HTTP／SQLite、服务端竞态与新规则竞态检查通过；前端 66 项检查、生产构建和游戏边界检查通过。
- 正常资源 1～4 人普通难度各守住 20 波，简单／困难通关，无尽守过 22 波；历史完整对局、40 波无尽及其余规则回归通过。
- 全量运行发现历史 25% 任务概率测试使用了当前新局入口。该测试已显式固定版本 8，单独复验；新版本另有 40% 精确阈值与四任务抽样检查。
- 浏览器 74 项检查通过：桌面、320px 原生触屏仿真、普通页面／全屏、四塔动态说明与逐帧悬停、实际塔身点击、睡眠 ZZZ 和减少动态效果。房间切换等待对应标题与画布就绪，点击前等待全屏按钮位置稳定，避免异步页面滚动影响坐标。
