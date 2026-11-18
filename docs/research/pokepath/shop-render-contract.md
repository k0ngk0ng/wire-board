# 商店与孵化

商店有一个双宽蛋位和十个道具货架。货架底色取决于货架组；无货时保留底色、显示售罄并禁用购买。金币不足时仍可查看说明。关闭地图效果时，商店背景停在静态米色棋盘；开启时按主题显示棋盘位移及配色变化。

购买蛋和道具消耗同一笔房间金币。蛋报价绑定库存数，价格到达 50,000 上限后也能拒绝过期报价。道具报价同时绑定货架和道具编号，旧报价不会误买补进同一货架的商品。已经确认的输入可安全重传。

购买蛋后，购买者看到服务端确认的新伙伴；其他玩家只收到共同的库存和队伍变化。普通道具补货，无获得窗口；索财灵道具换得赛富豪时沿用伙伴获得窗口。重新连接或续玩不会重播历史获得窗口。

```sh
. scripts/env.sh
npm --prefix web run build
node games/pokepath/tests/shop-render-oracle.mjs
node games/pokepath/tests/native-platform-browser.mjs --standard-web
go test ./games/pokepath/tests -run '^TestNative(PlatformShop|PlatformSharedCost|ProgressionInitialSharedShop|ProgressionEquipmentRestrictions)' -count=1
```

画面对照取自 `ShopScene.js` 的商店与孵化窗口。五组状态共 17 张 720×624 截图均无像素差异，购买回执和指定时刻的动画也一致。完整截图、回执及键盘道具说明见 `.local/pokepath/verification/shop-render/result.json`。双人正式网页报告在 `.local/pokepath/native-browser/run-548623a7/`，验证真实鼠标购买、另一位玩家提交过期报价、仅购买者收到孵化窗口，以及关闭窗口后可继续部署。
