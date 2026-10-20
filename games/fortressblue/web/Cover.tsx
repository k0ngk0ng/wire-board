function Tank({
  x,
  y,
  color,
  facing = 1,
}: {
  x: number;
  y: number;
  color: string;
  facing?: number;
}) {
  return (
    <g
      transform={`translate(${x} ${y}) scale(${facing} 1)`}
      stroke="#48534f"
      strokeWidth="3"
      strokeLinejoin="round"
      strokeLinecap="round"
    >
      <ellipse
        cx="0"
        cy="27"
        rx="53"
        ry="8"
        fill="#566451"
        opacity=".2"
        stroke="none"
      />
      <rect x="-46" y="0" width="91" height="29" rx="14" fill="#626b60" />
      {[-29, -8, 13, 33].map((cx) => (
        <circle key={cx} cx={cx} cy="14" r="8" fill="#b5b79c" />
      ))}
      <path d="M-44 0q6-24 32-25h28q21 4 28 25l-5 10h-74Z" fill={color} />
      <path d="M-25-22q-3-29 19-33 24-2 32 24l-8 13Z" fill={color} />
      <path d="m15-42 37-25 9 12-40 26Z" fill="#d8d6b9" />
      <path d="m48-65 9-7 9 14-8 6Z" fill="#778477" />
      <path
        d="M-35-2h59"
        fill="none"
        stroke="#e4e6cf"
        strokeWidth="3"
        opacity=".7"
      />
      <path d="M-17-41q7-8 17-6" fill="none" stroke="#e4e6cf" />
      <ellipse cx="-3" cy="-30" rx="4" ry="6" fill="#e8ead8" />
      <path d="m8-30 8 3" fill="none" />
      <path
        d="M-39 16h-6m13 12v-5m20 5v-5m21 5v-5m20 5v-5m15-10h-5"
        fill="none"
        strokeWidth="2"
      />
    </g>
  );
}

export function Cover() {
  return (
    <svg
      viewBox="0 0 520 320"
      role="img"
      aria-label="疯狂坦克：两辆卡通坦克分据高低山丘，炮弹沿弧线越过山谷"
    >
      <rect width="520" height="320" fill="#dce4dc" />
      <circle cx="417" cy="66" r="32" fill="#eee0b5" />
      <g fill="#f6f0df">
        <path d="M27 69q-4-17 13-20 1-24 25-23 19 0 25 21 26-6 29 22Z" />
        <path d="M324 104q-3-15 13-18 5-20 24-16 15 2 17 16 23-4 27 18Z" />
      </g>
      <path
        d="M0 219 97 116l91 99L305 99l111 115 104-72v178H0Z"
        fill="#b6c4b0"
      />
      <path
        d="m145 212 58-59 61 57 55-49 70 58 72-66 59 43v124H145Z"
        fill="#a4b6a0"
      />
      <path
        d="M0 228q57-34 125-4 59 29 100 61l40 35H0Z"
        fill="#b6976c"
        stroke="#826e56"
        strokeWidth="3"
      />
      <path
        d="M0 230q63-32 126-4 62 31 98 59"
        fill="none"
        stroke="#80996f"
        strokeWidth="13"
      />
      <path
        d="M319 320q1-74 41-91 25-11 13-47 44-28 147-8v146Z"
        fill="#baa076"
        stroke="#826e56"
        strokeWidth="3"
      />
      <path
        d="M320 314q2-61 40-85 25-16 13-47 52-25 147-8"
        fill="none"
        stroke="#81986d"
        strokeWidth="13"
      />
      <g fill="none" stroke="#8f7a59" strokeWidth="2" opacity=".7">
        <path d="m25 262 34-6 27 9m-56 25 21-3m56-8 28 8m246-35 31-9 35 3m-90 47 29-8 24 7m52-73 26 4" />
      </g>
      <path
        d="M145 156Q256-35 399 111"
        fill="none"
        stroke="#7f8a76"
        strokeWidth="3"
        strokeLinecap="round"
        strokeDasharray="5 10"
      />
      <path
        d="m279 61 14 5m-17-18 17 5"
        stroke="#c0a56c"
        strokeWidth="3"
        strokeLinecap="round"
      />
      <g transform="translate(309 63) rotate(18)">
        <path
          d="M-13-7h18q12 7 0 14h-18Z"
          fill="#bb795f"
          stroke="#61584c"
          strokeWidth="2.5"
        />
        <path
          d="m-14-5-11-4 3 9-3 9 11-4"
          fill="#d7b574"
          stroke="#8f7658"
          strokeWidth="2"
        />
      </g>
      <Tank x={101} y={191} color="#7e9ca9" />
      <Tank x={440} y={140} color="#b98969" facing={-1} />
      <g stroke="#8c7957" strokeWidth="2.5" strokeLinejoin="round">
        <path d="m254 271 7-25 12 17 14-29 5 31 24-7-15 24Z" fill="#e0bd75" />
        <path d="m268 277 9-15 5 13 10-7-3 15Z" fill="#f5e5b2" stroke="none" />
      </g>
      <path
        d="m245 247-9-11m74 3 8-12m-51-4-2-10"
        fill="none"
        stroke="#ab9269"
        strokeWidth="3"
        strokeLinecap="round"
      />
      <g fill="#6f8c68">
        <path d="m24 221 1-20 7 14 9-17-2 21ZM478 170l6-18 4 14 11-11-4 18Z" />
      </g>
    </svg>
  );
}
