import { useId } from "react";
export function Cover() {
  const id = useId().replaceAll(":", "");
  return (
    <svg viewBox="0 0 640 360" aria-hidden="true">
      <defs>
        <linearGradient id={id + "bg"} x2="1" y2="1">
          <stop stopColor="#9eaaa0" />
          <stop offset="1" stopColor="#536b67" />
        </linearGradient>
        <radialGradient id={id + "glow"}>
          <stop stopColor="#f7d28a" stopOpacity=".8" />
          <stop offset="1" stopColor="#f7d28a" stopOpacity="0" />
        </radialGradient>
      </defs>
      <path fill={"url(#" + id + "bg)"} d="M0 0h640v360H0z" />
      <path d="M0 282 320 226l320 50v84H0z" fill="#64756b" />
      <path
        d="M0 309 640 318M0 341l640-52M120 263l-32 97m131-115 3 115m146-126 24 126m120-111 35 111"
        stroke="#b6b5a0"
        strokeWidth="3"
        opacity=".5"
      />
      <path
        d="M179 278V101q0-61 71-61h126q75 0 75 65v179"
        fill="#3b5450"
        stroke="#b3b6a0"
        strokeWidth="9"
      />
      <path
        d="M215 279V111q0-36 41-36h111q44 0 44 38v166"
        fill="#243c3a"
        stroke="#9b9d7f"
        strokeWidth="6"
      />
      <path
        d="M239 273V116q0-17 20-17h99q27 0 27 19v155z"
        fill="#9b9b74"
        stroke="#c2b88c"
        strokeWidth="5"
      />
      <path d="M312 102v169m-65-108h129" stroke="#575f4d" strokeWidth="7" />
      <path
        d="m278 194 30-15 32 15-5 42h-53z"
        fill="#4c6259"
        stroke="#d6c58b"
        strokeWidth="5"
      />
      <circle cx="309" cy="208" r="10" fill="#ddc58c" />
      <path d="m309 209-5 17h10z" fill="#ddc58c" />
      <path
        d="M125 67h-35v177h62m357-165h42v161h-53"
        fill="none"
        stroke="#a9ad91"
        strokeWidth="14"
        strokeLinecap="round"
      />
      <circle cx="134" cy="197" r="81" fill={"url(#" + id + "glow)"} />
      <g transform="translate(133 194) rotate(-7)">
        <path
          d="M-16-26v-17q0-19 18-19t18 20v16"
          stroke="#d8bb75"
          strokeWidth="7"
          fill="none"
        />
        <path
          d="M-30-27h61l-8 66h-46z"
          fill="#ead08d"
          stroke="#656f57"
          strokeWidth="6"
        />
        <path d="M-18-17h36l-5 45h-26z" fill="#f8e8b5" />
        <path
          d="M-36 41h75"
          stroke="#4b6052"
          strokeWidth="9"
          strokeLinecap="round"
        />
      </g>
      <g transform="translate(480 271) rotate(10)">
        <path
          d="m-27-42 60 9-6 49-60-9z"
          fill="#a68a63"
          stroke="#475f52"
          strokeWidth="5"
        />
        <path d="m-20-35 45 7-5 32-46-6z" fill="#e0d3a5" />
        <path d="m-10-22 22 4m-24 6 22 4" stroke="#7b856b" strokeWidth="3" />
      </g>
      <path
        d="m43 351 5-37 19 21m508 17 7-26 15 16"
        fill="none"
        stroke="#9da47e"
        strokeWidth="5"
      />
    </svg>
  );
}
