import { useContext, useId } from "react";
import { AssetsContext } from "../../../web/src/ui/board-host";
import cover from "../assets/cover-manifest.json";

export function Cover() {
  const id = useId();
  const assets = useContext(AssetsContext).replace(/\/+$/, "");
  const officialCDN = /^https:\/\/cdn\.ichenj\.com(?:\/|$)/.test(assets);
  const heroes = !assets || officialCDN ? cover.url : assets + cover.localPath;
  return (
    <svg
      viewBox="0 0 640 380"
      role="img"
      aria-label="热血篮球：国夫与阿力背靠背站在街头球场，身后是三层篮筐"
    >
      <defs>
        <linearGradient id={`${id}-sky`} x2="0" y2="1">
          <stop stopColor="#f0e3c3" />
          <stop offset="1" stopColor="#d9d5b6" />
        </linearGradient>
        <linearGradient id={`${id}-court`} x2="1" y2="1">
          <stop stopColor="#b78864" />
          <stop offset="1" stopColor="#876c59" />
        </linearGradient>
        <clipPath id={`${id}-frame`}>
          <rect width="640" height="380" rx="18" />
        </clipPath>
      </defs>
      <g clipPath={`url(#${id}-frame)`}>
        <rect width="640" height="380" fill={`url(#${id}-sky)`} />
        <circle cx="113" cy="72" r="38" fill="#d9b879" />
        <path
          d="M0 220V118h37V89h49v111h25V137h45v80h26V74h53v137h33V122h43v91h32V91h48v122h40V66h55v125h30V116h51v83h28V98h45v139H0Z"
          fill="#9aac98"
        />
        <path
          d="M193 92h29m-29 18h29m-29 18h29m222-43h25m-25 18h25M46 109h27m-27 19h27m451 7h29"
          fill="none"
          stroke="#708b80"
          strokeWidth="5"
          opacity=".5"
        />
        <path d="M0 266 640 207v173H0Z" fill={`url(#${id}-court)`} />
        <path
          d="M0 337 640 275M46 380c-4-65 94-112 206-105m119 105c-4-41 55-73 130-75"
          fill="none"
          stroke="#e9d9b3"
          strokeWidth="4"
        />
        <path
          d="M0 233 640 184M0 257l640-50M26 205v66m46-71v67m45-71v66m46-71v67m286-88v66m45-70v66m45-70v66m44-71v67m45-71v67"
          fill="none"
          stroke="#687f70"
          strokeWidth="2"
          opacity=".62"
        />
        <path
          d="m25 215 30 34 32-39 30 34 31-39 31 34m293-57 30 34 31-39 32 34 30-39 31 34"
          fill="none"
          stroke="#819583"
          strokeWidth="1.5"
          opacity=".6"
        />
        <g stroke="#485a4f" strokeLinecap="round" strokeLinejoin="round">
          <path d="M590 337V43" fill="none" strokeWidth="9" />
          <path d="m491 48 123-9v77l-123 8Z" fill="#f1e8ce" strokeWidth="3" />
          <path
            d="m526 68 43-3v37l-43 3Z"
            fill="none"
            stroke="#ad654c"
            strokeWidth="3"
          />
          <path
            d="m503 129 14 38 35-2 13-37m-53 9 38 26m8-27-37 28"
            fill="none"
            stroke="#f7edce"
            strokeWidth="2.5"
          />
          <ellipse
            cx="534"
            cy="127"
            rx="36"
            ry="7"
            fill="#be8963"
            stroke="#85503e"
            strokeWidth="4"
          />
          <path d="m508 177 105-7v60l-105 7Z" fill="#d3d8bf" strokeWidth="3" />
          <path
            d="m518 242 12 31h28l13-32m-45 7 30 23m8-24-31 25"
            fill="none"
            stroke="#ede3c3"
            strokeWidth="2.3"
          />
          <ellipse
            cx="545"
            cy="240"
            rx="31"
            ry="6"
            fill="#b48260"
            stroke="#85503e"
            strokeWidth="3.5"
          />
          <path d="m526 288 86-5v43l-86 5Z" fill="#c7d0b7" strokeWidth="3" />
          <path
            d="m536 336 10 26h24l11-27m-38 7 26 18m6-19-26 20"
            fill="none"
            stroke="#e6debd"
            strokeWidth="2"
          />
          <ellipse
            cx="559"
            cy="334"
            rx="27"
            ry="5.5"
            fill="#ac7c59"
            stroke="#85503e"
            strokeWidth="3"
          />
        </g>
        <path
          d="M89 319c-30-31-25-73 11-110m-31 106c-20-34-12-73 20-105"
          fill="none"
          stroke="#d7b575"
          strokeWidth="4"
          strokeLinecap="round"
          opacity=".7"
        />
        <ellipse
          cx="307"
          cy="363"
          rx="155"
          ry="16"
          fill="#4b5246"
          opacity=".25"
        />
        <image
          href={heroes}
          x="148"
          y="17"
          width="320"
          height="356"
        />
      </g>
    </svg>
  );
}
