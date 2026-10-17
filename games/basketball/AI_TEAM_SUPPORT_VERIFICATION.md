# 原版双人支撑策略 mode28

## 实现前失败场景

- 独立全电脑cold在update519进入mode28。恢复A95F的队友支撑策略，不能用直接追球代替。
- 首次进入设置Entered并选择Run；已在Run时保留原动作进入状态。人类队友且未依附时随后转mode22，不能跳过原先的动作选择。
- 非Idle时按缓存zone的横向band及depth>=5生成原方向；进入中心且无需depth移动时请求Idle，不传送位置。
- Idle时仅接受队友链接高位有效且低四位指向自己；若自己持球转mode10。队友持球时请求动作9及原方向，否则请求动作7；不额外设计支撑规则。
- 任一返回只改变原明确写的动作、方向或mode，不改变球所有者、链接、体力、位置或RNG。若动作7/9物理尚未恢复，下一阶段必须明确报错并交动作负责者。
- 自然完整函数调用与隔离A95F矩阵分别验收，矩阵覆盖首次/持续、方向门槛、链接、席位、人类队友和持球关系，不视作自然整局证据。

## 证据与结果

原A95F完整REA证据为`ev_eef21fb30302a3ec59b8b3dc097be9a7fa2a559339d598963a8489a9d646759d`。AA28按席位给出的水平位翻转为[0,0,3,3]，AA18／AA20决定队友持球时的原方向；完整实现保留这些条件和顺序，没有换成寻路算法。

独立18,000帧自然参考中未命中A95F，零命中采样及RAM／PNG一致性记录仍保留，不能算自然调用覆盖。另使用原A95F及B1D5／B227执行15,360组隔离矩阵，覆盖进入状态、动作、席位、人类队友、链接、持球对象与zone；全部Go决策字段精确匹配，未额外改变随机流或物理。

测试先于实现；原生竞态与Wasm均通过。和mode29一同恢复后，独立全电脑cold达到6000逻辑更新两次一致，原mixed6000及1998/57原连续回归仍通过。详情见ALL_COMPUTER_COLD_VERIFICATION，不能据此声称完整视频比赛或所有玩法均已完成。

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/basketball/verify_ai_team_support.py
source scripts/env.sh
go test ./games/basketball/engine -run TestAITeamSupport -count=1
```
