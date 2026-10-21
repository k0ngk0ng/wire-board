export function Cover() {
  return (
    <svg viewBox="0 0 600 360" role="img" aria-label="街灯下的跑车驶向城市赛道">
      <defs>
        <linearGradient id="race-sky" x2="0" y2="1">
          <stop stopColor="#343f55" />
          <stop offset="1" stopColor="#9b9898" />
        </linearGradient>
        <linearGradient id="race-road" x2="0" y2="1">
          <stop stopColor="#727984" />
          <stop offset="1" stopColor="#3b4853" />
        </linearGradient>
      </defs>
      <rect width="600" height="360" fill="url(#race-sky)" />
      <circle cx="460" cy="70" r="30" fill="#d7cdb6" />
      <g fill="#4b5865" stroke="#384652" strokeWidth="3">
        <path d="M0 162V56h66v132H0M72 193V88h73v109M467 185V50h51v130M525 195V105h75v91" />
        <path d="M166 181v-67h54v67M397 184v-87h49v87" />
      </g>
      <g fill="#c9bc99">
        <path d="M15 78h10v14H15zM37 78h10v14H37zM15 111h10v14H15zM37 111h10v14H37zM88 110h12v18H88zM112 110h12v18h-12zM478 72h10v18h-10zM498 107h10v18h-10zM546 127h12v17h-12zM575 127h12v17h-12z" />
      </g>
      <path d="M245 159h102L600 360H0z" fill="url(#race-road)" />
      <path
        d="M250 166L63 360M343 166l200 194"
        stroke="#c8bda6"
        strokeWidth="6"
      />
      <path d="M298 177v18m0 19v32m0 28v65" stroke="#e5d4ac" strokeWidth="7" />
      <g stroke="#4c5865" strokeWidth="5" fill="none">
        <path d="M139 231V105h45M434 231V105h-45" />
      </g>
      <g fill="#e4d5a8">
        <ellipse cx="180" cy="108" rx="16" ry="5" />
        <ellipse cx="393" cy="108" rx="16" ry="5" />
      </g>
      <ellipse cx="306" cy="310" rx="118" ry="22" fill="#293a46" opacity=".5" />
      <path
        d="M194 271l32-51 29-13h90l35 18 34 47-10 39H201z"
        fill="#a76d60"
        stroke="#493f44"
        strokeWidth="4"
      />
      <path
        d="M233 223l27-9h80l29 14 15 32H218z"
        fill="#425b69"
        stroke="#443f46"
        strokeWidth="4"
      />
      <path
        d="M195 270l55-7h102l63 10-9 39H202z"
        fill="#ba8170"
        stroke="#4e4548"
        strokeWidth="4"
      />
      <path
        d="M212 281h53v12h-53zM346 281h52v12h-52z"
        fill="#dca196"
        stroke="#854e50"
        strokeWidth="3"
      />
      <path d="M270 285h70v17h-70z" fill="#465362" />
      <path d="M205 310v15h28v-12M374 313v12h28v-15" fill="#293641" />
      <path
        d="M208 251h190v8H208z"
        fill="#535361"
        stroke="#44424a"
        strokeWidth="3"
      />
      <path d="M211 304h50m87 0h47" stroke="#dec3a7" strokeWidth="4" />
    </svg>
  );
}
