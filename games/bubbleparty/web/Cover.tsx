function Neighbour({
  x,
  y,
  color,
  turn = 1,
}: {
  x: number;
  y: number;
  color: string;
  turn?: number;
}) {
  return (
    <g
      transform={`translate(${x} ${y}) scale(${turn} 1)`}
      stroke="#4b6267"
      strokeWidth="3"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <ellipse
        cy="51"
        rx="35"
        ry="9"
        fill="#607e72"
        opacity=".2"
        stroke="none"
      />
      <path
        d="M-21 34q-15 6-13 15 12 6 27-1M9 34q13 0 20 10-1 10-23 6"
        fill={color}
      />
      <path d="M-22 9q-10 19 0 30 18 13 37-1 12-16-1-30Z" fill={color} />
      <path
        d="M-21 16q-22-2-25 9 3 11 18 1m39-7q15-13 24-5 5 12-12 16"
        fill="#f3d9bb"
      />
      <path d="M-37-13q-3-35 34-39 38 0 39 35l-6 30-58 2Z" fill={color} />
      <path d="M-27-17q27-15 52 1l-2 25q-23 22-46 0Z" fill="#f6dfc0" />
      <path
        d="M-28-20q22-12 51-1"
        fill="none"
        stroke="#f9f0d9"
        strokeWidth="4"
      />
      <ellipse cx="-11" cy="-4" rx="3" ry="5" fill="#344d55" stroke="none" />
      <ellipse cx="11" cy="-4" rx="3" ry="5" fill="#344d55" stroke="none" />
      <path d="M-7 8q7 6 14 0" fill="none" strokeWidth="2.5" />
      <path d="M-9-49q8-13 18 0" fill={color} />
      <path d="M-8 23h16" stroke="#f9f0d9" strokeWidth="6" />
    </g>
  );
}

export function Cover() {
  return (
    <svg
      className="bubble-cover"
      viewBox="0 0 520 320"
      role="img"
      aria-label="泡泡堂：戴帽伙伴穿过住宅小区，透明水泡炸出十字水柱"
    >
      <rect width="520" height="320" fill="#dfe9e0" />
      <circle cx="429" cy="52" r="30" fill="#f2e6ba" />
      <path d="M0 142q113-38 230-12t290-9v199H0Z" fill="#b8c6a0" />
      <g stroke="#6d8172" strokeWidth="2.5" strokeLinejoin="round">
        <path d="M48 55h89v70H48Z" fill="#efe0bd" />
        <path d="m36 57 55-36 60 36Z" fill="#b67b6c" />
        <path d="M67 78h20v22H67Zm39 0h18v47h-18Z" fill="#90b4b9" />
        <path d="M340 51h101v73H340Z" fill="#e9d8b8" />
        <path d="m329 53 59-34 65 34Z" fill="#8ba3a4" />
        <path d="M354 75h23v23h-23Zm42 0h26v49h-26Z" fill="#9db8b6" />
        <path
          d="M14 112v30m17-33v29m129-29v27m17-30v29m285-25v27m17-29v29M8 120h33m111-3h35m267 1h34"
          fill="none"
          stroke="#f5ead1"
          strokeWidth="6"
        />
      </g>
      <g fill="#88a77f" stroke="#6d896e" strokeWidth="2.5">
        <path d="M207 126q-21-19-9-34 4-7 12-6-4-22 14-23 20 0 21 23 17-2 20 16 1 23-58 24Z" />
        <path d="M476 143q-23-16-12-31 3-8 12-8-3-20 12-20 15 0 19 20 17 3 13 22-6 19-44 17Z" />
      </g>
      <path d="M0 171 520 153v137L0 304Z" fill="#dfd8bd" />
      <g fill="none" stroke="#bdbea6" strokeWidth="2">
        <path d="m0 213 520-16M0 258l520-15M73 169 47 302M168 165l-11 134M265 161v136M363 159l13 133M454 156l25 134" />
      </g>
      <g stroke="#876f54" strokeWidth="2.5" strokeLinejoin="round">
        <path d="m20 165 35-8 29 11v35l-35 10-29-12Z" fill="#bf9b68" />
        <path
          d="m20 165 29 10 35-7m-35 7v38m-22-37 16 6m12 2 23-7m-51 13 16 6m12 1 23-6"
          fill="none"
        />
        <path d="m413 157 34-7 27 10v34l-34 9-27-12Z" fill="#c1a271" />
        <path
          d="m413 157 27 10 34-7m-34 7v36m-20-36 14 5m12 2 22-6m-48 13 14 5m12 1 22-6"
          fill="none"
        />
      </g>
      <path
        d="M282 143q-12-10-22 1l-1 54-54 2q-12 9 0 20l54-1v62q10 12 22 0v-62l69-3q11-10-1-21l-66 2Z"
        fill="#8fc6ce"
        stroke="#5b9cab"
        strokeWidth="3"
        strokeLinejoin="round"
      />
      <path
        d="m270 150 1 124m-60-64 134-4"
        fill="none"
        stroke="#dff1e9"
        strokeWidth="7"
        strokeLinecap="round"
      />
      <Neighbour x={141} y={217} color="#bb776c" />
      <Neighbour x={377} y={246} color="#759eaf" turn={-1} />
      <g stroke="#5994a6" strokeWidth="3">
        <circle cx="224" cy="247" r="33" fill="#aad4d8" fillOpacity=".65" />
        <path
          d="M204 244q-1-19 19-21"
          fill="none"
          stroke="#f5fbef"
          strokeWidth="5"
          strokeLinecap="round"
        />
        <path d="M234 271q15-7 16-22" fill="none" stroke="#75b0be" />
        <circle cx="309" cy="114" r="17" fill="#c1dfe0" fillOpacity=".7" />
        <path d="M299 111q1-7 7-8" fill="none" stroke="#f5fbef" />
        <circle cx="327" cy="154" r="7" fill="#a9d1d6" />
      </g>
      <g fill="#b6dde0" stroke="#6da8b7" strokeWidth="2">
        <path d="m245 129-5-11q-10 7-6 12 4 4 11-1Zm51 158 10 7q5-10 0-13-5-2-10 6ZM192 226l-11 3q1 11 8 7 5-2 3-10Z" />
      </g>
    </svg>
  );
}
