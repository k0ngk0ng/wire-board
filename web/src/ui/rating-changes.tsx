import type { Room } from "../types";
import { PlayerName } from "../profiles";
import { unratedDescription } from "./cooperative-rating";

export function RatingChanges({
  result,
}: {
  result: NonNullable<Room["result"]>;
}) {
  return (
    <div className="result-ratings">
      {result.rated ? (
        result.players.map((p) => (
          <span key={p.id}>
            <PlayerName user={p} />{" "}
            <b
              className={
                result.draw
                  ? "rating-neutral"
                  : (p.ratingDelta || 0) > 0
                    ? "rating-gain"
                    : "rating-loss"
              }
            >
              {(p.ratingDelta || 0) > 0 ? "+" : ""}
              {p.ratingDelta ?? 0} 积分
            </b>
          </span>
        ))
      ) : (
        <p>{unratedDescription(result)}</p>
      )}
    </div>
  );
}
