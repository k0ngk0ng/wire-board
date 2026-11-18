# 原版「风神」与施法中续玩验收

使用 98 版的原队员、法术、战场和敌人，普通体力。独立场景只用于缩短到同一场战斗的路程，不修改已学法术、真气、伤害、动画或奖励。

## 实现前失败场景

1. 只准备召唤形象或后续攻击中的一份素材；即使服务端已发出正确帧，首个客户端仍看不到某一段。
2. 分段时错把同一原版重复帧当成掉帧、把伤害提前到特效之前，或改变原版重复轮数。独立原生记录、游戏流和实际画布分别留证。
3. 在风系特效播放中保存，返回时从头施法、重复耗气、跳过剩余画面或丢失原版节奏。保存与继续的公开画面、待消费等待和原版结算分别核对。
4. 保存未释放原账号；保存后离线时规则仍推进；恢复后旧动作能够改写当前进度。用平台真实认证房间、保存和继续入口复现。
5. 手机全屏时遮挡或者挤占画布；施法、保存和继续仍必须通过原生触摸与平台控件完成。

## 实测结果

独立原版战斗使用巫后、灯笼敌人和战场 18，巫后从原法术列表选择「风神」（对象 315）。初始 790 点真气，施法时扣 64 点；战后原版成长将真气恢复至 758 点。原生对照逐帧核对敌人仍有 30 点体力，直到最后一张风系特效才结算伤害，随后才移除敌人，发放 3 文战利金。

原版先呈现召唤形象图集 14，再呈现风系图集 44。风系演出有 60 个 40 毫秒帧，从首帧到末帧相隔 2360 毫秒。图集的 16 张画面依序循环三次；原版留在同一张画面的重复帧不计为新画面。桌面连续实测从首帧到末帧为 2358 毫秒，游戏流和浏览器像素都观察到两张召唤形象及三轮共 48 次风系画面变化。两份已发布图集的描述文件各延迟 600 毫秒，均在玩家施法前准备完毕。

320px 手机全屏保持 320×200 的游戏画布。游戏连接实测 RTT 为 202 毫秒，玩家通过原版法术图标和文字选招，触摸实际「保存并退出」控件。第一轮风系特效第 10 帧冻结，原账号退出并释放席位，约 3.9 秒后通过平台「继续」返回同一对局。冻结和恢复时的图集、画面绘制指令、调色板、待消费的原生等待、队员真气、敌人与钱包一致，操作代次向前推进。保存恰逢平台当前期限已经到达，恢复不额外增加等待。实际画布保留已播前缀并显示剩余帧，最终仍只耗气一次、只发放 3 文。

两轮均未改原始规则或资源，沿用已发布 CDN 地址，无上传。此处验证巫后施放「风神」的特效与保存，不代表其他法术、并行密集战斗或所有伤害数字像素已经验收。账号权限和旧输入拒绝另有[存档验收](../../rules/saved-games.md)和[战斗恢复记录](BATTLE_RECOVERY_TESTING.md)。

## 复现

使用当前候选前端，并为每次运行使用新的作用域。在仓库根目录执行：

```sh
. scripts/env.sh
python3 -B games/pal/build/native.py --battle-fixture 1 --fixture-field 18 --fixture-roles 3 --out .local/pal/summon-wind/native
scope=summon-repeat
out="$PWD/.local/pal/performance/dense-battle-$scope"
mkdir -p "$out"
(cd web && npm run build -- --outDir "$out/web-dist")
PAL_BOARD_BROWSER=1 PAL_BOARD_ENTRY=battle PAL_SCREEN_BATTLE_CURRENT=1 PAL_DENSE_BATTLE_NATIVE=summon PAL_DENSE_BATTLE_SCOPE="$scope" go test -v ./internal/server -run '^TestPALBoardBrowserFixture$' -count=1 -timeout=8m
```

夹具写出 `url` 后，在另一终端运行 `python3 -B games/pal/tests/summon-wind-browser.py summon-repeat`。手机另开作用域，增加 `--phone-save`。原生参考、WebSocket、实际画布和截图位于 `.local/pal/performance/dense-battle-summon-desktop/` 和 `.local/pal/performance/dense-battle-summon-phone-save/`。
