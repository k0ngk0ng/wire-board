import { useId } from "react";
export function Cover() {
  const id = useId();
  return (
    <svg
      viewBox="0 0 640 380"
      role="img"
      aria-label="灵丸的光芒照亮灵界庭院，两位武者对峙"
    >
      <defs>
        <linearGradient id={id + "-sky"} x2="0" y2="1">
          <stop stopColor="#d8dccb" />
          <stop offset="1" stopColor="#aebfb3" />
        </linearGradient>
        <radialGradient id={id + "-spirit"}>
          <stop stopColor="#f8e8b6" />
          <stop offset=".45" stopColor="#a9cfc0" />
          <stop offset="1" stopColor="#6a9a95" stopOpacity="0" />
        </radialGradient>
      </defs>
      <rect width="640" height="380" rx="18" fill={"url(#" + id + "-sky)"} />
      <circle cx="496" cy="72" r="42" fill="#e8d3a5" />
      <path
        d="M0 214q82-50 166-10t158-17q87-53 165-9t151 14v188H0Z"
        fill="#73918a"
      />
      <path d="M0 276 640 228v152H0Z" fill="#b99d7e" />
      <path
        d="M0 321 640 274M84 380l67-112m173 112 11-128m211 128-56-140"
        stroke="#d7c6a4"
        strokeWidth="3"
        fill="none"
      />
      <g fill="#647c79" stroke="#4e6765" strokeWidth="3" strokeLinejoin="round">
        <path d="M356 183v-68h151v68M337 117l93-42 97 42Z" />
        <path d="m323 116 107-46 113 43-12 13H337Z" fill="#7d8d7f" />
        <path d="M383 184v-53h40v53m19 0v-53h40v53" fill="#a6b6a0" />
      </g>
      <path
        d="M0 234h127m37-5h111m272-23h93M27 239v39m56-44v40m487-64v38m46-40v37"
        stroke="#687e73"
        strokeWidth="7"
        strokeLinecap="round"
      />
      <ellipse cx="189" cy="341" rx="89" ry="17" fill="#5c6f68" opacity=".3" />
      <ellipse cx="478" cy="318" rx="69" ry="14" fill="#5c6f68" opacity=".25" />
      <g
        stroke="#425d59"
        strokeWidth="5"
        strokeLinecap="round"
        strokeLinejoin="round"
      >
        <path
          d="m169 249-33 77-26 13 53 2 39-67 25 49 48 9-31-23-27-73Z"
          fill="#637e6b"
        />
        <path d="m156 177-19 74 58 25 38-53-13-47Z" fill="#528370" />
        <path
          d="m153 183-35 35 29 10 34-26m35-16 32 23 43-12-5-18-39 4-23-25"
          fill="#c7a98e"
        />
        <path d="m183 144-7 33 31 17 18-30-11-30Z" fill="#ceb394" />
        <path
          d="m174 156-12-20 17 1-4-17 16 7 11-21 10 19 16-5-4 20 15 2-16 20-11-15-14 10-5-12Z"
          fill="#365f55"
        />
        <path
          d="m187 174 17 12-19 20"
          fill="none"
          stroke="#b2c3a3"
          strokeWidth="4"
        />
        <path
          d="m474 231-32 68-34 15 47 1 31-49 17 36 47 6-31-24-13-61Z"
          fill="#66576c"
        />
        <path d="m465 166-18 71 45 14 29-52-16-36Z" fill="#807286" />
        <path
          d="m462 177-34 16-23-22-11 15 32 32 38-15m40-25 30 32-22 25-15-12 11-15-20-16"
          fill="#b9a58f"
        />
        <path d="m479 128-8 35 26 13 19-31-12-23Z" fill="#c4ad92" />
        <path
          d="m473 145-10-24 22 5 8-17 10 16 19 1-8 23-12-11-7 12-10-14Z"
          fill="#55546a"
        />
      </g>
      <circle cx="293" cy="191" r="69" fill={"url(#" + id + "-spirit)"} />
      <path
        d="M265 199q52-42 109-18m-101 23q48-21 88-10"
        stroke="#e3dfbd"
        strokeWidth="7"
        strokeLinecap="round"
        fill="none"
      />
      <circle
        cx="283"
        cy="193"
        r="14"
        fill="#f1e3b5"
        stroke="#c5d5b9"
        strokeWidth="5"
      />
      <g fill="#d6e0c8">
        <ellipse cx="83" cy="99" rx="9" ry="17" transform="rotate(-25 83 99)" />
        <ellipse
          cx="325"
          cy="55"
          rx="5"
          ry="11"
          transform="rotate(32 325 55)"
        />
        <ellipse
          cx="564"
          cy="144"
          rx="6"
          ry="13"
          transform="rotate(44 564 144)"
        />
      </g>
    </svg>
  );
}
