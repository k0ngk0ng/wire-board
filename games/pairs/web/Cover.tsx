import { useContext, useState } from "react";
import { AssetsContext } from "../../../web/src/ui/board-host";
import { asset } from "./media";
export function CoverArt() {
  return (
    <svg
      viewBox="0 0 520 260"
      preserveAspectRatio="xMidYMid slice"
      aria-hidden="true"
    >
      <rect width="520" height="260" fill="#dce6cf" />
      <path d="M0 182Q110 139 260 183T520 168V260H0Z" fill="#bdcfaf" />
      <circle cx="442" cy="45" r="25" fill="#eed79f" />
      <path
        d="M45 260Q36 174 17 131Q65 147 70 199Q82 140 106 124Q111 187 78 260M470 260Q463 183 492 140Q506 199 489 260"
        fill="#739676"
      />
      <g
        fill="none"
        stroke="#e8bc61"
        strokeWidth="10"
        strokeLinecap="round"
        strokeLinejoin="round"
      >
        <path d="M142 91V43H371V169" />
        <path d="M143 170H258V93" />
      </g>
      {[
        { x: 103, y: 60, star: true, angle: -7 },
        { x: 333, y: 140, star: true, angle: 6 },
        { x: 104, y: 141, star: false, angle: 5 },
        { x: 221, y: 62, star: false, angle: -4 },
      ].map(({ x, y, star, angle }, i) => (
        <g key={i} transform={`translate(${x} ${y}) rotate(${angle} 39 39)`}>
          <rect
            x="3"
            y="7"
            width="78"
            height="78"
            rx="13"
            fill="#466354"
            opacity=".2"
          />
          <rect
            width="78"
            height="78"
            rx="13"
            fill="#fff3da"
            stroke="#66846a"
            strokeWidth="2.5"
          />
          <rect
            x="5"
            y="5"
            width="68"
            height="68"
            rx="10"
            fill={star ? "#ead5a0" : "#cddfc7"}
          />
          {star ? (
            <path
              d="m39 13 8 17 19 2-14 13 4 19-17-9-17 9 4-19-14-13 19-2Z"
              fill="#cf9655"
              stroke="#986c43"
              strokeWidth="2"
            />
          ) : (
            <g fill="#668d69" stroke="#466b54" strokeWidth="2">
              <path d="M24 53Q13 20 55 19Q62 53 24 53Z" />
              <path d="m24 58 25-30" fill="none" />
            </g>
          )}
        </g>
      ))}
      <g fill="#fff4da">
        <path d="m268 34 3 8 8 3-8 3-3 8-3-8-8-3 8-3Z" />
        <circle cx="410" cy="97" r="4" />
        <circle cx="85" cy="97" r="3" />
      </g>
    </svg>
  );
}
export function Cover() {
  const base = useContext(AssetsContext);
  const [failed, setFailed] = useState(false);
  return base && !failed ? (
    <img
      src={asset(base, "cover.webp")}
      alt=""
      onError={() => setFailed(true)}
      style={{ width: "100%", height: "100%", objectFit: "cover" }}
    />
  ) : (
    <CoverArt />
  );
}
