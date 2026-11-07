export function Cover() {
  return (
    <svg viewBox="0 0 640 360" aria-hidden="true" className="pvz-cover">
      <defs>
        <linearGradient id="pvz-cover-sky" x2="0" y2="1"><stop stopColor="#bed8d4"/><stop offset="1" stopColor="#f3e8ba"/></linearGradient>
        <linearGradient id="pvz-cover-grass" x2="0" y2="1"><stop stopColor="#839d5e"/><stop offset="1" stopColor="#465d42"/></linearGradient>
      </defs>
      <path fill="url(#pvz-cover-sky)" d="M0 0h640v360H0z"/>
      <circle cx="111" cy="71" r="34" fill="#eedc91"/>
      <path d="M0 214q160-54 320-4t320-12v162H0z" fill="url(#pvz-cover-grass)"/>
      <path d="M0 222q157-47 322-3t318-12M0 280q153-47 320-6t320-14M108 207l-30 153m151-150-12 150m149-140 14 140m144-148 38 148" fill="none" stroke="#bbbd7a" strokeWidth="3" opacity=".35"/>
      <path d="M22 201V115l65-50 67 50v87" fill="#dac8aa" stroke="#6f715e" strokeWidth="5" strokeLinejoin="round"/>
      <path d="m16 121 70-58 74 58" fill="none" stroke="#b47660" strokeWidth="15" strokeLinecap="round"/>
      <path d="M69 149h33v53H69z" fill="#758e8c"/><path d="M33 128h22v25H33zm83 0h22v25h-22z" fill="#a9c1ba"/>
      <g transform="translate(236 197) rotate(-8)">
        <path d="M0 78Q-19 38 2 4" fill="none" stroke="#466b46" strokeWidth="17" strokeLinecap="round"/>
        <path d="M-5 53q-62-40-61-3 30 31 61 3m9-18q56-40 60-8-19 29-60 8" fill="#71965a" stroke="#456b46" strokeWidth="4"/>
        <path d="M-42-3q-9-55 35-65 38-9 54 21l46 2 3 41-49 9q-18 29-50 21-32-6-39-29z" fill="#8aa866" stroke="#456b46" strokeWidth="6"/>
        <ellipse cx="92" cy="-25" rx="13" ry="20" fill="#506f48" stroke="#456b46" strokeWidth="4"/>
        <ellipse cx="9" cy="-39" rx="6" ry="10" fill="#293b2f"/><circle cx="11" cy="-42" r="2" fill="#f3f1ce"/>
      </g>
      <g transform="translate(479 181) rotate(9)">
        <path d="m-24 74-13 68m49-65 23 64" stroke="#4c5352" strokeWidth="22" strokeLinecap="round"/>
        <path d="m-39 143-20 2m92-4 23 6" stroke="#54504b" strokeWidth="16" strokeLinecap="round"/>
        <path d="m-44 17 29-10 40 7 19 69-87 2z" fill="#8e7567" stroke="#535b52" strokeWidth="5"/>
        <path d="m-29 25-37 27-35-2m119-27 26 33 35 4" fill="none" stroke="#8a9c81" strokeWidth="13" strokeLinecap="round"/>
        <path d="m-15 10 12 54 15-48" fill="#e0d5b9"/><path d="m-3 20 8 28 8-25" fill="#af7565"/>
        <path d="M-36-15q-3-48 30-52 38-5 48 27l-4 37-18 21-30-4-22-18z" fill="#99a38a" stroke="#536253" strokeWidth="6"/>
        <path d="m-36-39 7-25 32-12 29 20" fill="#777d65" stroke="#536253" strokeWidth="5"/>
        <ellipse cx="-18" cy="-28" rx="10" ry="12" fill="#f4eace"/><ellipse cx="14" cy="-24" rx="11" ry="12" fill="#f4eace"/>
        <circle cx="-15" cy="-27" r="4" fill="#424a3c"/><circle cx="11" cy="-23" r="4" fill="#424a3c"/>
        <path d="m-17 0 36 5-6 10-25-5z" fill="#53594b"/><path d="m-4 3 8 1-1 9-7-1z" fill="#f3e5bf"/>
      </g>
      <g fill="#d0df91"><circle cx="350" cy="175" r="9"/><circle cx="390" cy="179" r="7"/></g>
      <path d="m170 335 4-20 7 21m205 9 7-23 4 22m205-37 5-24 8 22" stroke="#a4b673" strokeWidth="4" fill="none"/>
    </svg>
  );
}
