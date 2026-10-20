const pieces = [
  {
    color: "#bd7360",
    cells: [
      [0, 5],
      [1, 5],
      [1, 4],
      [2, 4],
    ],
  },
  {
    color: "#d8b45f",
    cells: [
      [0, 2],
      [1, 2],
      [0, 3],
      [1, 3],
    ],
  },
  {
    color: "#839b74",
    cells: [
      [2, 5],
      [3, 5],
      [4, 5],
      [4, 4],
    ],
  },
  {
    color: "#9882a4",
    cells: [
      [5, 5],
      [6, 5],
      [7, 5],
      [6, 4],
    ],
  },
  {
    color: "#6e9ca9",
    cells: [
      [7, 4],
      [7, 3],
      [7, 2],
      [7, 1],
    ],
  },
  {
    color: "#be8d59",
    cells: [
      [5, 4],
      [5, 3],
      [5, 2],
      [6, 2],
    ],
  },
];

function Block({ x, y, color }: { x: number; y: number; color: string }) {
  return (
    <g transform={`translate(${x} ${y})`} strokeLinejoin="round">
      <rect
        width="35"
        height="35"
        rx="4"
        fill={color}
        stroke="#4b514c"
        strokeWidth="2.5"
      />
      <path
        d="M5 29V6h24"
        fill="none"
        stroke="#fff8e8"
        strokeWidth="2.5"
        opacity=".65"
      />
      <path
        d="M30 8v22H9"
        fill="none"
        stroke="#49534c"
        strokeWidth="2"
        opacity=".3"
      />
    </g>
  );
}

export function Cover() {
  return (
    <svg
      viewBox="0 0 520 320"
      role="img"
      aria-label="俄罗斯方块：彩色积木高低堆叠，青色长条落入空隙"
    >
      <rect width="520" height="320" fill="#eee7d6" />
      <circle cx="421" cy="77" r="69" fill="#e0dac5" />
      <path d="M31 261q214-23 458 0v59H31Z" fill="#d6d1bc" />
      <g fill="none" stroke="#c6c6b4" strokeWidth="1.2">
        <path d="M86 66v212M123 66v212M160 66v212M197 66v212M234 66v212M271 66v212M308 66v212M345 66v212M382 66v212" />
        <path d="M86 92h296M86 129h296M86 166h296M86 203h296M86 240h296" />
      </g>
      <path
        d="M76 63v218q0 10 12 10h302q12 0 12-12V63"
        fill="none"
        stroke="#797e6b"
        strokeWidth="5"
        strokeLinecap="round"
      />
      <path
        d="M68 292h340"
        stroke="#b2b39d"
        strokeWidth="6"
        strokeLinecap="round"
      />
      {pieces.flatMap((piece, i) =>
        piece.cells.map(([x, y], j) => (
          <Block
            key={`${i}-${j}`}
            x={87 + x * 37}
            y={56 + y * 37}
            color={piece.color}
          />
        )),
      )}
      <g transform="translate(199 17) rotate(-3 18 74)">
        {[0, 1, 2, 3].map((n) => (
          <Block key={n} x={0} y={n * 37} color="#73a8ae" />
        ))}
      </g>
      <path
        d="M211 181v30m-8-8 8 9 8-9"
        fill="none"
        stroke="#61939a"
        strokeWidth="3"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
      <g fill="none" stroke="#b99a62" strokeWidth="3" strokeLinecap="round">
        <path d="m53 116 4 9 9 4-9 4-4 9-4-9-9-4 9-4ZM441 187l5 11 11 5-11 5-5 11-5-11-11-5 11-5Z" />
        <path d="m421 244 13-5m-5 22 13 2M48 220l8 5" />
      </g>
      <g transform="translate(424 83) rotate(12)">
        <rect
          width="28"
          height="28"
          rx="4"
          fill="#9882a4"
          stroke="#4b514c"
          strokeWidth="2.5"
        />
        <rect
          x="28"
          width="28"
          height="28"
          rx="4"
          fill="#9882a4"
          stroke="#4b514c"
          strokeWidth="2.5"
        />
        <rect
          x="56"
          width="28"
          height="28"
          rx="4"
          fill="#9882a4"
          stroke="#4b514c"
          strokeWidth="2.5"
        />
        <rect
          x="28"
          y="28"
          width="28"
          height="28"
          rx="4"
          fill="#9882a4"
          stroke="#4b514c"
          strokeWidth="2.5"
        />
        <path
          d="M5 22V5h16m12 17V5h16m12 17V5h16m-44 45V33h16"
          fill="none"
          stroke="#ede2e9"
          strokeWidth="2"
        />
      </g>
    </svg>
  );
}
