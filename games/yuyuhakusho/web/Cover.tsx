import { useId } from "react";

export function Cover() {
  const id = useId();
  return (
    <svg viewBox="0 0 640 380" role="img" aria-label="浦饭幽助身穿绿色制服，举起指尖凝聚灵丸，身后是暗黑武术会的屋檐">
      <defs>
        <linearGradient id={`${id}-paper`} x2="1" y2="1">
          <stop stopColor="#e6e0c9" />
          <stop offset="1" stopColor="#b3c2ad" />
        </linearGradient>
        <linearGradient id={`${id}-jacket`} x2="1" y2="1">
          <stop stopColor="#577e63" />
          <stop offset="1" stopColor="#2b554a" />
        </linearGradient>
        <radialGradient id={`${id}-light`}>
          <stop stopColor="#fff5ce" />
          <stop offset=".25" stopColor="#c9e8d3" />
          <stop offset=".6" stopColor="#8dc1b7" stopOpacity=".7" />
          <stop offset="1" stopColor="#7aa89f" stopOpacity="0" />
        </radialGradient>
        <clipPath id={`${id}-frame`}><rect width="640" height="380" rx="18" /></clipPath>
      </defs>
      <g clipPath={`url(#${id}-frame)`}>
        <rect width="640" height="380" fill={`url(#${id}-paper)`} />
        <circle cx="499" cy="126" r="117" fill="#dce1c6" />
        <path d="M392 0H640V380H316l73-109-31-88 67-76Z" fill="#55756c" opacity=".24" />
        <path d="M0 271q115-41 218-14t222-28q112-27 200 2v149H0Z" fill="#879d87" />
        <path d="M0 324q145-35 281-6t359-14v76H0Z" fill="#5d7b70" />
        <g stroke="#708779" strokeWidth="3" strokeLinejoin="round">
          <path d="M25 187v-68h124v68" fill="#b3bea3" />
          <path d="m5 121 82-44 91 44-12 10H19Z" fill="#6d8576" />
          <path d="m18 116 69-37 76 37M55 131v54m63-54v54" fill="none" />
          <path d="M25 148h124m-124 17h124" fill="none" opacity=".55" />
        </g>
        <path d="m0 225 148-13m-131 9v54m53-58v48m52-52v45" fill="none" stroke="#6a8477" strokeWidth="5" />
        <path d="m377 312 263-13m-238 25 238-8m-260 31 207-5" fill="none" stroke="#b8c4a8" strokeWidth="3" opacity=".5" />
        <g stroke="#304c43" strokeWidth="4" strokeLinejoin="round" strokeLinecap="round">
          <path d="m165 234-48 146h243l-22-104-42-55-64-16Z" fill={`url(#${id}-jacket)`} />
          <path d="m227 218 30 5 25 74-33 83h-48l22-91Z" fill="#e4ddbf" />
          <path d="m207 206-37 29 39 57 27-62m35-18 35 11-13 68-26-58" fill="#77947a" />
          <path d="m172 250-29 86 55 44h55l-20-44-47-7 27-50Z" fill="#426d57" />
          <path d="m151 337 55 31 33-23-46-20Z" fill="#d2ad89" />
          <path d="m251 345 42-31 47-61-23-18-57 61-39 27Z" fill="#d8b390" />
          <path d="m238 326 18-27 21 14-18 30Z" fill="#718d73" />
          <path d="m278 231 30-18 31 33-25 43-40-17Z" fill="#557e62" />
          <path d="m314 239 76-64 27 28-57 66-37 12-20-24Z" fill="#d5af8b" />
          <path d="m325 248 71-57 12 14-54 51-23 11Z" fill="#b68e72" stroke="none" />
          <path d="m309 238-12 24 28 24 20-18Z" fill="#8ca188" />
          <path d="m389 177 11-30 13-21q7-8 12-1l-4 23 12 5 67-3q12 1 11 10-1 8-12 8l-49 3 13 11q7 7 1 14l-11 12q-8 9-20 6l-27-13Z" fill="#dfbb96" />
          <path d="m420 153 19 6m-5 17 17 7-9 11m-19-4 14 10m-24-4 14 9" fill="none" stroke="#a78066" strokeWidth="2.5" />
          <path d="m398 173 7-17" fill="none" stroke="#f0d2ac" strokeWidth="4" />
          <path d="m221 188-6 33 24 15 29-19-8-33Z" fill="#c99f7d" />
          <path d="m224 197 32-3-5 22-16 7Z" fill="#aa8066" stroke="none" />
          <path d="M194 113q-8 37 4 63l25 26q20 10 37-2l27-25 3-42-21-37-49-7Z" fill="#dfbd96" />
          <path d="m195 127 13 30 3 23 21 23-13-1-25-26-8-33Z" fill="#bd9678" stroke="none" />
          <path d="M199 140q-17-17-22-3-4 12 9 25l14 2" fill="#d4ad87" />
          <path d="m186 139 7 11-4 5" fill="none" stroke="#aa8066" strokeWidth="2.5" />
          <path d="m255 151 6 14-10 3m-18 16 20-4" fill="none" stroke="#936e57" strokeWidth="2.5" />
          <path d="m212 141 18 5-8 7-10-4m37-4 22-6-6 12-15 2" fill="#f2ead4" strokeWidth="2.5" />
          <path d="m220 145 1 5m38-5-1 5" fill="none" stroke="#263c34" strokeWidth="4" />
          <path d="m208 131 24 9m15 0 27-11" fill="none" stroke="#2e493e" strokeWidth="5" />
          <path d="m194 153-13-22-11-19-21-7 19-12-6-19 25 1 4-17 25 11 17-21 17 14 23-10 8 21 23 8-8 17 17 13-17 20-8-16-18-7-9-16-19 17-27 2-9 32Z" fill="#28483d" />
          <path d="m181 100 30-15 24 4 23-16m-52 33 27-10 28 5m-67-25 20 6" fill="none" stroke="#4d6c51" strokeWidth="3" />
          <path d="m273 103 6 24" fill="none" stroke="#547655" strokeWidth="3" />
          <path d="m172 306 10-27m-39 83 24 13m119-29 13-24" fill="none" stroke="#91a185" strokeWidth="2.5" />
          <circle cx="275" cy="262" r="3" fill="#d3ba7a" stroke="none" />
          <circle cx="281" cy="281" r="3" fill="#d3ba7a" stroke="none" />
        </g>
        <circle cx="514" cy="151" r="105" fill={`url(#${id}-light)`} />
        <g fill="none" strokeLinecap="round">
          <path d="M459 118q24-34 60-29t56 42m-14 62q-35 29-73 5" stroke="#b4d3bd" strokeWidth="3" />
          <path d="M474 139q23-26 50-17t25 36q-6 24-32 26" stroke="#d7e5c4" strokeWidth="4" />
          <path d="m542 108 12-20m13 65 32-4m-61 51 12 18m-73-26-15 15m-2-59-20-6" stroke="#d8e6c9" strokeWidth="3" />
          <path d="M523 152q41-12 72-7m-69 14 57 12" stroke="#e2ebcd" strokeWidth="4" />
        </g>
        <circle cx="514" cy="151" r="24" fill="#b5d6be" stroke="#dce9c8" strokeWidth="5" />
        <circle cx="514" cy="151" r="14" fill="#fff0bb" />
        <path d="m510 138 5 8 9 4-9 5-5 9-3-10-8-4 8-4Z" fill="#fff8df" />
        <g fill="#dce6c8">
          <path d="m366 74 5 9-3 11-7-8Z" /><path d="m574 272 8 3-2 13-8-5Z" />
          <path d="m81 305 12-4 5 5-15 6Z" /><circle cx="444" cy="239" r="3" /><circle cx="548" cy="60" r="3" />
        </g>
      </g>
    </svg>
  );
}
