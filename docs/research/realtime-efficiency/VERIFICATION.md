# 实时效率、泡泡堂地形与人物验收

已完成实时传输、内存检查点、本机运动显示和断线恢复，并落实本轮追加的树木碰撞、人物选择、固定红蓝队色和禁止人物数字角标要求。此前疯狂坦克重构的实现与原版资料保留；其验收见 `docs/research/fortressblue-refactor/verification.md`。

## 对照清单

| 要求 | 当前行为与证据 |
| --- | --- |
| 不按移动逐条写数据库 | 普通输入与物理帧在内存中推进；每秒保存有变化的房间，关键操作立即保存。4 房间、32 个真实登录席位、2 秒 320 次输入，快照 UPDATE 从 400 降至 8、动作日志新增 0、错误 0。原始数据见 `load-stream-4.json`，范围是平台计数模块，不作实际游戏容量承诺。 |
| 高频收发不重复完整状态 | 双向 WebSocket 直接发送输入、确认和当前游戏快照，大厅／配置／聊天不进入热路径；浏览器五游戏原生输入期间状态 GET 和动作 HTTP POST 均为 0。原 HTTP 与通知通道保留兼容。 |
| 不用空闲倒计时托管 | 清除实时模块操作期限及旧超时托管；保留主动托管、电脑、开局／选择／比赛时限。65 秒真实空闲验收无接管；持续输入不推迟系统帧。 |
| 弱网与断线恢复 | 50／150／300 ms RTT 验收通过。五游戏在真实 TCP WebSocket 被关闭后不再续发旧按键，重新按键均获确认；俄罗斯方块和街霸方块的队列序号下界随最新权威状态推进。 |
| 慢用户不阻塞别人 | 不读取 4 MiB WebSocket 数据的真实客户端不阻塞其他用户，另一请求约 2.731 ms、12 次行动成功，堵塞订阅按时清理。HTTP 慢读隔离另有独立证据。 |
| 本机响应与服务器校正 | 泡泡堂、坦克大战最多预测 250 ms，仅改善位置显示；可见障碍约束和权威校正保留。PC 键盘与真实 320px 触屏均在服务器位置尚未变化时出现实际绘制位移。泡泡堂旧规则 0 不套用连续运动预测。 |
| 人物位置与放泡格一致 | 八名真人房间中用浏览器实际控制第 3 席：走到杉树右侧停在 `px=4.78`，稳定绘制点与服务器误差小于 0.015 格；进入可穿灌木后，放泡落在实际所在格。角色精灵不再因棋盘边界钳制而与脚下位置脱离。 |
| 树不能当空地 | 官方图复核只在 `y=6,x=0/2/4/10/12/14` 有六棵杉树；第 5／9 纵列是果实灌木。原包 TileInfo 将 Tree 与房屋归为类型 1，Herb 为类型 3。新局高树阻挡移动和水柱，灌木可进入并遮挡部分人物；旧局不在玩家脚下补墙，缺少碰撞位的旧树改画灌木。 |
| 多套人物、队色不变 | 从 2002 原包恢复四套人物，提供泡泡帽、飞行帽、熊耳帽、条纹帽选择。外观与队伍分离，红蓝各有四套，包含四方向、困泡和死亡。选择保存到席位；2／4／6／8 人、刷新、重启和托管均通过。 |
| 无人物数字角标 | AGENTS 已写明禁止默认身份数字角标；泡泡堂标号、赛车悬浮编号、坦克回退编号、大富翁图像回退数字及疯狂坦克缺名时的 P 编号回退已清理。规则所需资源、倒计时、排名仍正常显示。 |

## 追加问题的原版依据

参考分为三类，不能混用：官方截图用于小区 10 的视觉位置；2002 客户端对象表／TileInfo 用于树和灌木的原始类型区分；原包人物图集及 ObjectTable 用于人物分层、矩形和帧顺序。2002 `Village09` 不等于现有小区 10，未用它替换当前地图。原版运行中的精确碰撞半径、时间单位和定位字段公式仍未证明，当前参数与接入约定在契约及素材清单明确记录。

人物素材生成器校验原包 SHA，完整解析 RGB565 RLE，输出 48 张红蓝图集，共 202,874 字节。Static 先铺底、着色 Adjust 后覆盖，避免 C00 右向的重叠 Static 红衣使蓝队变红。原包完整分层核查覆盖 32 组、176 帧；发布文件另逐文件验 SHA、矩形、透明度和红蓝变换。泡泡堂 v2/v3 与疯狂坦克 v3 共 96 个对象已上传至不可变路径，OSS 与 CDN 的完整字节校验均通过，详见 `cdn-assets-final.json`。前端包已核验 82 份 v2/v3 本地回退文件。

## 本地验收产物

本阶段结果位于 `.local/realtime-efficiency/`：

- `tree4e884f/report.json`：原生人物选择、真实杉树碰撞／灌木通行、放泡落格、蓝队四种选择以及右向／困泡／死亡。记录 PC 和 320px 弹窗实际矩形、可见性、点击命中、最终背景与文字颜色。
- `browser-avatar-result-focus-final/result.json`：完整泡泡堂交互、困泡／救援期限、死亡刷新不重播、结算浮层及选择窗关闭后的焦点交接、八人布局、电脑／接管、三张自定义图、音效静音、v2/v3 素材失败回退。
- `browser-final-motion/report.json`：最终人物和地形下的 300 ms 键盘／320px 原生触屏预测、松手先于 ACK、普通流式输入。
- `browser-reconnect-all/report.json`：五款游戏实际关闭服务器侧 TCP 连接，验证旧按键清除和新按键恢复。故障入口只存在于显式开启的本地测试夹具，不编译进生产服务。
- `slow-websocket-reader.json`、`slow-http-reader.json`：真实慢读连接隔离。
- `avatar-bubble-race.txt`、`avatar-map-matrix.txt`：泡泡堂规则／公开 HTTP／完整对局竞态与全地图矩阵。
- `final-frontend.txt`：公共前端及六款受影响游戏共 173 项；疯狂坦克另有 `no-avatar-number-fortress-front.txt` 的 23 项。
- `final-build.txt`、`final-vet.txt`、`final-boundaries.txt`、`final-packaged-assets.txt`、`final-fortress-assets.txt`：编译、静态检查、24 游戏边界及打包素材校验。
- 检查点失败重试、关键事务失败回滚、真实 SIGKILL 恢复、版本回退与旧实例控制拒绝，详见 [CHECKPOINT-RESULTS.md](CHECKPOINT-RESULTS.md)。

150 ms 最终五游戏汇总中，街霸方块新按键恰好进入落地结算阶段而合法返回 409；修正验收等待条件后，`browser-final-rtt150-spf2t/report.json` 单游戏复验通过。该汇总的失败结果未改写为通过，其他四游戏证据仍保留。

中途的失败均已修正并复验：错误地把纵向灌木当高树；截图工具改变 320px 视口；边缘角色图片钳制；人物选择关闭后空格重新打开弹窗；街霸方块断线续发旧方向及重用序号。最终结果不把这些失败运行算作通过。

## 复跑与发布

先执行 `. scripts/env.sh`，再运行按范围选择的命令：

```sh
python3 -B scripts/bubbleparty/verify-assets.py --dist
python3 -B scripts/bubbleparty/check-terrain.py
python3 -B scripts/bubbleparty/check-refactor.py
python3 -B scripts/realtime/check-browser.py --games bubbleparty,battlecity --rtt-ms 300 --prediction --idle-seconds 1
python3 -B scripts/realtime/check-browser.py --rtt-ms 300 --reconnect --idle-seconds 0
go test -race ./games/bubbleparty/server ./games/bubbleparty/tests -count=1
go test ./games/bubbleparty/tests -run '^TestBotMatchesAllMaps$' -count=1
go test -race ./internal/server -run '^(TestModuleRealtime|TestRealtime)' -count=1
```

发布沿用用户已授权的流程：同一提交先通过 main 的增量检查与 Linux 构建，再创建版本 tag；tag 工作流复用该提交的已验证二进制，发布 GitHub Release 和同 SHA 镜像。生产用既有备份／回滚脚本更新，核对版本、健康及上下架设置后，仅清理指定旧备份和围桌历史镜像。正式运行编号、提交和生产结果记录在 GitHub Release 及 `.local/realtime-efficiency/release-result.json`。
