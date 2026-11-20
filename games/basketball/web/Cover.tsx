import { useId } from "react";

export function Cover() {
  const id = useId();
  return (
    <svg
      viewBox="0 0 640 380"
      role="img"
      aria-label="热血篮球：飞旋的篮球冲向街头球场的三层篮筐"
    >
      <defs>
        <linearGradient id={`${id}-sky`} x2="0" y2="1">
          <stop stopColor="#eee5cc" />
          <stop offset="1" stopColor="#d3d9be" />
        </linearGradient>
        <linearGradient id={`${id}-ball`} x1="0" y1="0" x2="1" y2="1">
          <stop stopColor="#e7b16b" />
          <stop offset=".55" stopColor="#ce8651" />
          <stop offset="1" stopColor="#ac6246" />
        </linearGradient>
        <clipPath id={`${id}-frame`}>
          <rect width="640" height="380" rx="18" />
        </clipPath>
        <clipPath id={`${id}-ball-clip`}>
          <circle cx="258" cy="183" r="79" />
        </clipPath>
      </defs>
      <g clipPath={`url(#${id}-frame)`}>
        <rect width="640" height="380" fill={`url(#${id}-sky)`} />
        <circle cx="105" cy="78" r="36" fill="#dbbf86" opacity=".65" />
        <path
          d="M0 221V139h41v-28h43v91h26v-49h42v64h30V96h48v114h28v-75h44v75h30V83h47v123h31v-65h48v58h26V104h51v97h30v-48h45v71Z"
          fill="#a6b49a"
        />
        <path
          d="M193 114h22m-22 17h22m129-29h23m-23 17h23m130 3h25M48 130h24m-24 17h24"
          fill="none"
          stroke="#839782"
          strokeWidth="5"
          opacity=".6"
        />
        <path d="M0 272 640 208v172H0Z" fill="#b29472" />
        <path d="m0 322 640-67v125H0Z" fill="#a17d61" />
        <g fill="none" stroke="#eadbbc" strokeWidth="4">
          <path d="m0 339 640-66M82 380l27-63 205-21 74 84" />
          <path d="M160 380c-12-38 19-62 67-68s99 14 109 68m112 0c-20-42 11-80 80-88" />
        </g>
        <g fill="none" stroke="#7c927b" strokeWidth="2" opacity=".7">
          <path d="m0 239 640-65M0 262l640-65M29 222v50m46-54v49m46-54v49m46-54v49m211-71v49m46-54v49m46-54v49m46-54v49m46-54v49m46-54v49" />
          <path d="m0 241 26 17 24-23 26 17 24-23 26 17 24-23 26 17m204-43 26 17 24-23 26 17 24-23 26 17 24-23 26 17 24-23 26 17" />
        </g>
        <ellipse
          cx="257"
          cy="326"
          rx="106"
          ry="16"
          fill="#645c4c"
          opacity=".2"
        />
        <g strokeLinecap="round" strokeLinejoin="round">
          <path
            d="M568 347V43m0 104-30 27m30 56-24 22m24 52-17 17"
            fill="none"
            stroke="#4d6458"
            strokeWidth="9"
          />
          <path
            d="m439 52 153-13v84l-153 14Z"
            fill="#f0e8ce"
            stroke="#516657"
            strokeWidth="4"
          />
          <path
            d="m476 76 52-5v44l-52 5Z"
            fill="none"
            stroke="#ad6d52"
            strokeWidth="4"
          />
          <path
            d="m454 148 16 43 43-4 18-45m-71 18 54 26m8-31-45 34m-11-41 12 40m22-43-5 44"
            fill="none"
            stroke="#f4e9cc"
            strokeWidth="3"
          />
          <ellipse
            cx="493"
            cy="145"
            rx="43"
            ry="9"
            fill="#b57d58"
            stroke="#85563e"
            strokeWidth="5"
          />
          <path
            d="m462 201 130-11v62l-130 12Z"
            fill="#d5d8bd"
            stroke="#516657"
            strokeWidth="4"
          />
          <path
            d="m491 217 44-4v29l-44 4Z"
            fill="none"
            stroke="#9e7557"
            strokeWidth="3"
          />
          <path
            d="m477 270 14 31 36-3 15-34m-58 14 43 19m7-26-37 28m-7-28 10 28m17-30-4 30"
            fill="none"
            stroke="#f0e5c8"
            strokeWidth="2.5"
          />
          <ellipse
            cx="510"
            cy="266"
            rx="36"
            ry="7"
            fill="#b57d58"
            stroke="#85563e"
            strokeWidth="4"
          />
          <path
            d="m487 311 105-9v42l-105 9Z"
            fill="#bcc8ad"
            stroke="#516657"
            strokeWidth="3"
          />
          <path
            d="m498 357 10 22 31-3 12-24m-46 10 33 13m6-19-29 20"
            fill="none"
            stroke="#eaddbb"
            strokeWidth="2.5"
          />
          <ellipse
            cx="525"
            cy="353"
            rx="29"
            ry="6"
            fill="#b57d58"
            stroke="#85563e"
            strokeWidth="4"
          />
        </g>
        <path
          d="M215 116c-65 21-106 68-164 72l36 24-56 42c60 1 96-6 137-24l-44 57c66-20 119-39 155-78Z"
          fill="#c1945c"
        />
        <path
          d="M227 134c-50 20-72 55-120 66l28 15-55 29c66-10 85-23 122-44l-34 48 91-48Z"
          fill="#e0ba77"
        />
        <g fill="none" strokeLinecap="round">
          <path
            d="m67 152 55-23m-70 98 41-15m13 88 49-24"
            stroke="#a77b53"
            strokeWidth="5"
          />
          <path
            d="M353 153c24-3 47-1 65 7m-63-27 28 1"
            stroke="#eee2c1"
            strokeWidth="5"
          />
        </g>
        <g transform="rotate(-24 258 183)">
          <circle cx="258" cy="183" r="79" fill={`url(#${id}-ball)`} />
          <g
            clipPath={`url(#${id}-ball-clip)`}
            fill="none"
            stroke="#684e3c"
            strokeWidth="5"
          >
            <path d="M179 183h158M258 104v158M205 110c67 22 69 122 0 146m107-146c-67 22-69 122 0 146" />
            <path
              d="M195 154c9-19 22-30 39-35"
              stroke="#f4d39a"
              strokeWidth="8"
              strokeLinecap="round"
            />
          </g>
          <circle
            cx="258"
            cy="183"
            r="79"
            fill="none"
            stroke="#684e3c"
            strokeWidth="6"
          />
        </g>
        <g fill="none" stroke="#f7edce" strokeWidth="4" strokeLinecap="round">
          <path d="m360 92 12-16m15 29 21-8m-54-20 3-14" />
        </g>
      </g>
    </svg>
  );
}
