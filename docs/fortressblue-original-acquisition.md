# 疯狂坦克原版素材获取与本地运行记录

日期：2026-10-20。对象：《Fortress 2 BLUE（疯狂坦克）》，Steam app `3152590`。
原始媒体版权归 CCR / 原权利方所有，仅经用户明确授权在本私有平台使用；原始文件不进入 Git。

## Steam 客户端尝试

使用本机原生 SteamCMD 6.5.0（非 QEMU）执行：

```sh
steamcmd +login anonymous +app_info_print 3152590
```

公开 appinfo 返回：

```text
depot 3152591
  public manifest gid 7079628454353527068
  size 320,846,385 bytes, download 119,583,952 bytes
  buildid 19611761
```

继续尝试匿名安装/按 manifest 下载：

```text
+app_update 3152590 validate
ERROR! Failed to install app '3152590' (No subscription)

+download_depot 3152590 3152591 7079628454353527068
Depot download failed : missing license for depot (No subscription)
```

Steam 将免费游戏的 AppID 加入库仍需登录账号并完成许可/手机号验证；当前环境没有可用的
Steam 账号，因此无法取得 Steam 安装包。没有使用第三方镜像或来源不明的 depot 替代。

## 官方网页客户端

改为从官方网页客户端 `https://game.fortress2blue.com/` 获取同一套 CCR 资源。下载并保留在
`.local/fortressblue-original/game/`：

| 文件 | SHA-256 |
| --- | --- |
| `tankclient_emsc.c906c.data.00` | `e9f5ff8817e59516bb95e7ec95548c4567749ea762cfc5322ec0bcc8dfc67415` |
| `tankclient_emsc.c906c.data.01` | `71b4130dd2206e744e21f07e7e05c5f7575c01ad64824f72534df2ed07439773` |
| `tankclient_emsc.c906c.data.02` | `1d75e86388ca7fc76b34119139b6ab290c81493496993a36c2d1f83dca75322f` |
| `tankclient_emsc.eeb2b.wasm` | `35974c0d7a06cf86f1f028b0ee1fc46c324921f6d829ce4db684f776199e489d` |
| `tankclient_emsc.9485d.js` | `0b283d4c4e09f0fa01e0ed0a2eaf3ac39cf1fc57517859e1d94f2de8f3102e27` |

哈希同时记录在 `games/fortressblue/assets/sources.json`。Emscripten 包拆出
`Download/stageN_*.i16`、`charNN.i16`、`bulletNN.i16`、`explo.i16` 和 `Asset/` 资源；
解码链为 LH5 解压 → 逐行 span RLE → RGB565，前景层 alpha 生成 4px 碰撞位图。

## REA 分析

使用 REA 6.5.0 对原版客户端 JS 做静态重建：

```sh
rea analyze-javascript-application .local/fortressblue-original/game \
  --artifact-format directory --source-path tankclient_emsc.9485d.js --format json
```

结果（完整 Evidence 保存在 `.local/fortressblue-rea/js-analysis.json`）：

```text
evidence_id: ev_17113b50352625a62983d4b80bfe42a1c9529e03a2d3dc311eedd165ee1203cd
root_artifact_sha256: 844113272508b0c426e2eba5d9fac4f4f1e7e26ac0f076c67f9ee2feeff25059
parsed_javascript_files: 1
visited_ast_nodes: 53,716
semantic_nodes: 20,003
relations: 16,686
findings: 10
parse_failures: 0
```

REA 重建了 Emscripten/WebGL/WebSocket、鼠标/键盘/触摸回调和文件加载边界。当前 REA
provider 覆盖 JavaScript/Electron、Apple artifact、.NET 与 SQLite，没有 Cocos `.i16`/LH5
解码器，因此原版精灵仍由项目内的 `prepare-original-assets.py` 解码；REA 分析用于确认客户端
加载与资源入口，而不是替代自定义贴图解码。

## 本地运行原版客户端

将下载的客户端目录用本地静态服务器打开：

```sh
python3 -m http.server 18233 --bind 127.0.0.1
```

浏览器访问 `http://127.0.0.1:18233/index.html`，原版 Emscripten 客户端在本地启动并显示
「Fortress 2 Blue CCR 2002 版权所有 / 欢迎来到 Fortress 2 Blue / 正在初始化」。截图保存在
`.local/fortressblue-rea/original-local.png`。后续登录需要官方账号/手机号验证，无法继续进入
官方大厅，因此平台内的玩法由本项目服务端重写。

## 原版素材复现结果

`python3 -B scripts/fortressblue/verify-original-source.py` 从原始 `.i16` 重新生成并逐项比较：
**250/250 通过**，覆盖 17 张地图的碰撞位图/出生点/前景/背景/叠加层/缩略图，14 种坦克的
idle/fire/die/move 和炮弹，以及 8 种 `itemlist!.i16` 道具图标与元数据。结果保存在 `.local/fortressblue/asset-redecode.json`。

## 重做范围

- 17 张原版地图、14 种坦克、8 种原版道具、原版属性/技能/角度/射程、炮弹、爆炸、音乐和音效。
- 服务端权威回合制炮战、可破坏像素地形、风力、特殊弹、坠落、突然死亡、电脑/托管、观战和恢复。
- 2/4/6/8 人（最多 4v4）红蓝队伍，PC 鼠标/键盘和手机触屏。
- 未包含原版 3D 引擎的账号成长、商城/道具购买、语音聊天和全部官方模式；这些不属于本次
  平台玩法的实现范围。
