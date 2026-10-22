export function SanguoshaCover() {
  return (
    <svg preserveAspectRatio="xMidYMid slice" viewBox="0 0 520 260">
      <rect width="520" height="260" fill="#e9d8b5" />
      <circle cx="413" cy="57" r="38" fill="#bd6650" />
      <path
        d="M0 161L93 91 182 158 281 80 399 150 520 109V260H0"
        fill="#b1b899"
      />
      <path d="M0 203Q120 160 260 201T520 178V260H0" fill="#71866a" />
      {[
        { x: 124, y: 38, r: -13, c: "#577c94", v: "魏" },
        { x: 218, y: 29, r: 0, c: "#b6604d", v: "蜀" },
        { x: 314, y: 40, r: 13, c: "#477a62", v: "吴" },
      ].map((c) => (
        <g
          key={c.v}
          transform={`translate(${c.x} ${c.y}) rotate(${c.r} 40 85)`}
        >
          <path d="M6 9H88V186H6Z" fill="#283e37" opacity=".18" />
          <rect width="82" height="176" rx="7" fill="#fbf1d9" />
          <rect x="6" y="6" width="70" height="164" rx="4" fill={c.c} />
          <path
            d="M17 140L41 104 65 140M41 42V100M24 67H58"
            fill="none"
            stroke="#eed8a3"
            strokeWidth="4"
          />
          <circle
            cx="41"
            cy="60"
            r="24"
            fill={c.c}
            stroke="#eed8a3"
            strokeWidth="2"
          />
          <text
            x="41"
            y="70"
            textAnchor="middle"
            fill="#fff1cf"
            fontFamily="serif"
            fontSize="28"
          >
            {c.v}
          </text>
        </g>
      ))}
    </svg>
  );
}
