function BrickWall({
  x,
  y,
  width,
  height,
}: {
  x: number;
  y: number;
  width: number;
  height: number;
}) {
  return (
    <g transform={`translate(${x} ${y})`} stroke="#825e4d" strokeWidth="2">
      <rect width={width} height={height} rx="2" fill="#ba8970" />
      {Array.from({ length: height / 14 - 1 }, (_, i) => (
        <path key={i} d={`M0 ${(i + 1) * 14}h${width}`} />
      ))}
      {Array.from({ length: height / 14 }, (_, row) =>
        Array.from({ length: Math.ceil(width / 28) }, (_, col) => (
          <path
            key={`${row}-${col}`}
            d={`M${col * 28 + (row % 2 ? 7 : 21)} ${row * 14}v14`}
          />
        )),
      )}
    </g>
  );
}

function Tank({
  x,
  y,
  color,
  angle = 0,
}: {
  x: number;
  y: number;
  color: string;
  angle?: number;
}) {
  return (
    <g
      transform={`translate(${x} ${y}) rotate(${angle})`}
      stroke="#4b554b"
      strokeWidth="3"
      strokeLinejoin="round"
    >
      <rect x="-31" y="-24" width="16" height="57" rx="5" fill="#687263" />
      <rect x="15" y="-24" width="16" height="57" rx="5" fill="#687263" />
      <path
        d="M-29-13h12m-12 12h12m-12 12h12m-12 12h12M17-13h12M17-1h12M17 11h12M17 23h12"
        stroke="#b3b7a0"
        strokeWidth="2.5"
      />
      <path d="M-20-22h40v52h-40Z" fill={color} />
      <path d="M-14 20h28" stroke="#f0e7c5" opacity=".65" />
      <rect x="-14" y="-12" width="28" height="28" rx="7" fill={color} />
      <path d="M-5-7v-34H5v34Z" fill="#ded5ad" />
      <path d="M-9 8h18" stroke="#59634e" />
    </g>
  );
}

export function Cover() {
  return (
    <svg
      viewBox="0 0 520 320"
      role="img"
      aria-label="坦克大战：俯视砖墙迷宫，黄绿坦克守卫鹰形基地"
    >
      <rect width="520" height="320" fill="#d7d5bc" />
      <path
        d="M27 0v320M78 0v320M129 0v320M180 0v320M231 0v320M282 0v320M333 0v320M384 0v320M435 0v320M486 0v320M0 32h520M0 83h520M0 134h520M0 185h520M0 236h520M0 287h520"
        fill="none"
        stroke="#c4c6ad"
        strokeWidth="1.5"
      />
      <path
        d="M0 0h83v22H25v83H0ZM520 0h-29v90h29ZM0 297h89v23H0ZM474 270h46v50h-74v-22h28Z"
        fill="#91a080"
      />
      <g fill="#aab394" stroke="#879878" strokeWidth="2" strokeLinecap="round">
        <path d="m13 14 10-7m15 5 9-6m-31 42 8-7M502 12l9-6m-12 30 13-6m-11 34 10-7M17 310l10-6m31 9 10-5m419-19 9-6m3 26 11-6" />
      </g>
      <BrickWall x={59} y={42} width={56} height={70} />
      <BrickWall x={208} y={25} width={112} height={42} />
      <BrickWall x={207} y={67} width={42} height={56} />
      <BrickWall x={59} y={177} width={98} height={42} />
      <BrickWall x={363} y={117} width={98} height={42} />
      <BrickWall x={405} y={159} width={56} height={56} />
      <g stroke="#6e807b" strokeWidth="2.5" fill="#aabcb4">
        {[
          [277, 103],
          [305, 103],
          [277, 131],
          [305, 131],
          [129, 55],
          [129, 83],
        ].map(([x, y], i) => (
          <g key={i} transform={`translate(${x} ${y})`}>
            <rect width="26" height="26" rx="2" />
            <path
              d="m5 20 15-15M5 5h15v15"
              fill="none"
              stroke="#dce4d5"
              strokeWidth="2"
            />
          </g>
        ))}
      </g>
      <path
        d="M199 304v-75h120v75"
        fill="none"
        stroke="#825e4d"
        strokeWidth="25"
        strokeLinejoin="round"
      />
      <path
        d="M199 302v-73h120v73"
        fill="none"
        stroke="#bd8b70"
        strokeWidth="20"
        strokeLinejoin="round"
      />
      <path
        d="M189 247h20m-20 18h20m-20 18h20m100-36h20m-20 18h20m-20 18h20M223 218v22m25-22v22m25-22v22m24-22v22"
        stroke="#825e4d"
        strokeWidth="2"
      />
      <rect
        x="222"
        y="250"
        width="74"
        height="58"
        rx="5"
        fill="#ede3c6"
        stroke="#8f8a6a"
        strokeWidth="2.5"
      />
      <path
        d="m259 264-12 7-18-12 5 19 18 7-5 13 12-6 12 6-5-13 18-7 5-19-18 12-3-9-9 2Z"
        fill="#847955"
        stroke="#605e47"
        strokeWidth="2.5"
        strokeLinejoin="round"
      />
      <circle cx="263" cy="269" r="2" fill="#eee3be" />
      <Tank x={174} y={149} color="#d1b568" />
      <Tank x={369} y={254} color="#849e75" angle={90} />
      <Tank x={411} y={59} color="#9b9f89" angle={180} />
      <g fill="#f2dc9c" stroke="#9c8555" strokeWidth="2">
        <rect x="170" y="74" width="7" height="15" rx="3" />
        <rect x="459" y="250" width="14" height="7" rx="3" />
        <path d="m341 171 7 13 14-4-5 14 12 9-15 3-1 15-11-10-13 8 3-15-14-7 16-5Z" />
      </g>
      <g fill="#b98971" stroke="#825e4d" strokeWidth="2">
        <path d="m365 178 8 3-3 8-8-3ZM305 187l9-3 3 7-9 4ZM365 215l7-1 2 7-7 2Z" />
      </g>
    </svg>
  );
}
