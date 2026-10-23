import { useId } from "react";

export function Cover() {
  const id = useId().replaceAll(":", "");
  return (
    <svg
      viewBox="0 0 520 260"
      role="img"
      aria-label="海图上的彩色疆域、城堡与航线"
    >
      <defs>
        <linearGradient id={`${id}-sea`} x2="1" y2="1">
          <stop stopColor="#c1d4cf" />
          <stop offset="1" stopColor="#90b8b9" />
        </linearGradient>
        <pattern
          id={`${id}-waves`}
          width="26"
          height="19"
          patternUnits="userSpaceOnUse"
        >
          <path
            d="M3 11q5-5 10 0t10 0"
            fill="none"
            stroke="#eef1df"
            strokeWidth="1"
            opacity=".35"
          />
        </pattern>
      </defs>
      <rect width="520" height="260" rx="12" fill={`url(#${id}-sea)`} />
      <rect width="520" height="260" rx="12" fill={`url(#${id}-waves)`} />
      <path
        d="m39 64 27-26 41 6 15-13 34 12 24-7 27 22 4 27-21 19 17 19-15 26-31 8-17 39-31 28-16-19 7-29-26-19-10-30-28-10Z"
        fill="#d8d1a9"
        stroke="#536f61"
        strokeWidth="2"
        strokeLinejoin="round"
      />
      <path
        d="m122 31 34 12 24-7 27 22 4 27-21 19-28-15-21 3-12-22-26-26Z"
        fill="#b1bf90"
        stroke="#536f61"
        strokeWidth="1.5"
      />
      <path
        d="m141 92 21-3 28 15 17 19-15 26-31 8-11-23-21-5Z"
        fill="#ba7f69"
        stroke="#536f61"
        strokeWidth="1.5"
      />
      <path
        d="m161 157-17 39-31 28-16-19 7-29 26-12 2-25 18-5Z"
        fill="#d0ab66"
        stroke="#536f61"
        strokeWidth="1.5"
      />
      <path
        d="m285 57 28-22 32 9 21-11 20 16 36-2 39 30-5 27 27 16-9 25-39 1-24 23-35-14-15 17-30-11-25-40-30-5-12-24Z"
        fill="#b8c89c"
        stroke="#536f61"
        strokeWidth="2"
        strokeLinejoin="round"
      />
      <path
        d="m285 57 28-22 32 9 21-11 20 16-16 34-25 7-11 31-28-3-30-2-12-24Z"
        fill="#caa879"
        stroke="#536f61"
        strokeWidth="1.5"
      />
      <path
        d="m370 83 16-34 36-2 39 30-5 27-37-6-20 19-24-12Z"
        fill="#8cabbe"
        stroke="#536f61"
        strokeWidth="1.5"
      />
      <path
        d="m334 121 11-31 25-7 5 22 24 12 20-19 37 6 27 16-9 25-39 1-24 23-35-14-15 17-30-11Z"
        fill="#b897b0"
        stroke="#536f61"
        strokeWidth="1.5"
      />
      <path
        d="m330 164 31 8 15-17 23 35-13 33-23 10-18-21Z"
        fill="#b5c49b"
        stroke="#536f61"
        strokeWidth="2"
      />
      <path
        d="M176 118Q234 52 315 84M141 181Q257 232 356 203"
        fill="none"
        stroke="#416d70"
        strokeWidth="2"
        strokeDasharray="5 7"
      />
      <g
        fill="#ece5cf"
        stroke="#605f4e"
        strokeWidth="1.8"
        strokeLinejoin="round"
      >
        <path d="M154 116V94h7v7h7v-7h7v22m-19-15h17m-26 26v-13h9v-9h17v9h9v13Z" />
        <path d="M309 85V63h7v7h7v-7h7v22m-19-15h17m-26 26V83h9v-9h17v9h9v13Z" />
        <path d="M348 202v-15h7v-10h13v10h7v15Z" />
      </g>
      <g stroke="#65533f" strokeWidth="2">
        <path d="M168 94V71m0 0 18 5-18 7" fill="#a6503c" />
        <path d="M322 63V40m0 0 18 5-18 7" fill="#547b89" />
      </g>
      <g transform="translate(240 155) rotate(-12)">
        <path
          d="m-15 11 7 9h23l9-9Z"
          fill="#806b4e"
          stroke="#465d57"
          strokeWidth="1.5"
        />
        <path
          d="M4 11v-31M1-18-15 6H1Zm6 2V6h18Z"
          fill="#f2edda"
          stroke="#607366"
          strokeWidth="1.5"
        />
      </g>
      <g transform="translate(458 216)" fill="none" stroke="#4a7374">
        <circle r="19" strokeWidth="1.2" />
        <path
          d="m0-28 5 23 22 5-22 5-5 23-5-23-22-5 22-5Z"
          fill="#dce2ce"
          strokeWidth="1.2"
        />
        <path d="M0-25V25M-25 0H25" />
      </g>
    </svg>
  );
}
