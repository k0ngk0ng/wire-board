# 代码检查

本次涉及 TypeScript／TSX、JavaScript、Go、Python、CSS 和新增的 C 适配文件。规则定义在 `build/lint-rules.json`，由对应语言的语法／标记解析器检查，不能用文本搜索把字符串中的注释标记误判为代码注释。当前检查范围以配置中的 `targets` 为准，不代表全部历史源文件；不修改其他游戏、不可变验收归档、第三方原版源码或许可声明。

## 实现前失败场景

1. 文件开头、语句间或末尾的行注释／块注释漏检；Python 文档字符串被用来绕过限制。
2. 字符串、正则和模板文字中的 `//`、`/*` 或 `#` 被误报为注释。
3. Go、Python、TS／JS 或 CSS 语法错误被当作通过。
4. 文件命名、后缀或所属目录不符合该语言配置，却未报错。
5. 配置缺失、未知语言或配置项失效时静默跳过检查。
6. 相对路径逃出工作区，或输入通过符号链接指向工作区外。
7. 负例测试为写入违规源码而污染工作区；应通过标准输入在内存中解析，测试例存为数据。
8. 只检测固定字符串而没有调用实际语言解析器，或成功／失败退出码与诊断不一致。

文件名规则沿用本项目当前约定：Go 使用小写下划线；React 组件使用 PascalCase 的 `.tsx` 文件；其余仙剑 TS、JS、Python、CSS 文件使用小写短横线或相应既有下划线形式。目录范围和禁止注释分别在每种语言配置中声明。新增语言或新命名约束时，须扩充对应 lint 规则及负例验收。

本轮公共存档组件 `web/src/ui/saved-games.tsx` 进入检查范围，沿用公共界面既有文件名；配置仅增加该公共路径和文件名例外。公共历史文件的全部存量代码不因本次检查而自动视为已覆盖，实际范围仍以 `targets` 为准。

区域奖励验收的 `campaign_inn_e2e_test.go`、`campaign_evidence_scope_test.go` 和 `regional_fixture_e2e_test.go` 同样进入检查范围。早期夹具使用独立证据目录；目录名的穿越、分隔符、空白及超长输入通过针对性测试拒绝，默认路径继续对应已接受的剧情父档。

米行只读证明器 `rice-shop-review.py` 和本批证据验收 `river-temple-evidence-e2e.py` 也在检查范围内，沿用 Python 文件名、目录、语法及禁止注释规则。

黑水镇阶段的 `blackwater-pending-review.py` 与 `blackwater-tomb-evidence-e2e.py` 同样接入 Python 检查，分别用于待核验原始记录和本批完整证据汇总。

鬼阴山阶段的 `ghost-mountain-evidence-e2e.py` 接入同一 Python 检查，验证本批领取、旧进度内容及最终保存点清单。

扬州阶段的 `yangzhou-original-review.py` 与 `yangzhou-evidence-e2e.py` 接入同一 Python 检查，覆盖原生状态只读对照、普通领取与无物品箱子的独立审计。

蛤蟆山阶段的 `toad-original-review.py` 与 `toad-evidence-e2e.py` 接入同一 Python 检查，核对尸体、秀才条件奖励及实际钱包账目；库存等待器沿用已登记的 `campaign_caiyi_forest_e2e_test.go` 检查。

京城阶段的 `capital_economy_e2e_test.go` 接入 Go 检查，`capital-side-original-review.py` 与 `capital-side-evidence-e2e.py` 接入 Python 检查，覆盖条件资助、原版购买、菜单续档和旧证据保持；原版可重复购买不套用一次性防重要求。

京城完整支线的 `capital_quests_e2e_test.go` 接入 Go 检查，`hidden-conditional-catalog-e2e.py`、`capital-quests-original-review.py` 和 `capital-quests-evidence-e2e.py` 接入 Python 检查，覆盖道具使用条件清单、原版拒绝后续行、玉佩交还、布包实取与交付及真实法术菜单。

圣姑家与灵山阶段的 `sacred-rewards-original-review.py` 和 `sacred-rewards-evidence-e2e.py` 接入 Python 检查，区域测试沿用已登记 Go 范围，覆盖可再生采集、同点后续炼蛊皿、香蕉树帧、精灵 70 采后消失和原版台阶。

神木林与桃源阶段的 `peach_quest_e2e_test.go` 接入 Go 检查，`forest-unreachable-review.py`、`hidden-touch-catalog-e2e.py`、`forest-peach-original-review.py` 和 `forest-peach-evidence-e2e.py` 接入 Python 检查，覆盖原强制攀爬和一次运送、不可达分类、完整触碰剧情链、临时单人自动归队及旧证据保持。F 战斗诊断不加入正式进度，完整未续跑支线另留实际通过报告。

桃花林与桃源村收集阶段的 `peach-unreachable-review.py`、`peach-rewards-original-review.py` 和 `peach-rewards-evidence-e2e.py` 接入 Python 检查，区域 Go 测试及进度汇总沿用既有范围。覆盖五间民居的原门口、剧情障碍开关、当前 98 版两个傀儡虫箱实取、两条原版保留记录，以及与采集同名的正常战斗掉落独立记账。

大理与火麒麟奖励阶段的 `dali-rewards-original-review.py` 和 `dali-rewards-evidence-e2e.py` 接入 Python 检查，区域 Go 测试及进度汇总沿用既有范围。覆盖六间白苗民居、杂货铺和汉人民居的原门口、无战斗钱包账目、布靴原记录待处理，以及同核心既有火麒麟完整链证据复核；新执行与既有证据分开记账。

十年前南诏与水底迷宫阶段的 `past-rewards-original-review.py` 和 `past-rewards-evidence-e2e.py` 接入 Python 检查，区域 Go 测试、根档登记及进度汇总沿用既有范围。覆盖原钱箱与开启帧、培养室机关、真实水魔兽追逐及战斗、从原法术表读取耗气、三版实际执行源码，以及既有天蛇杖领取和交付证据复核。

盛渔村与试炼窟前批的 `trial_rewards_route_e2e_test.go`、`old_yuhang_rewards_e2e_test.go` 接入 Go 检查，`trial-rewards-plan.py`、`yuhang-trial-original-review.py`、`yuhang-trial-evidence-e2e.py` 接入 Python 检查；区域测试和进度汇总沿用既有范围。覆盖原落洞／绳索规划与真实输入、保留库存的土灵珠返回、正常傀儡虫掉落账目、全局已领箱子保持，以及姥姥实际重复对白和既有完整交换链复核。

深层试炼窟的 `trial_deep_priority_e2e_test.go` 接入 Go 检查，`trial-deep-rewards-plan.py`、`trial-deep-priority-plan.py`、`trial-deep-unreachable-review.py`、`trial-deep-original-review.py`、`trial-deep-evidence-e2e.py` 接入 Python 检查；区域测试和路由辅助沿用既有范围。覆盖两种箱体、同名箱内虫／正常掉落、菜单期间遭遇、原同点传送顺序、全部入口乐观图与原隐蛊保留证明；旧规划及归档不改写。

试炼窟底层的 `trial_bottom_e2e_test.go` 接入 Go 检查，`trial-bottom-plan.py`、`trial-bottom-original-review.py`、`trial-bottom-evidence-e2e.py` 接入 Python 检查。覆盖原菜单放置芦苇漂、错误放置保留、多阶段机关船、骑乘中续档、两条真实队伍的火神分配、完整原生法术表与旧已领箱子状态保持；进度仍按六个物品事件去重。

最终迷宫的 `final_maze_rewards_route_e2e_test.go`、`final_maze_empty_chest_e2e_test.go` 接入 Go 检查，`final-maze-rewards-plan.py`、`final-maze-unreachable-review.py`、`final-maze-original-review.py`、`final-maze-evidence-e2e.py` 接入 Python 检查；区域测试、路由辅助和进度汇总沿用既有范围。覆盖全部原入口、共享脚本独立状态、同坐标搜索优先顺序、召唤敌人的原结束奖励、真实伪箱，以及缺少独立分类证据时明确保持待处理。

锁妖塔补验的 `tower_remainder_route_e2e_test.go`、`shu_story_rewards_repeat_e2e_test.go` 接入 Go 检查，`tower-remainder-plan.py`、`tower-remainder-original-review.py`、`tower-remainder-evidence-e2e.py` 接入 Python 检查；原伪箱测试、区域测试及路由辅助沿用既有范围。覆盖独立作用域、原字节根档别名、已有剧情奖励的实际重复调查、11 个新宝箱、独立空奖励箱，以及缺少分类证据时保留待处理。

早期商贩与拾取补验的 `misc_market_fixture_e2e_test.go`、`misc_market_e2e_test.go`、`misc_pickups_e2e_test.go` 接入 Go 检查，`misc-rewards-original-review.py`、`misc-rewards-evidence-e2e.py` 接入 Python 检查。覆盖原新游戏港口根档、13 位置搜索顺序与原生实际命中、原收费前后时点、重复交易、原受伤恢复和两处地面物品。沿用的京城交易辅助和原前史源码保持原内容，归档只读快照，不以本批检查代表全部历史源码均通过。

TS／JS 使用 TypeScript 语法树及注释区间；Go 使用 `go/parser`；Python 使用 `ast` 和 `tokenize`；CSS 使用 PostCSS 语法树；C 使用 WASI Clang 的语法检查和原始词法标记。C 文件与头文件限于 `native/` 的直接文件及 `native/include/`，文件名采用小写下划线；原版第三方源码及许可保持原样。工作区路径和符号链接检查在读取源码前执行。`--stdin` 入口在内存中检查负例，不创建违规源码文件。

```sh
. scripts/env.sh
npm --prefix web run lint:pal
node games/pal/tests/lint-e2e.mjs
```

正负例报告为 `.local/pal/verification/lint-e2e.json`，当前覆盖 31 项输入，包含普通 `.js` 与 `.mjs` 文件，以及 C 注释、字符串、语法、命名和目录正负例。命令用于配置列出的仙剑改动，不代表其他游戏或全部历史文件已接入同样的 lint 规则。

剑圣探望核查的 `transient_scene_e2e_test.go` 接入 Go 检查，`transient-scene-original-review.py` 与 `transient-scene-evidence-e2e.py` 接入 Python 检查。覆盖过场当前代次方向请求拒绝、真实定时器与续档、同地图独立事件、入口续行和不足分类证据时保持原进度。

外层 JSON 解析优化的 `server/module.go`、`state_decode.go`、`state_decode_test.go` 接入 Go 检查，复用的个人存档和旧核心 HTTP 回归同样纳入当前范围；手动回退回归沿用原登记。触及文件中的说明性注释已清除，相关行为仍由原规则与验收文档约束。`tests/state-json-evidence-e2e.py` 接入 Python 检查，覆盖实际两轮性能样本、固定预算、存档回归、旧产物保持和当前范围声明。模糊测试的失败输入作为数据保留，不能用语法检查代替解析边界验收。

主菜单和战斗主指令预测的 `selection-prediction.ts` 及浏览器采样、网络观测和证据脚本已登记进对应语言检查。服务端页面夹具和原版战斗续玩继续使用已登记范围，分开保留两套实际加载的前端包与原始响应证据。

本阶段新增 `internal/server/pal_input_arrival_e2e_test.go` 和 `tests/menu-input-arrival-evidence.py` 接入语言检查；原版选择探针沿用登记范围，新增作用域只影响产物路径。既有平台文件只修改接收时间与执行接口，未将其历史代码纳入本次仙剑检查范围。

连续画面阶段登记原版战斗帧测试、画布采样脚本和证据验证器；现有浏览器夹具、选择探针和共享采样器沿用登记。平台已有代码只增改截止时钟与通知流，未将其他历史代码纳入仙剑检查范围。

并行房间阶段登记截止帧工作调度文件、真实客户端隔离 E2E 与证据聚合脚本；浏览器探针继续沿用既有语言检查范围。

战斗恢复阶段登记 `tests/battle-recovery-browser.py`、`tests/lifecycle-driver.mjs` 和 `tests/battle-recovery-evidence.py`。原版音频和战斗的浏览器夹具沿用既有登记，平台只改动模块声明和输入分发。

密集合击基线登记 `tests/dense-battle-browser.py`、`tests/dense-battle-original.mjs`、`tests/dense-battle-page.js` 及证据校验器。共享浏览器观察器和战斗 Go 夹具沿用原检查范围。

战斗素材提前准备登记原生 `battle_assets.c`、Go 观察及续档验收文件和候选证据校验器；浏览器与升级夹具沿用既有范围。

战斗帧内存复用登记 `engine/continuation.go`、`engine/dense_frame_memory_e2e_test.go` 和证据校验器；核心读取、保留版本、旧存档编解码和浏览器验收沿用原检查范围。

「风神」演出登记原版原生对照、召唤像素探针、桌面与手机浏览器脚本、保存恢复观察夹具和证据校验器。已有实时流观察器沿用原检查范围。

并行「风神」登记双会话协调器和证据校验器；原演出脚本、像素采样与规则时钟观察沿用既有范围。
