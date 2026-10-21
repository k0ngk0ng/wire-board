# 图片素材回退

联网对局加载图片时，先验证远端完整素材组；远端不可用或图片摘要不符时，重新加载本地完整素材组。两个来源均失败则明确报错，不返回缺少图片的半成品。

先编写浏览器验收，再添加加载器及联网客户端接入。原生 Chromium 的四个场景通过：远端正常、远端不可用、远端 PNG 摘要错误、远端与本地均不可用。前三种均加载全部 8 个球场、8 个结算场景和 120 个结算姿态；最后一种拒绝加载。

```sh
source scripts/env.sh
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/accept_asset_fallback.py
```

失败记录为 `.local/asset-fallback-red.log`，通过记录为 `.local/asset-fallback-green.log`。完整 324 条请求记录与结果保存在 `.local/basketball-asset-fallback/result.json`，浏览器截图为同目录 `browser.png`。

这些是本地服务模拟的远端故障，未验证实际 OSS 网络。游戏专属素材地址、生产打包和真实 CDN 回读分别验收。
