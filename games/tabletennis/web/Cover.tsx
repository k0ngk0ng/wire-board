import { useId } from "react";
export function Cover() {
  const id = useId();
  return (
    <svg
      viewBox="0 0 640 360"
      role="img"
      aria-label="乒乓球台与四支球拍"
      xmlns="http://www.w3.org/2000/svg"
    >
      <defs>
        <linearGradient id={id} x2="0" y2="1">
          <stop stopColor="#e3dfca" />
          <stop offset="1" stopColor="#c2cfb1" />
        </linearGradient>
      </defs>
      <rect width="640" height="360" fill={`url(#${id})`} />
      <path d="M0 292 430 135 640 223V360H0Z" fill="#b9a17a" opacity=".5" />
      <path
        d="m133 194 290-94 144 91-299 128Z"
        fill="#346b5b"
        stroke="#234e43"
        strokeWidth="9"
        strokeLinejoin="round"
      />
      <path
        d="m148 194 274-85 127 81-280 115Z"
        fill="none"
        stroke="#e8e6c8"
        strokeWidth="3"
      />
      <path
        d="m352 122-44 176M264 151l129 91"
        stroke="#e3e6cf"
        strokeWidth="2"
      />
      <path
        d="m263 152 129 91v-35l-128-88z"
        fill="#e8e5cb"
        fillOpacity=".3"
        stroke="#f2edd4"
        strokeWidth="3"
      />
      <path
        d="m268 128 120 75m-118-65 117 74m-118-65 118 75m-98-91v32m22-20v32m22-19v32m22-19v32"
        stroke="#ece8cf"
        strokeWidth="1.4"
      />
      <g transform="translate(120 238) rotate(-30)">
        <rect x="-10" y="37" width="20" height="70" rx="7" fill="#d2b172" />
        <ellipse
          rx="51"
          ry="62"
          fill="#bd6652"
          stroke="#eee2b8"
          strokeWidth="7"
        />
      </g>
      <g transform="translate(502 276) rotate(26)">
        <rect x="-8" y="30" width="16" height="52" rx="5" fill="#d2b172" />
        <ellipse
          rx="38"
          ry="46"
          fill="#254f43"
          stroke="#eee2b8"
          strokeWidth="6"
        />
      </g>
      <g transform="translate(368 75) rotate(22)">
        <rect x="-5" y="18" width="10" height="32" rx="3" fill="#c9a86b" />
        <ellipse
          rx="21"
          ry="26"
          fill="#b8654c"
          stroke="#eddfbb"
          strokeWidth="4"
        />
      </g>
      <g transform="translate(464 100) rotate(-20)">
        <rect x="-5" y="18" width="10" height="32" rx="3" fill="#c9a86b" />
        <ellipse
          rx="21"
          ry="26"
          fill="#b8654c"
          stroke="#eddfbb"
          strokeWidth="4"
        />
      </g>
      <path
        d="M204 182q59-84 115-7"
        fill="none"
        stroke="#f3ebc9"
        strokeWidth="4"
        strokeLinecap="round"
      />
      <circle
        cx="321"
        cy="177"
        r="12"
        fill="#fff8e4"
        stroke="#62806b"
        strokeWidth="2"
      />
    </svg>
  );
}
