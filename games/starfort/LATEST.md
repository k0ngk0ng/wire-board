# 星垒生命、逐路奖励与灾害调整

来源：许愿 `a364f0ebcf8f617c445080bd8b3d0b28`，正文修订 0。新局使用存档版本 13、`revisionRules=2`；版本 1～12 保留原地图、数值及随机序列。

## 实现前失败场景

- 生命上限在第 10／20 波边界漏增、提前增或重复增；查看、暂停、重连和恢复触发回血；旧局上限被改动；损坏存档绕过校验。
- 奖励仍按全图最远道路计算，单路两档叠加，未出兵道路白领奖励，越界或最终波重复结算。
- 灾害／任务每种各抽 70%，查看重抽；地震坑少于 1 或多于 5、互相重叠；弹坑工作量反复增加；地震保底停塔数超出 1～2 或恢复时漏守下限。
- 湿润五秒期限丢失，冰封额外两秒漏算、睡眠倍增顺序错误、已过期仍有增益；减速未增加 50 个百分点或越过最低速度；洪水伤害重复、恢复非线性、十秒后仍有紫圈。
- 初始塔位仍两侧交替，向内选择后压道路／重叠／越界，少于每路六点或多于一个高地；后续地图或旧局被重排。
- 无敌报价 700 而扣 1000，金币不足仍可买、重复堆叠、使用／暂停／恢复改变十秒期限；商人费用仍按 20% 成长，失败退钱或未送达开放商店。
- 薄雾箭雨目标、击退 +20、残废 ×1.3 不一致；星辉流星雨额外 15 个百分点漏算；法师威力／魔法攻速与魔法伤害倍率错误；护甲护盾额外伤害叠乘；说明和战斗结果不同。
- 紫圈遮挡塔或道路、手机不可见、缩放后错位；新规则关闭原有天气／奖励界面；电脑／真人托管或自然通关失效。

## 计算口径

- 星核最大生命为 `30 + floor(当前波/10)`，第 10 波整备起生效；增加上限不自动回复当前生命。查看和恢复不改变生命。
- 每条实际出兵道路分别按历史最远进度取 `A×10`（不超过 50%）、`A×5`（不超过 70%）或零，再求和发给每位玩家。召唤与死亡不清除历史进度，没有出兵记录的道路不计奖励。
- 自然灾害与任务各做一次共用 70% 判定。地震 1～5 坑；区域内新炮塔弹坑工作量乘 1.8。地震涉及塔按道路计算 10% 保底，向上取整后限制在 1～2 座；每次五秒恢复一座，最多三次，保底塔于波末恢复。
- 湿润仍持续接触后五秒；新冰封额外两秒，攻速冰塔额外减速 50 个百分点，沿用最低 10% 移速。洪水塔伤害从 70% 在十秒内线性恢复到 100%；恢复期间塔基显示紫色魔法光圈，随恢复消退。
- 初始每路六点、一个高地。以道路等距位置的局部法线选择靠近星核的一侧；共享／交叉段优先避开所有道路、其他塔位和地图边界。保存地图后不重排。
- 无敌道具 700 金币，团队最多持有一份，用完才可再买，持续十秒；商人任务从 200 金币开始，每次承接后乘 1.1，四舍五入，失败不退款。
- 薄雾箭雨额外目标仍加一，击退由 50 增至 70；残废本塔伤害乘 1.3。星辉法师免疫负面攻速，威力伤害仍乘 1.2；流星雨触发由 20% 增至 35%。法师威力攻速乘 0.6；魔法伤害乘 0.8、攻速乘 0.6；其余护甲、护盾、成长和控制规则沿用。

## 可重复验收

先编写 `latest_e2e_test.go` 和界面验收，再改实现。规则通过公开 Module、序列化、定时器与 HTTP／SQLite 验证；完整局覆盖 1～4 人和全部难度。浏览器覆盖桌面、320px 触屏及全屏，保留截图、实际 Canvas 绘制数据、操作结果和源码摘要。记录位于 `.local/starfort-latest/`。


```sh
. scripts/env.sh
go test -count=1 -json -run '^TestStarfortLatest' ./games/starfort/tests
go test -race -count=1 -run '^TestStarfortLatestHTTP$|^TestStarfortRevisionHTTP$' ./games/starfort/tests
go test -count=1 -timeout=45m ./games/starfort/server ./games/starfort/tests
npm --prefix web test -- starfort
npm --prefix web run build
python3 -B games/starfort/tests/accept_latest_ui.py --skip-build
```

| 需求 | 验收依据 |
| --- | --- |
| 新旧存档、生命上限和不自动回血 | `TestStarfortLatestVersionAndHealth`、`TestStarfortLatestHTTP` |
| 逐路最高档、无出兵道路、单次发奖 | `TestStarfortLatestPerRouteRewards` |
| 两个共用 70% 判定、1～5 地震坑、商人成长 | `TestStarfortLatestDisastersAndTasks` 的 400 波统计 |
| 地震弹坑、湿润、洪水恢复 | `TestStarfortLatestWetAndQuake`、`TestStarfortLatestFloodRecovery` |
| 地震停塔与恢复下限 | `TestStarfortLatestQuakeRecoveryFloor` |
| 700 金币报价、余额校验、扣款和专精／天气 | `TestStarfortLatestShopAndWeather` |
| 六个塔位、单个高地、朝内、无重叠 | `TestStarfortLatestInwardPads` |
| 托管、手动接管、SQLite 恢复 | `TestStarfortLatestHTTP` |
| 1～4 人与全部难度自然通关 | `TestStarfortLatestNatural` 的六组正常资源对局 |
| 桌面／320px、全屏、紫圈、真实塔身点击 | `accept_latest_ui.py` 的真实浏览器产物 |
