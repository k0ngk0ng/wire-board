import { useId } from "react";
export function CoverArt() {
  const grain = useId(),
    shine = useId();
  return (
    <svg
      viewBox="0 0 520 260"
      preserveAspectRatio="xMidYMid meet"
      style={{ background: "#dce6d9" }}
      aria-hidden="true"
    >
      <defs>
        <pattern id={grain} width="9" height="9" patternUnits="userSpaceOnUse">
          <path
            d="M1 2l2 1m3 4l2-1"
            stroke="#f0ebcc"
            strokeWidth=".65"
            opacity=".12"
          />
        </pattern>
        <radialGradient id={shine} cx="30%" cy="25%">
          <stop offset="0" stopColor="#fffbed" />
          <stop offset=".28" stopColor="#efe4c4" />
          <stop offset="1" stopColor="#c5c8ab" />
        </radialGradient>
      </defs>
      <rect width="520" height="260" fill="#dce6d9" />
      <path d="M0 213Q93 169 215 211T520 183V260H0Z" fill="#b8cfba" />
      <circle cx="275" cy="126" r="112" fill="#e9ecd8" />
      <g transform="rotate(-7 260 130)">
        <rect
          x="80"
          y="38"
          width="378"
          height="184"
          rx="17"
          fill="#88744f"
          stroke="#62785c"
          strokeWidth="3"
        />
        <rect
          x="92"
          y="50"
          width="354"
          height="160"
          rx="9"
          fill="#356752"
          stroke="#b9ae7b"
          strokeWidth="3"
        />
        <rect
          x="95"
          y="53"
          width="348"
          height="154"
          rx="7"
          fill={`url(#${grain})`}
        />
        <path
          d="M168 54v150m0-105a30 30 0 0 0 0 60"
          fill="none"
          stroke="#d1d7b2"
          strokeWidth="1.5"
        />
        {[
          [96, 54],
          [269, 52],
          [442, 54],
          [96, 205],
          [269, 208],
          [442, 205],
        ].map(([x, y], i) => (
          <circle key={i} cx={x} cy={y} r="7" fill="#213f35" />
        ))}
        {Array.from({ length: 5 }, (_, row) =>
          Array.from({ length: row + 1 }, (_, col) => (
            <g key={row + "-" + col}>
              <circle
                cx={342 + row * 10}
                cy={132 + (2 * col - row) * 5.5}
                r="5.1"
                fill="#9d4c46"
                stroke="#6e4d3d"
                strokeWidth=".8"
              />
              <circle
                cx={340.5 + row * 10}
                cy={130.5 + (2 * col - row) * 5.5}
                r="1.3"
                fill="#e0a197"
              />
            </g>
          )),
        )}
        {[
          [168, 97, "#46714d"],
          [168, 163, "#d6b95b"],
          [168, 130, "#987052"],
          [269, 130, "#588aa1"],
          [329, 130, "#d49aa3"],
          [416, 130, "#263c35"],
        ].map(([x, y, c], i) => (
          <circle
            key={i}
            cx={x as number}
            cy={y as number}
            r="5.5"
            fill={c as string}
            stroke="#3d5543"
            strokeWidth=".8"
          />
        ))}
        <circle
          cx="204"
          cy="152"
          r="7.2"
          fill={`url(#${shine})`}
          stroke="#c7c8a7"
          strokeWidth="1"
        />
        <path
          d="M15 220l167-60"
          stroke="#594c3b"
          strokeWidth="9"
          strokeLinecap="round"
        />
        <path
          d="M81 196l111-39"
          stroke="#cfb680"
          strokeWidth="5"
          strokeLinecap="round"
        />
        <path d="M188 159l5-2" stroke="#7dadaa" strokeWidth="5" />
        <path
          d="M218 146l85-21"
          stroke="#d6d9af"
          strokeWidth="1.5"
          strokeDasharray="5 7"
          opacity=".6"
        />
      </g>
      <g transform="rotate(12 464 226)">
        <rect
          x="444"
          y="209"
          width="26"
          height="24"
          rx="3"
          fill="#d4c694"
          stroke="#698373"
          strokeWidth="2"
        />
        <rect x="448" y="211" width="18" height="9" fill="#83aeb1" />
      </g>
    </svg>
  );
}
