import type { View } from "./types";
import { describeOutcome, roundRow } from "./outcome";

export function ResultDetails({ data }: { data: View }) {
  const reason = describeOutcome(data.rounds ?? [], data.winnerTeam);
  if (!reason) return null;
  return (
    <div className="pvz-outcome" data-pvz-outcome>
      <p data-pvz-outcome-reason>{reason}</p>
      <table aria-label="两回合攻防结果">
        <thead>
          <tr>
            <th>回合</th>
            <th>进攻</th>
            <th>防守</th>
            <th>结果</th>
            <th>用时</th>
          </tr>
        </thead>
        <tbody>
          {data.rounds.map((round, index) => {
            const row = roundRow(round, index);
            return (
              <tr key={index}>
                <th>{row.round}</th>
                <td>{row.attack}</td>
                <td>{row.defense}</td>
                <td>{row.result}</td>
                <td>{row.time}</td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}
