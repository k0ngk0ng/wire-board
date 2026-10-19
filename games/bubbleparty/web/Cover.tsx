export function Cover() {
  return (
    <svg
      className="bubble-cover"
      viewBox="0 0 520 320"
      role="img"
      aria-label="红蓝两队角色在小区草坪上投掷水泡"
    >
      <defs>
        <linearGradient id="bubble-sky" x1="0" y1="0" x2="0" y2="1">
          <stop offset="0" stopColor="#dbe7e2" />
          <stop offset="1" stopColor="#f3e9d7" />
        </linearGradient>
        <linearGradient id="bubble-lawn" x1="0" y1="0" x2="0" y2="1">
          <stop offset="0" stopColor="#a9bea0" />
          <stop offset="1" stopColor="#7e9b7d" />
        </linearGradient>
      </defs>
      <rect width="520" height="320" rx="22" fill="url(#bubble-sky)" />
      <path d="M0 190 Q110 156 205 181 T520 170 V320 H0Z" fill="url(#bubble-lawn)" />
      <g opacity=".55" fill="#f7f3e8">
        <rect x="38" y="86" width="112" height="78" rx="8" />
        <path d="M28 88 94 44l66 44Z" fill="#a57662" />
        <rect x="58" y="112" width="24" height="30" rx="3" fill="#8ba7a0" />
        <rect x="102" y="112" width="26" height="52" rx="3" fill="#8ba7a0" />
        <rect x="380" y="78" width="104" height="86" rx="8" />
        <path d="M370 80 432 40l62 40Z" fill="#9c7c68" />
        <rect x="396" y="106" width="22" height="34" rx="3" fill="#8ba7a0" />
        <rect x="438" y="106" width="24" height="58" rx="3" fill="#8ba7a0" />
      </g>
      <g stroke="#6f8c74" strokeWidth="3" opacity=".7">
        <path d="M20 236h480M14 282h492" />
        <path d="M120 198v110M260 188v122M400 198v110" />
      </g>
      <g transform="translate(126 210)">
        <ellipse cx="0" cy="54" rx="36" ry="10" fill="#5c785e" opacity=".35" />
        <circle cx="0" cy="8" r="26" fill="#bd6c5b" stroke="#5f3a34" strokeWidth="4" />
        <circle cx="-9" cy="4" r="3" fill="#3d2c2a" />
        <circle cx="9" cy="4" r="3" fill="#3d2c2a" />
        <path d="M-8 16q8 7 16 0" fill="none" stroke="#3d2c2a" strokeWidth="3" strokeLinecap="round" />
        <path d="M-18 32h36v36h-36z" fill="#f1e7d6" stroke="#5f3a34" strokeWidth="4" />
        <path d="M-34 40h20M14 40h20" stroke="#5f3a34" strokeWidth="7" strokeLinecap="round" />
      </g>
      <g transform="translate(394 214)">
        <ellipse cx="0" cy="50" rx="36" ry="10" fill="#5c785e" opacity=".35" />
        <circle cx="0" cy="6" r="26" fill="#6f95b4" stroke="#334b60" strokeWidth="4" />
        <circle cx="-9" cy="2" r="3" fill="#273746" />
        <circle cx="9" cy="2" r="3" fill="#273746" />
        <path d="M-8 14q8 7 16 0" fill="none" stroke="#273746" strokeWidth="3" strokeLinecap="round" />
        <path d="M-18 30h36v36h-36z" fill="#eee8da" stroke="#334b60" strokeWidth="4" />
        <path d="M-34 38h20M14 38h20" stroke="#334b60" strokeWidth="7" strokeLinecap="round" />
      </g>
      <g fill="#7faec4" stroke="#547d91" strokeWidth="3">
        <circle cx="248" cy="122" r="22" />
        <circle cx="284" cy="92" r="14" />
        <circle cx="222" cy="82" r="11" />
      </g>
      <path d="M166 180 Q226 70 258 120" fill="none" stroke="#c7b478" strokeWidth="3" strokeDasharray="7 9" />
      <path d="M354 180 Q300 60 280 92" fill="none" stroke="#c7b478" strokeWidth="3" strokeDasharray="7 9" />
      <g fill="#f7f3e8" opacity=".9">
        <circle cx="212" cy="58" r="4" />
        <circle cx="310" cy="48" r="3" />
        <circle cx="278" cy="150" r="3" />
      </g>
    </svg>
  );
}
