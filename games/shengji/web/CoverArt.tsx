import { useId } from "react";

// Tractor climbing toward an ace: a landscape and an ascending silhouette.
export function CoverArt() {
  const grain = useId();
  return (
    <svg
      viewBox="0 0 520 260"
      preserveAspectRatio="xMidYMid meet"
      style={{ background: "#dbe5e9" }}
      aria-hidden="true"
    >
      <defs>
        <pattern id={grain} width="9" height="9" patternUnits="userSpaceOnUse">
          <circle cx="2" cy="3" r=".6" fill="#45647a" opacity=".12" />
        </pattern>
      </defs>
      <rect width="520" height="260" fill="#dbe5e9" />
      <circle cx="406" cy="52" r="31" fill="#f2dfae" />
      <path
        d="M15 132l70-82 81 86 68-107 104 112 60-70 89 77 33-27v139H0Z"
        fill="#becfd0"
      />
      <path d="M153 143l81-114 32 55-29-11-11 20-18-9Z" fill="#ecf0df" />
      <path d="M0 196q97-67 206-11T399 145l121 24v91H0Z" fill="#99b6aa" />
      <path d="M0 235q134-42 228-6l110-39 182 22v48H0Z" fill="#809e8a" />
      <g fill="none" stroke="#eee5c4" strokeWidth="3" strokeLinecap="round">
        <path d="M17 211q50-18 78-17M25 223l52-15M421 181l59 8" />
      </g>
      {/* A broad staircase rises across the scene rather than a tabletop. */}
      <g stroke="#9b895d" strokeWidth="2.2" strokeLinejoin="round">
        <path
          d="M211 232v-31h50v-32h50v-32h50v-32h50V73h46v159Z"
          fill="#bdab7c"
        />
        <path
          d="M211 201h50v-32h50v-32h50v-32h50V73h46l22 10h-46v32h-50v32h-50v32h-50v32h-50Z"
          fill="#f1dfaa"
        />
        <path
          d="M233 211h50v-32h50v-32h50v-32h50V83h46v149H233Z"
          fill="#d8c28b"
        />
        <path
          d="M234 211h47m4-32h46m4-32h45m5-32h44m-73 78h55"
          fill="none"
          stroke="#b19b66"
        />
      </g>
      <g
        stroke="#6d7867"
        strokeWidth="2.5"
        strokeLinecap="round"
        strokeLinejoin="round"
      >
        <path d="M448 74V25" fill="none" strokeWidth="4" />
        <path
          d="M450 27q23-10 47 1l-7 17q-19-8-40 1Z"
          fill="#d19b55"
          stroke="#aa834e"
        />
        <path
          d="M462 39l5-10 5 10m-8-3h6"
          fill="none"
          stroke="#fff4d1"
          strokeWidth="2"
        />
      </g>
      {/* The towing vehicle is deliberately oversized for small lobby cards. */}
      <g
        transform="translate(220 154) rotate(-12)"
        stroke="#405a52"
        strokeWidth="2.8"
        strokeLinecap="round"
        strokeLinejoin="round"
      >
        <path d="M-96 44h-36" fill="none" strokeWidth="5" />
        <path d="M-143 8h-64l7 35h54Z" fill="#b47e55" stroke="#796548" />
        <path
          d="M-201 18h54m-51 10h51"
          fill="none"
          stroke="#d7b889"
          strokeWidth="2"
        />
        <g stroke="#6d7967" strokeWidth="1.7">
          <g transform="translate(-203 -48) rotate(-7 17 28)">
            <rect width="34" height="53" rx="3" fill="#fff6df" />
            <path
              d="M17 11C-1 28 10 35 17 27c7 8 18 1 0-16ZM12 40h10l-5-15Z"
              fill="#466457"
              stroke="none"
            />
          </g>
          <g transform="translate(-168 -45) rotate(6 17 28)">
            <rect width="34" height="53" rx="3" fill="#fff6df" />
            <path
              d="M17 11C-1 28 10 35 17 27c7 8 18 1 0-16ZM12 40h10l-5-15Z"
              fill="#466457"
              stroke="none"
            />
          </g>
        </g>
        <circle cx="-177" cy="47" r="16" fill="#4f5d53" />
        <circle cx="-177" cy="47" r="7" fill="#d1bd84" strokeWidth="2" />
        <path d="M-96 37V-14h51v19h49q13 0 15 11v22Z" fill="#709482" />
        <path d="M-96-14v-44h45v44" fill="#9fb8a4" />
        <path d="M-103-59h60v9h-60Z" fill="#54786a" />
        <path d="M-87-45h26v26h-26Z" fill="#e5eddb" strokeWidth="2" />
        <path d="M-28 4v-38h9v38" fill="#667b6a" />
        <path d="M-31-34h15v-7h-15Z" fill="#425c51" />
        <path d="M-2 10v20m-7-20v20m-7-20v20" fill="none" strokeWidth="2" />
        <path d="M7 15h8v10H7Z" fill="#edd794" />
        <path d="M-43 34h36m-92-10h-18" fill="none" strokeWidth="5" />
        <circle cx="-78" cy="37" r="32" fill="#46564c" />
        <circle cx="-78" cy="37" r="18" fill="#dfc992" />
        <circle cx="-78" cy="37" r="5" fill="#8a815f" strokeWidth="2" />
        <path
          d="M-78 10v6m-27 21h6m21 21v6m21-27h6m-8-19-5 5m-34 28-5 5m0-38 5 5m34 28 5 5"
          fill="none"
          stroke="#8c9b84"
          strokeWidth="3"
        />
        <circle cx="-1" cy="39" r="19" fill="#46564c" />
        <circle cx="-1" cy="39" r="10" fill="#dfc992" strokeWidth="2" />
      </g>
      <g fill="none" stroke="#829885" strokeWidth="3" strokeLinecap="round">
        <path d="M164 90q-11-9-3-16m13-5q-14-14-2-27" />
      </g>
      <g fill="#f3e6c0" stroke="#aa9566" strokeWidth="1.7">
        <path d="M332 113l6-10 6 10-6 10ZM383 81l5-8 5 8-5 8Z" />
      </g>
      <rect
        width="520"
        height="260"
        fill={`url(#${grain})`}
        pointerEvents="none"
      />
    </svg>
  );
}
