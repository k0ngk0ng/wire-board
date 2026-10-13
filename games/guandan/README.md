# 掼蛋

四人固定搭档、两副 108 张牌，采用江苏 2017 规则基线。红心级牌可配普通牌但不能代王；实现多义牌型声明、炸弹／同花顺比较、进贡还贡、双下、合持两大王抗贡、对家接风及从 2 打到过 A。也可约定有限副数休闲局，平级加打一副。

规则、快照、私有视图、机器人与前端分别维护在本目录。服务器负责最终判型；手动选牌可通过私密 `preview` 获得全部合法解释，提示列表不限制可用的实体牌组合。还贡牌及预览只对有权查看的玩家显示。明确地区差异与线上补充条款见房间「玩法速查」和 [实施约定](../../docs/poker-implementation.md)。

```sh
. scripts/env.sh
go test -count=1 -timeout=5m ./games/guandan/...
python3 -B scripts/accept_poker.py
```

自然完整对局记录在 `.local/poker/guandan/natural-*.json`，真实 HTTP 的双贡／私密还贡、整局和恢复记录在 `.local/poker/guandan/http/`。精确贡还测试使用明确标注的里程碑夹具，不能与自然发牌完局数量混计。

默认下架，管理员上架后才可创建。牌面／牌背与平台音效使用规则无关的共享呈现，详见 [素材清单](../../assets/poker/README.md)。
