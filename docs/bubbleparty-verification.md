# 泡泡大作战验收记录

日期：2026-10-19。范围：`bubbleparty` 游戏规则、实时生命周期、2/4/6/8 人地图与机器人、HTTP/SQLite、前端 Canvas/输入/音频和真实浏览器。

## 来源与素材

- 许愿原文：「参考网游泡泡堂的玩法，做出一个流畅版本的相同玩法的游戏，最好复刻出网游中地图：小区10地图和音效」。
- 未使用商业游戏图片、角色、音频或原始地图；角色、地图、封面和音效均为项目原创。
- 音效由 `scripts/bubbleparty/prepare-audio.mjs` 确定性离线合成，11 个 WAV；`games/bubbleparty/assets/manifest.json` 记录生成器与逐文件 SHA-256。
- `小区10` 是原创住宅小区主题对称地图；另有水乐园、森林公园、沙丘营地。

## 已执行检查

```sh
. scripts/env.sh
go test -count=1 ./games/bubbleparty/... ./games
npm --prefix web test
npm --prefix web run build
python3 -B scripts/check_game_boundaries.py
python3 -B scripts/bubbleparty/check-browser.py
```

- 规则测试：人数/地图选项、四张地图对称与出生安全、出生区连通、队伍分配、水泡阻挡、木箱破坏、连锁爆炸、护盾、道具上限、团队胜利、和局、输入序号/epoch 校验。
- 模块 E2E：2/4/6/8 人全电脑对局在 `community10` 完成；水乐园/森林公园/沙丘营地覆盖 2/4/8 人；结果均有 Team、Ranks、Winners/Scores，允许显式和局。
- HTTP E2E：真实注册/登录/SQLite；默认下架；2/8 人倒计时→游戏；观众拒绝；错误 epoch/seq；输入应用；服务重启恢复同一 epoch/地图；奇数容量和非法地图拒绝；真人+电脑实时输入。
- JS 输入测试：单请求在途、合并方向、释放、epoch 重置、禁用、键盘映射、D-pad 死区，5 项通过。
- 前端构建通过；边界检查 23 款游戏无游戏间直接导入。
- 真实浏览器 19 项通过：默认下架、8 座等待、小区10、倒计时、320/390/1280、320 角色≥24px、计算样式对比度、键盘移动/放水泡、原生触屏移动、刷新保留 epoch/序号、结果浮层关闭/重开、再来一局回到等待房间。截图与 `result.json` 在 `.local/bubbleparty/browser/`。
- 本次浏览器运行：`.local/bubbleparty/browser/result.json`（19/19）。

## 已知取舍

- 商业原作的地图布局与音效无法取得，未做逐像素复刻；提供原创的小区10主题地图和原创音效。
- 采用 15×13 格子、单命淘汰、3 分钟后收缩安全区；不同时实现原作的全部道具/角色。
- 机器人以有限 BFS 和炸弹规避为主，保证可完成对局与托管；不追求竞技强度。

## 发布与部署

- main 提交 `fd784037` 的 `Check main and build` 运行 `39245613959` 全绿，包含 `Game bubbleparty (0)`、shared 检查和 `Test and Linux amd64 binary`。
- `v1.19.0` 指向同一提交；Release 运行 `39246891742` 复用已验证构建，未重新编译应用。
- 服务器运行 `ghcr.io/k0ngk0ng/wire-board@sha256:6cc6a1d870b6cde129ba3f123c15c15d8444c537378cd75ed42bc84451c9a2ae`，OCI revision `fd784037`、version `v1.19.0`，`/healthz` 正常。
- 线上 bundle 包含「泡泡大作战」；本地回退音频 `/bubbleparty/v1/audio/place.wav` 返回 200 `audio/wav`。
- 新游戏保持 `bubbleparty=0`，已有 `battlecity=1`、`spf2t=1` 等设置不变；需管理员在后台显式上架。
- 保留本次更新前与上一份备份 `data-20261019T114631Z-865536.tar.gz`、`data-20261019T105535Z-753854.tar.gz`（均通过 `quick_check`）；更旧备份已删除。镜像清理后保留当前 `v1.19.0` 与上一版 `v1.18.1` 回滚镜像。
- 许愿已通过代理接口回写为「已实现」，备注包含版本和默认下架说明。
