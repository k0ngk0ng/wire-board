# 魂斗罗真人版交付

本次交付单人挑战和双人真人合作。按用户最新要求，暂不开放电脑玩家与托管，不以电脑通关作为交付门槛。原版八关战役、正常生命数、继续游戏和合作计分逻辑保留；测试补命仅存在于研发工具中。

## 可用功能

- 单人可以独立开局，双人房间由两位真人加入并准备。
- 复用平台「控制器设置」、FC 手柄、全屏、加载页、音频和结算浮层。
- 通关由服务器判定，真人通关全员加分，失败全员扣分；结算重复请求不重复记账。
- 前端不显示电脑或托管入口，服务端拒绝 `add_bot` 和 `autoplay` 请求。

## 已执行验证

[真实主程序验收](evidence/human-delivery/result.json)与[浏览器记录](evidence/human-delivery/browser/result.json)覆盖默认注册、健康检查、单人开局与移动、双人真人房间、PC／320px、键鼠／触屏、控制器改键、全屏、加载、观战、刷新及网络恢复。验收使用真实主程序和隔离的本地数据库，结束后清理测试账户所在数据库。

[公共控制器检查](evidence/human-delivery/controllers/verification.json)验证了多指 A＋B、方向＋A＋B，以及依次松开 A、方向后仍保持 B。使用浏览器原生触摸事件，未声称完成 iPhone 真机验收。单指原地覆盖 A、B 无法可靠表达两个独立触点；按用户要求，本次不增加组合键或特殊手势。

[原生构建检查](evidence/human-delivery/native-binding.json)、[前端检查](evidence/human-delivery/frontend.json)和[CI 选择检查](evidence/human-delivery/ci.json)已通过。普通 main 检查不运行电脑长战役，实验性电脑检查需要显式开启且不阻塞真人版交付。此前的八关规则对照、合法输入通关回放和合作积分 HTTP／SQLite 记录继续保留，范围见[实现与验收记录](IMPLEMENTATION.md)。

## 本地运行

从仓库根目录执行。需要 Go 1.26+、Node 22+ 和支持 `lzma` 的 Python 3；不需要读取 ROM 或开发机的研究音频目录。

```sh
. scripts/env.sh
npm --prefix web ci
npm --prefix web run build
python3 -B games/contra/build/native.py --runtime .local/contra-web-runtime/public/manifest.json --out .local/wire-board
INVITE_CODE=local-friends ADDR=127.0.0.1:18080 DATA_DIR="$PWD/.local/data" .local/wire-board
```

前端构建自动验证并展开无损音频源码包，生成带摘要的运行资源。原生构建必须绑定同一份运行清单；未注入真实身份的普通 `go build` 产物会拒绝启动。

## 交付边界

本文记录真人版功能和本地验收；线上版本以 `/api/version` 与对应 Release 记录为准。正式发布仍须由版本 tag 复用 main 已验证的同提交产物。公网并发容量和实体设备听感不在本次本地验收结论内。

电脑策略仍有未解决的完整战役问题，研发记录明确标为未通过或辅助接续；不把这些记录当作电脑功能可用的证明。
