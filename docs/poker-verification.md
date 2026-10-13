# 扑克游戏验收记录

三款独立游戏已接入统一大厅、创建、等待和玩家栏。牌型、状态与机器人分别在各游戏目录维护；共享部分只有呈现、素材、手势、偏好和测试会话工具。

## 已完成的本地验证

| 验证 | 范围与结果 | 可核验产物 |
| --- | --- | --- |
| 斗地主 | 三人／四人、个人加倍开／关的自然多副对局；四王大于八炸、组合限制、叫分、隐私、零和结算、重复 nonce、SQLite 恢复、再来一局 | `.local/poker/doudizhu/`，`.local/doudizhu-coalition-verified.log` |
| 掼蛋 | 自然有限副数及从 2 到过 A；贡还、接风、百搭解释、明确双贡夹具、私密还贡／牌型预览、独立时限与恢复 | `.local/poker/guandan/`，`.local/guandan-preview-verified.log` |
| 升级 | 单双副有限副数及从 2 到过 A；亮主反主、取底扣底、结构跟牌、甩牌失败、计分分档、抠底、换庄和 HTTP 自然整局 | `.local/poker/shengji/`，`.local/shengji-http.log` |
| 真实浏览器 | 40 个截图检查点通过；五种模式、所有席位、1280／390／320px、地主及取底 33 张、鼠标与原生触屏多选、皮肤保留选牌、实际出牌／预览／扣底、重载无旧动效、统一静音及最终计算样式对比度 | `.local/poker/browser/result.json`，同目录截图与 `*-contrast.json` |
| 素材发布 | 162 张牌面、3 款牌背、3 款原创封面、4 个音效，共 172 个 OSS 文件逐个回读；校验和、MIME 与格式签名通过 | `assets/poker/manifest.json`，`.local/poker/oss-readback.json` |
| 平台回归 | 306 项前端测试通过（含延迟事件与重连反馈过滤）；平台模块生命周期、管理和合作模式针对性回归通过；`go vet ./...`、生产构建与依赖边界检查通过 | `.local/poker-web-tests.log`、`.local/poker-platform-tests.log`、`.local/poker-vet.log` |

自然对局记录保存初始快照、实际行动及结果，副数以每份记录的 `result.details` 为准；随机发牌可能导致不同局长，平级会增加决胜副。贡还等明确里程碑夹具不冒充自然完整对局。浏览器验收使用真实模块、真实 HTTP 身份和生产前端，未提供绕过规则的调试接口。

## 重复执行

```sh
. scripts/env.sh
npm --prefix web run build
npm --prefix web test
go test -count=1 -timeout=5m ./games/doudizhu/... ./games/guandan/... ./games/shengji/...
python3 -B scripts/check_game_boundaries.py
python3 -B scripts/accept_poker.py
WIRE_BOARD_POKER_ASSETS=https://cdn.ichenj.com/bga/v20261002-1 python3 -B scripts/accept_poker.py
python3 -B scripts/poker/verify_oss.py
```

浏览器脚本通过 `agent-browser` 运行。触屏手势由 CDP 注入脚本送入该工具拥有的浏览器会话。测试账号、SQLite、浏览器配置和截图全部位于工作区 `.local/`。

## 发布状态

main 执行检查并上传已验证二进制；版本 tag 的发布 workflow 复用同一 SHA 的产物。GitHub Actions 的 `Release v…` 运行记录、Release 与镜像标签共同提供发布来源证据。服务器使用既有备份／回滚脚本更新，并核验健康状态与上下架设置。三个新游戏保持默认下架，部署不会自动上架。
