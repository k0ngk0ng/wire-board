function Gem({
  x,
  y,
  color,
  size = 39,
}: {
  x: number;
  y: number;
  color: string;
  size?: number;
}) {
  return (
    <g transform={`translate(${x} ${y})`} strokeLinejoin="round">
      <rect
        width={size}
        height={size}
        rx="5"
        fill={color}
        stroke="#504c54"
        strokeWidth="2.5"
      />
      <path
        d={`M3 3 10 11H${size - 10}L${size - 3} 3M10 11v${size - 22}L3 ${size - 3}M10 ${size - 11}h${size - 20}l7 8M${size - 10} 11v${size - 22}`}
        fill="none"
        stroke="#faf0e3"
        strokeWidth="2"
        opacity=".65"
      />
      <path
        d={`M${size - 4} 8v${size - 12}H8l5-6h${size - 22}V13Z`}
        fill="#504c54"
        opacity=".18"
      />
    </g>
  );
}

export function Cover() {
  return (
    <svg
      viewBox="0 0 520 320"
      role="img"
      aria-label="街霸方块：红蓝宝石阵左右对抗，中央宝石碎裂引发连锁"
    >
      <rect width="520" height="320" fill="#eae1d6" />
      <path d="M0 0h260L182 320H0Z" fill="#e7ccc2" />
      <path d="M520 0H322L248 320h272Z" fill="#cbd9dc" />
      <path
        d="m289 0-43 117 22 13-59 190"
        fill="none"
        stroke="#f8f0dc"
        strokeWidth="13"
      />
      <g fill="none" strokeWidth="3" opacity=".45">
        <path
          d="m0 56 174 63M0 111l168 34M0 209l177-28M15 282l166-74"
          stroke="#a9655a"
        />
        <path
          d="m520 45-172 76m172-12-161 38m161 66-165-32m150 98-154-68"
          stroke="#64899b"
        />
      </g>
      <path
        d="M37 272h158M326 272h158"
        stroke="#777773"
        strokeWidth="5"
        strokeLinecap="round"
      />
      <g transform="rotate(-6 112 204)">
        {[
          [44, 212],
          [86, 212],
          [128, 212],
          [44, 170],
          [86, 170],
          [128, 170],
          [86, 128],
        ].map(([x, y], i) => (
          <Gem key={i} x={x} y={y} color="#bf7669" />
        ))}
        <Gem x={44} y={128} color="#d3b567" />
        <Gem x={128} y={128} color="#91a47c" />
      </g>
      <g transform="rotate(6 408 204)">
        {[
          [350, 212],
          [392, 212],
          [434, 212],
          [350, 170],
          [392, 170],
          [434, 170],
          [392, 128],
        ].map(([x, y], i) => (
          <Gem key={i} x={x} y={y} color="#7297af" />
        ))}
        <Gem x={350} y={128} color="#d3b567" />
        <Gem x={434} y={128} color="#9683a5" />
      </g>
      <g transform="translate(93 49) rotate(-16)">
        <Gem x={0} y={0} color="#bf7669" size={34} />
        <circle
          cx="17"
          cy="51"
          r="17"
          fill="#e9a79a"
          stroke="#6f4e4d"
          strokeWidth="3"
        />
        <path d="m17 39 5 8 7 4-7 4-5 8-5-8-7-4 7-4Z" fill="#fff0d9" />
      </g>
      <g transform="translate(389 43) rotate(17)">
        <Gem x={0} y={0} color="#7297af" size={34} />
        <circle
          cx="17"
          cy="51"
          r="17"
          fill="#a6c3d0"
          stroke="#4c6373"
          strokeWidth="3"
        />
        <path d="m17 39 5 8 7 4-7 4-5 8-5-8-7-4 7-4Z" fill="#fff0d9" />
      </g>
      <path
        d="m257 100 11 30 29-15-7 33 34 6-28 20 25 25-34-1 1 33-26-22-23 26-1-35-34 4 23-27-30-17 34-8-11-32 29 13Z"
        fill="#f8e7b5"
        stroke="#b99a67"
        strokeWidth="2.5"
        strokeLinejoin="round"
      />
      <g stroke="#755e60" strokeWidth="2.5" strokeLinejoin="round">
        <path d="m239 146 21-9-2 26-24 8Z" fill="#c98176" />
        <path d="m268 139 19 16-20 19-4-10Z" fill="#7ba3b7" />
        <path d="m239 179 20-10 5 25-18 8Z" fill="#d3ad6d" />
        <path d="m275 181 17-7-2 21-11 6Z" fill="#9683a5" />
      </g>
      <g fill="#f8edcf" stroke="#a9936b" strokeWidth="2">
        <path d="m193 91 7 12-13 5ZM318 88l10 13-13 6ZM185 240l12-5 4 13ZM324 235l13 4-7 12Z" />
      </g>
    </svg>
  );
}
