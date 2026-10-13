import { useId } from "react";
export function CoverArt() {
  const id = useId();
  return (
    <svg
      viewBox="0 0 520 260"
      preserveAspectRatio="xMidYMid slice"
      aria-hidden="true"
    >
      <defs>
        <pattern id={id} width="9" height="9" patternUnits="userSpaceOnUse">
          <circle cx="2" cy="2" r=".55" fill="#314d40" opacity=".13" />
        </pattern>
      </defs>
      <rect width="520" height="260" fill="#d4dfea" />
      <rect width="520" height="260" fill={`url(#${id})`} />
      <circle cx="432" cy="50" r="27" fill="#f2dca4" />
      <path
        d="M0 218Q142 161 276 225T520 206V260H0Z"
        fill="#506a80"
        opacity=".19"
      />
      <ellipse
        cx="246"
        cy="192"
        rx="176"
        ry="43"
        fill="#506a80"
        opacity=".25"
      />
      <path
        d="M94 139L284 74 408 159 224 232Z"
        fill="#506a80"
        stroke="#f5ecd3"
        strokeWidth="3"
      />
      <g stroke="#8c784d" strokeWidth="2" fill="#d9bc7c">
        <path d="M349 218v-28h30v-28h30v-28h30v-28h30v112Z" />
        <path d="M365 91l50-35 41 13-43 38Z" fill="#f0d99e" />
        <path d="M400 99v41" stroke="#596851" strokeWidth="5" />
      </g>
      <g
        transform="translate(139 78) rotate(-19 55 75)"
        stroke="#796d58"
        strokeWidth="2"
      >
        <rect x="-12" y="10" width="91" height="130" rx="7" fill="#d8d4bf" />
        <rect width="91" height="130" rx="7" fill="#fff7e4" />
        <path d="M10 19l7-10 7 10-7 10Z" fill="#c3a25f" stroke="none" />
        <path
          d="M33 40l16-22 16 22-16 22ZM33 85l16-22 16 22-16 22Z"
          fill="#c3a25f"
          stroke="none"
        />
      </g>
      <g
        transform="translate(240 80) rotate(14 50 65)"
        stroke="#796d58"
        strokeWidth="2"
      >
        <rect x="9" y="9" width="91" height="130" rx="7" fill="#d9d6c4" />
        <rect width="91" height="130" rx="7" fill="#fff8e9" />
        <path
          d="M17 9C-4 28 11 34 17 27c6 7 21 1 0-18ZM13 33h8l-4-7Z"
          fill="#506a80"
          stroke="none"
        />
        <path
          d="M45 29C-1 74 34 93 45 75c11 18 46-1 0-46ZM35 95h20L45 72Z"
          fill="#506a80"
          stroke="none"
        />
      </g>
      <g stroke="#8d9b82" strokeWidth="2" fill="#f6edd6">
        <ellipse cx="76" cy="98" rx="23" ry="8" />
        <path d="M53 98v20c0 17 46 17 46 0V98" />
        <path d="M99 101c25-4 22 24 0 20" fill="none" />
        <ellipse cx="76" cy="98" rx="18" ry="5" fill="#aa8360" />
      </g>
      <path
        d="M35 35h72M35 43h40"
        stroke="#506a80"
        strokeWidth="2"
        opacity=".45"
      />
    </svg>
  );
}
