# 星垒 · 合作塔防

## 本次交付与玩法

1–4 人共同守护星核基地。固定路线、共同基地生命、个人金币；击杀与过波时每人获得同额奖励，无抢尾刀。每波开始前准备，所有人确认后出发；波间可一致投票暂停／恢复。服务器自动存档，断线重连恢复当前对局。沿用平台规则，24 小时无人操作的房间会关闭，暂停不是永久存档。单局可选 10／20 波闯关或无尽模式，简单／普通／困难；设置开局后固定。闯关守完指定波且基地存活即全队胜利，基地为零即全队失败；无尽不设波数胜利线，最终记录波数与团队击杀。合作局不改变平台竞争积分。

箭塔、炮塔、冰塔、法师塔：单体速射、范围爆炸、减速、魔法穿甲。可建造、升级、分支专精、出售，操作只能处置自己的塔；金币赠予队友可用于合作。波次逐渐加强敌人生命／数量，快速、重甲、成群、护盾等编队轮换，每 5 波精英、每 10 波首领；开波前公布规则。怪物数量和事件记录有界，无尽后期主要提高质量；塔满基础等级后仍可强化。

## 独立模块边界

ID `starfort`；所有实现与测试、原创 SVG、素材制作脚本归本目录。统一大厅、创建弹窗、等待房间、玩家栏与平台音效。默认下架，只有管理员显式上架才开放。卡通奇幻风的矢量插画作源素材，WebP 图集和 WAV 成品使用 `starfort/v1/` 上传 OSS；支持本地随包素材作为无 CDN 的备用。战场由 Canvas 绘制，服务器定时推进、客户端插值及特效，客户端不决定伤害和胜负。

## 联调协议 v1

选项：`{mode:"campaign"|"endless",waves:10|20,difficulty:"easy"|"normal"|"hard",prepSeconds:20|30|45}`。默认 campaign/10/normal/30。无尽 waves 仍规范化保存为 10，实际不设结束波次。

动作：`{type:"build",pad:number,tower:"arrow"|"cannon"|"frost"|"arcane"}`；`{type:"upgrade",towerId:number,branch?:"power"|"speed"}`；`{type:"sell",towerId:number}`；`{type:"ready"}`；`{type:"pause"}`；`{type:"resume"}`；`{type:"gift",target:number,amount:number}`。所有动作携带 `epoch`（只在阶段／波次变化时增加，不随仿真 tick 增加），服务端认证席位、校验当前真实余额／塔归属；同 epoch 的旧版本可重验接受。准备阶段确认可切换；全体就绪立即出波，达到设置的 20／30／45 秒准备时限后自动出波；暂停和恢复均一致投票，仅波间暂停。退出、托管由平台处理。

视图（JSON camelCase，全部公开）：
- `options`, `phase:"prepare"|"battle"|"paused"|"won"|"lost"`, `epoch`, `wave`（准备即将来袭波，战斗当前波）, `completedWaves`, `baseHP`, `baseMaxHP`, `players:[{gold,kills,damage,ready,pauseVote,resumeVote}]`, `kills`, `tick`, `serverAt`, `nextAt`（准备截止毫秒）, `elapsedMS`。
- `map:{width,height,path:[{x,y}],pads:[{id,x,y}]}` 坐标为地图逻辑像素；路径不含秘密。
- `towers:[{id,pad,owner,kind,level,branch,angle,cooldown}]`；`enemies:[{id,kind,x,y,progress,hp,maxHP,shield,speed,slowUntil}]`。
- `waveInfo:{title,description,rule,total,spawned,boss}`；`catalog:[{kind,name,cost,range,damage,interval,description}]`。
- `events:[{id,type,x,y,toX?,toY?,kind?,owner?,amount?,tick}]` 短环形记录，id 单调增加，供绘制射击／爆炸／死亡／基地受击等特效与音效。初始快照不重播旧事件。
- `controls:{canAct,canReady,canPause,canResume,builds:[{pad,kind,cost,enabled}],upgrades:[{towerId,cost,enabled,needsBranch}],sells:[{towerId,refund,enabled}]}`；观众 canAct false。

规则引擎使用 200ms 固定小步，OnTimer 大约每秒推进并输出快照。服务器停机后不追赶整段离线时间造成瞬间输局；最大每次推进 1 秒，在线持续推进不依赖网页打开。多人点击不驱动额外仿真时间。暂停无系统截止，恢复重设时钟。

## 先行验收场景

- 默认下架与后台上架；1／2／4 人正常创建、设置校验、准备开局、观众不能操作。
- 建塔越界／重复占位／金币不足／伪造玩家、升级分支、出售退款、赠予、零金币；并发占同格只成功一次；幂等重复请求不重复扣款；无效动作快照不变。
- 每种塔的战斗作用、每类怪物、护盾／减速／精英／首领，固定步推进、敌人泄漏扣基地、共同奖励、闯关全队胜利、全队失败与无尽超越 20 波。
- 存档重启恢复、暂停期间不推进、全体投票恢复、断线重连、机器人／托管、重复结算不刷分、再来一局；合作失败没有伪造赢家。
- 大厅／创建／等待／战场／结算真实浏览器，320px／390px／桌面；金币与基地生命真正可见；点击建塔与升级；敌人移动、射击、受击与减速有动画，减少动态效果；背景页不发声，静音即时生效，刷新不重播。
- SVG→WebP 成品解码、尺寸与透明通道；WAV 解码与非静音；OSS 回读 MIME／SHA256；禁止出货测试夹具。

保留 HTTP 轨迹、初始快照、完整局结果、浏览器截图和执行命令于 `.local/starfort/`。实测结果及限制见下文。

视图中的防御塔另包含服务器计算的 `effectiveRange`、`effectiveDamage`、`effectiveInterval`，用于准确显示升级后的属性。`progress` 是沿路径的距离；`tick` 与 `slowUntil` 使用 200ms 小步编号。空闲机器人返回 `nil, nil`，暂停期间没有空闲动作写库。


## 已完成验证

- 规则通过公开模块契约执行完整动作、计时和存档链路：1／2／4 人各完成 10／20 波普通闯关，双人简单／困难各完成 10 波；另有四塔混合 20 波、未建塔自然失败及无尽守过 22 波。完整局使用正常资源、正常敌潮，每房间每秒最多一个机器人动作；没有额外发钱或跳过波次。
- HTTP 验收覆盖后台上架、1／2／4 人、配置、权限、并发建同一格、幂等、计时、重启、暂停投票、空闲机器人、托管、合作胜败与战绩。合作结果不写竞争积分。
- 浏览器覆盖桌面、390px 和 320px，四人资源、创建与等待设置、建造升级出售、金币支援、一致暂停／恢复、观战、胜负与刷新。敌人动画使用实际 Canvas 精灵绘制坐标和动作帧记录验证；减少动态效果固定动作姿态。金币／基地生命文字检查真实计算颜色，达到 4.5:1。
- OSS 共 57 个媒体文件已逐个回读，核对 SHA-256、MIME 并实际解码；图集几何随前端构建，避免跨域读取 JSON。图像按普通媒体方式加载，素材失败时使用随包备用。
- 长期无尽平衡仍需玩家实玩调优；“无尽”指没有设定终止波次，不代表已验证任意时长或波次。服务器每次最多推进 1 秒，停机后不会补算整段离线时间并瞬间判负。

复现命令（在仓库根目录执行）：

```sh
. scripts/env.sh
go test -count=1 -timeout=3m ./internal/gameapi ./games/starfort/tests
npm --prefix web test -- starfort
WIRE_BOARD_STARFORT_ASSETS=https://cdn.ichenj.com/bga/v20261002-1 python3 -B games/starfort/tests/accept_ui.py
```

规则记录位于 `.local/starfort/rules/<测试名>/summary.json` 与 `replay.jsonl`；HTTP 记录在 `.local/starfort/http/`；浏览器结果、截图、动画绘制证据在 `.local/starfort/browser/`。浏览器使用专属测试服务与账户，明确标注的战斗／终局夹具只存在于测试二进制；生产模块拒绝 `scenario` 选项。素材制作及复验见 [素材说明](assets/README.md)，CDN 清单见 [发布记录](assets/published.json)。
