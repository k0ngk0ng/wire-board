import type { ReactNode } from "react";
import { Check, Landmark } from "lucide-react";
import type { Room } from "./types";
import { gemCityProgress } from "./splendor-city-state";
import { cityAtlasStyle } from "./splendor-city-art";
import "./splendor-cities.css";

export function SplendorCities({
  room,
  assets,
  renderGem,
}: {
  room: Room;
  assets: string;
  renderGem: (color: number) => ReactNode;
}) {
  const s = room.game!.splendor!;
  const me = room.spectating ? undefined : s.players[room.you];
  if (!s.options?.cities || !s.cities?.length) return null;
  return (
    <section className="gem-cities" aria-label="城市目标">
      <header>
        <Landmark size={18} />
        <h3>城市目标</h3>
        <details>
          <summary>规则</summary>
          <p>
            城市替代贵族。回合结束时满足任意一座城市的分数与发展卡条件，触发最后一轮。城市不会被拿走，多人可以达成同一座城市。
          </p>
          <p>
            只在达成人员中比较分数；同分时，发展卡更少者获胜，仍相同则共同获胜。
          </p>
          <p>
            条件按卡牌张数计算，双宝石卡也只算一张。彩色复制卡计入配对后的颜色；黄金卡不计入任何颜色。
          </p>
        </details>
      </header>
      <div className="gem-city-list">
        {s.cities.map((city, index) => {
          const art = cityAtlasStyle(assets, city.tile);
          const progress = me ? gemCityProgress(me, city) : undefined;
          const eligible = !!progress?.eligible;
          const owners = room.seats.filter((_, seat) =>
            s.cityEligibility?.[seat]?.includes(index),
          );
          return (
            <article
              key={`${city.tile}-${city.side}`}
              className={`gem-city ${eligible ? "is-eligible" : ""}`}
              aria-label={city.name}
            >
              {art && (
                <div
                  className="gem-city-art"
                  style={art}
                  aria-hidden="true"
                />
              )}
              <h4>
                {city.name}
                {eligible && <Check size={17} aria-label="已达成" />}
              </h4>
              <div className="gem-city-score">
                <b>{city.points}</b>
                <span>分</span>
                {me && (
                  <small className={me.score >= city.points ? "is-met" : ""}>
                    你 {me.score} 分
                  </small>
                )}
              </div>
              <div className="gem-city-requirements">
                {city.cost.map(
                  (need, color) =>
                    need > 0 && (
                      <span
                        key={color}
                        className={`gem-city-requirement ${progress && progress.counts[color] >= need ? "is-met" : ""}`}
                      >
                        {renderGem(color)}
                        <b>
                          {progress ? `${progress.counts[color]}/` : ""}
                          {need}
                        </b>
                        <small>张</small>
                      </span>
                    ),
                )}
                {!!city.any && (
                  <span
                    className={`gem-city-requirement gem-city-any ${progress && progress.other >= city.any ? "is-met" : ""}`}
                  >
                    <span>{city.cost.some(Boolean) ? "另一色" : "任一色"}</span>
                    <b>
                      {progress ? `${progress.other}/` : ""}
                      {city.any}
                    </b>
                    <small>张</small>
                  </span>
                )}
                {!city.cost.some(Boolean) && !city.any && (
                  <small>无颜色要求</small>
                )}
              </div>
              {!!owners.length && (
                <p className="gem-city-owners">
                  <Check size={12} />
                  {owners.map((p) => p.name).join("、")} 已达成
                </p>
              )}
            </article>
          );
        })}
      </div>
      <small className="gem-city-caption">
        {me ? "已持有 / 需要的发展卡张数" : "所需分数与发展卡张数"}
        {s.lastRound ? " · 最后一轮" : ""}
      </small>
    </section>
  );
}
