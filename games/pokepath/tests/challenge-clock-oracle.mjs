import { referenceDOMClass, verifyDOMOracle } from "./dom-oracle.mjs";

const utility = await referenceDOMClass("utils/Utility.js", "Utility", ["chrono"]);
const reference = `
 const {dictionary,parameters}=window.oracleData;
 const parent=document.querySelector('.section-scene-container');
 const footer=document.createElement('div');
 footer.className='ui-bottom-bar';
 Object.assign(footer.style,{left:'0',right:'0'});
 parent.appendChild(footer);
 if(parameters.active==='yes'){
  const label=document.createElement('span');
  label.className='ui-challenge stroke';
  label.textContent=dictionary.text.challenge.label[Number(parameters.language)].toUpperCase();
  const output=document.createElement('span');
  output.id='ui-chrono';
  footer.append(label,output);
  let dispatch;
  const setInterval=next=>{dispatch=next;return 1};
  const clearInterval=()=>{dispatch=undefined};
  ${utility.code}
  const chrono=new Utility().chrono();
  chrono.restart();chrono.start();
  for(let tick=0;tick<Math.floor(Number(parameters.elapsed)/10);tick++)dispatch();
  chrono.stop();
 }
`;
const measure = (status, clock) => `(()=>{const read=selector=>{const target=document.querySelector(selector);if(!target)return null;const r=target.getBoundingClientRect();return{text:target.textContent,geometry:[r.left,r.top,r.width,r.height]}};return [read(${JSON.stringify(status)}),read(${JSON.stringify(clock)})]})()`;
await verifyDOMOracle({
  name: "challenge-clock", command: "node games/pokepath/tests/challenge-clock-oracle.mjs",
  sources: [utility.identity], styles: [".local/pokepath/rejected-embedding/assets/src/css/ui.css"], reference,
  native: 'import "./games/pokepath/tests/ChallengeClockFixture.tsx";',
  input: {},
  cases: [
    { language: "8", theme: "original", font: "PressStart2P", elapsed: "0", active: "yes" },
    { language: "8", theme: "original", font: "PressStart2P", elapsed: "9", active: "yes" },
    { language: "8", theme: "original", font: "PressStart2P", elapsed: "10", active: "yes" },
    { language: "0", theme: "classic", font: "Rubik", elapsed: "59999", active: "yes" },
    { language: "2", theme: "halloween", font: "Lexend", elapsed: "60010", active: "yes" },
    { language: "8", theme: "original", font: "PressStart2P", elapsed: "10000", active: "no" }
  ],
  steps: [{ name: "bottom-bar" }],
  platformStyles: true,
  measure: { reference: measure(".ui-challenge", "#ui-chrono"), native: measure("[data-challenge-status]", "[data-challenge-clock]") }
});
