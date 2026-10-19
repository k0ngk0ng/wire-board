const coverSVG = `
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 520 260" role="presentation">
  <defs>
    <linearGradient id="bc-sky" x1="0" y1="0" x2="0" y2="1">
      <stop offset="0" stop-color="#24333a"/>
      <stop offset="1" stop-color="#152026"/>
    </linearGradient>
    <linearGradient id="bc-ground" x1="0" y1="0" x2="0" y2="1">
      <stop offset="0" stop-color="#5f5545"/>
      <stop offset="1" stop-color="#3c352c"/>
    </linearGradient>
    <pattern id="bc-brick" width="18" height="12" patternUnits="userSpaceOnUse">
      <rect width="18" height="12" fill="#8d513b"/>
      <path d="M0 6h18M9 0v6M0 0v6M18 6v6M0 6v6" stroke="#5f3428" stroke-width="1.3"/>
    </pattern>
  </defs>
  <rect width="520" height="260" fill="url(#bc-sky)"/>
  <path d="M0 74 Q90 48 180 72 T360 66 T520 76 V260 H0Z" fill="#293c3b"/>
  <path d="M0 192 Q120 170 240 190 T520 184 V260 H0Z" fill="url(#bc-ground)"/>
  <g opacity=".55" fill="#6f8d72">
    <circle cx="54" cy="56" r="24"/><circle cx="86" cy="64" r="17"/>
    <circle cx="454" cy="52" r="27"/><circle cx="486" cy="62" r="18"/>
    <path d="M24 72h70v18H24zM424 68h76v20h-76z"/>
  </g>
  <!-- upper base -->
  <g transform="translate(238 22)">
    <path d="M10 34 L26 4 L42 34 Z" fill="#d7b98e" stroke="#3a2d24" stroke-width="2"/>
    <rect x="4" y="30" width="44" height="8" rx="2" fill="#8d513b" stroke="#3a2d24" stroke-width="2"/>
    <path d="M0 38h52v10H0z" fill="url(#bc-brick)"/>
    <path d="M6 8L20 26M46 8L32 26" stroke="#d7b98e" stroke-width="2" opacity=".6"/>
  </g>
  <!-- lower base -->
  <g transform="translate(238 204)">
    <path d="M10 34 L26 4 L42 34 Z" fill="#b6d2df" stroke="#26343b" stroke-width="2"/>
    <rect x="4" y="30" width="44" height="8" rx="2" fill="#4f6f7f" stroke="#26343b" stroke-width="2"/>
    <path d="M0 38h52v10H0z" fill="url(#bc-brick)"/>
    <path d="M6 8L20 26M46 8L32 26" stroke="#b6d2df" stroke-width="2" opacity=".6"/>
  </g>
  <!-- upper formation -->
  <g transform="translate(64 96)" stroke="#2a211c" stroke-width="2.4" stroke-linejoin="round">
    <rect x="0" y="10" width="38" height="30" rx="5" fill="#c47a5a"/>
    <rect x="-3" y="13" width="6" height="24" fill="#4e3a2c"/>
    <rect x="35" y="13" width="6" height="24" fill="#4e3a2c"/>
    <rect x="14" y="0" width="10" height="18" fill="#c47a5a"/>
    <circle cx="19" cy="25" r="7" fill="#e8c79d"/>
    <path d="M-1 40h40M6 43v4M32 43v4" fill="none"/>
  </g>
  <g transform="translate(128 112)" stroke="#2a211c" stroke-width="2.2" stroke-linejoin="round">
    <rect x="0" y="8" width="28" height="22" rx="4" fill="#b56e50"/>
    <rect x="-2" y="10" width="4" height="18" fill="#4e3a2c"/>
    <rect x="26" y="10" width="4" height="18" fill="#4e3a2c"/>
    <rect x="10" y="0" width="8" height="13" fill="#b56e50"/>
    <circle cx="14" cy="19" r="5" fill="#e8c79d"/>
  </g>
  <g transform="translate(24 132)" stroke="#2a211c" stroke-width="2" stroke-linejoin="round">
    <rect x="0" y="6" width="22" height="16" rx="3" fill="#a9654c"/>
    <rect x="8" y="0" width="6" height="9" fill="#a9654c"/>
    <circle cx="11" cy="14" r="4" fill="#e8c79d"/>
  </g>
  <!-- lower formation -->
  <g transform="translate(418 112) scale(-1 1)" stroke="#1f2b31" stroke-width="2.4" stroke-linejoin="round">
    <rect x="0" y="10" width="38" height="30" rx="5" fill="#5d8ca8"/>
    <rect x="-3" y="13" width="6" height="24" fill="#2b3d46"/>
    <rect x="35" y="13" width="6" height="24" fill="#2b3d46"/>
    <rect x="14" y="0" width="10" height="18" fill="#5d8ca8"/>
    <circle cx="19" cy="25" r="7" fill="#d8ecf4"/>
    <path d="M-1 40h40M6 43v4M32 43v4" fill="none"/>
  </g>
  <g transform="translate(364 126) scale(-1 1)" stroke="#1f2b31" stroke-width="2.2" stroke-linejoin="round">
    <rect x="0" y="8" width="28" height="22" rx="4" fill="#527e98"/>
    <rect x="-2" y="10" width="4" height="18" fill="#2b3d46"/>
    <rect x="26" y="10" width="4" height="18" fill="#2b3d46"/>
    <rect x="10" y="0" width="8" height="13" fill="#527e98"/>
    <circle cx="14" cy="19" r="5" fill="#d8ecf4"/>
  </g>
  <g transform="translate(462 138) scale(-1 1)" stroke="#1f2b31" stroke-width="2" stroke-linejoin="round">
    <rect x="0" y="6" width="22" height="16" rx="3" fill="#4b7288"/>
    <rect x="8" y="0" width="6" height="9" fill="#4b7288"/>
    <circle cx="11" cy="14" r="4" fill="#d8ecf4"/>
  </g>
  <!-- central clash -->
  <g transform="translate(252 126)">
    <path d="M-44 0H-12M12 0H44M0 -44V-12M0 12V44" stroke="#e9c77e" stroke-width="3" stroke-linecap="round"/>
    <path d="M-20 -20L-7 -7M20 20L7 7M20 -20L7 -7M-20 20L-7 7" stroke="#e9c77e" stroke-width="2" opacity=".8"/>
    <circle r="9" fill="#f3d99a" stroke="#8d513b" stroke-width="2"/>
    <circle r="3" fill="#8d513b"/>
    <path d="M-15 5l-8 8M15 -5l8 -8M15 5l8 8M-15 -5l-8 -8" stroke="#e9c77e" stroke-width="2" opacity=".7"/>
  </g>
  <!-- foreground brick fragments -->
  <path d="M0 238h72v22H0zM92 244h58v16H92zM430 240h90v20h-90z" fill="url(#bc-brick)" opacity=".75"/>
  <path d="M40 230l8-10 6 12 9-8 5 10" fill="none" stroke="#9a6a4f" stroke-width="2" opacity=".7"/>
  <path d="M448 230l-8-10-6 12-9-8-5 10" fill="none" stroke="#6d8fa0" stroke-width="2" opacity=".7"/>
</svg>
`;

export function Cover() {
  return <div dangerouslySetInnerHTML={{ __html: coverSVG }} />;
}
