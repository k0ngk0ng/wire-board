# DotA：兵线争锋 · 可行性样板

经典 DOTA1 素材、原创桌游规则。当前版本 **v0.9-full-ancient**：1v1、2v2、3v3 都必须摧毁敌方遗迹。

- [可行性报告及验证边界](REPORT.md)
- [完整纸面规则、英雄、装备](RULES.md)
- [57 张原版素材预览](assets-preview.png)
- [桌面试玩截图](desktop-preview.png) / [手机试玩截图](mobile-preview.png)
- [素材来源与校验清单](assets.json)
- [当前版本的 3,000 局推演结果](simulation-results.json)

## 本机试玩

在仓库根目录运行，仅需 Python 3.10 或以上，无第三方依赖：

```sh
python3 -B docs/research/dota1/lab_server.py
```

打开 <http://127.0.0.1:18764/>。选择人数 → 普通房间 → 开始 → 分队 → 选英雄 → 出牌。你控制一名英雄，其他位置由电脑补齐；“电脑代打一轮”之后可以继续手动操作。

这是单机规则样板，只监听本机地址。没有账户、网络多人认证、持久化或正式大厅接入；刷新页面需重新开局，关闭进程后牌局丢失。不要把这个 HTTP 样板直接发布到服务器。正式实现继续使用现有 Go 服务。

## 重现验证

```sh
python3 -B -m unittest discover -s docs/research/dota1 -p 'test_*.py' -v
python3 -B docs/research/dota1/simulate.py --count 200
```

第二条运行 3 种人数 × 5 种策略配对 × 200 局，共 3,000 局，会覆盖 `simulation-results.json`。`--count 10` 可做较短检查，但结果不能再称为 3,000 局验证。

素材已保存在本目录，试玩不需要联网下载。确需重新抓取时：

```sh
python3 -B docs/research/dota1/verify_assets.py
```

该脚本会访问 iCCup 并覆盖本目录素材和清单，外站结构变化可能使抓取失效。

## 旧结果的含义

`simulation-v0.1.json`、`simulation-v0.2.json`、`simulation-v0.3.json` 是早期规则记录；`simulation-v0.5-solo-rejected.json` 和 `holdout-results.json` 使用了已被否决的 Solo 胜利规则。它们仅用于追溯试验，**不能作为当前规则的验证依据**。当前依据以 `simulation-results.json` 的 `rules: v0.9-full-ancient` 为准。
