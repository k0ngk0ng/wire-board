import assert from "node:assert/strict";
import { referenceDOMClass, verifyDOMOracle } from "./dom-oracle.mjs";

const sources = await Promise.all([
  referenceDOMClass("utils/Element.js", "Element", ["constructor", "create"]),
  referenceDOMClass("utils/GameScene.js", "GameScene", ["constructor"]),
  referenceDOMClass("utils/Utility.js", "Utility", ["numberDot", "colorTier", "darkenColor"]),
  referenceDOMClass("game/scenes/ShopScene.js", "ShopScene", ["render", "update", "updateEgg", "updateItems"]),
  referenceDOMClass("game/scenes/ShopScene.js", "DisplayPokemon", ["render", "update"])
]);
const input = { gold: 10000000, pokemon: { id: 78, species: "hoothoot" }, keys: ["protein", "leftovers", "charizarditeX", "assaultVest", "silphScope", "quickClaw", "revysBook", "noxDex", "lifeOrb", "choiceScarf"] };
const reference = `
 const {catalog,dictionary,parameters,input}=window.oracleData,text=dictionary.text,playSound=()=>{};
 ${sources.map(source=>source.code).join("\n")}
 const {variant,theme,language}=parameters;
 document.documentElement.dataset.theme=theme;
 if(variant==='still')document.documentElement.classList.add('disable-shop-background-animation');
 const stock=input.keys.map((key,index)=>{if(variant==='sold'&&(index===0||index===4))return null;const item=catalog.items[key];return{...item,sprite:'./'+item.sprite}});
 const eggPrice=variant==='sold'?50000:5432;
 const count=variant==='sold'?0:87;
 const gold=variant==='low'?0:input.gold;
 const main={lang:Number(language),utility:new Utility(),tooltip:{bindTo(){},unbindTo(){}},player:{gold},shop:{itemStock:stock,eggPrice,eggList:Array(count),buyEgg(){if(count&&gold>=eggPrice)window.shopActions.push({type:'egg',quote:count})},buyItem(index){if(stock[index]&&gold>=stock[index].price)window.shopActions.push({type:'buyItem',itemID:stock[index].id,stock:index})}}};
 window.shopActions=[];
 const viewport=document.createElement('div'),root=document.createElement('div'),screen=document.createElement('div');
 viewport.id='view-port';root.id='screen-root';screen.id='screen';root.style.transform='scale(1)';
 screen.append(document.querySelector('.section-scene-container'));root.append(screen);viewport.append(root);document.body.append(viewport);
 const shop=Object.assign(Object.create(ShopScene.prototype),{main,container:document.querySelector('.section-scene-container')});
 shop.render();shop.update();
 if(variant==='hatch'){
  const reel=Object.assign(Object.create(DisplayPokemon.prototype),new GameScene(200,200));
  const species=catalog.species[input.pokemon.species];
  reel.main=main;reel.pokemon={name:species.names,specie:species,sprite:{base:'./'+species.sprite.base}};
  reel.close=()=>{window.shopActions.push({type:'close'});reel.background.remove()};
  reel.header.removeChild(reel.closeButton);reel.render();reel.update();screen.append(reel.background);
 }
`;
const selectors = {
  reference: { tile: ".shop-scene-item", price: ".shop-scene-purchase", dialog: ".game-scene-window", title: ".game-scene-title", prompt: ".dp-scene-prompt", name: ".dp-scene-pokemon-name", okay: ".shop-scene-purchase-egg" },
  native: { tile: ".pokepath-shop-tile", price: ".pokepath-shop-price", dialog: ".pokepath-hatch", title: ".pokepath-hatch-title", prompt: ".pokepath-hatch-prompt", name: ".pokepath-hatch-name", okay: "[data-hatch-close]" }
};
const probe = table => `(()=>{const s=${JSON.stringify(table)},find=key=>document.querySelector(s[key]),nodes=key=>[...document.querySelectorAll(s[key])],read=node=>node?.innerText?.trim()??'',rect=node=>{const r=node.getBoundingClientRect();return[r.x,r.y,r.width,r.height]};return{tiles:nodes('tile').map(node=>[read(node.querySelector(s.price)),getComputedStyle(node).filter,rect(node)]),dialog:find('dialog')?[read(find('title')),read(find('prompt')),read(find('name')),read(find('okay')),rect(find('dialog'))]:null,actions:window.shopActions}})()`;
const layout = table => `(()=>{const fields=['fontFamily','fontSize','lineHeight','fontWeight','fontKerning','fontFeatureSettings','textRendering','letterSpacing','textShadow','boxSizing','display','backgroundColor','transform','backdropFilter'];return Object.fromEntries(Object.entries(${JSON.stringify(table)}).flatMap(([key,selector])=>{const node=document.querySelector(selector);if(!node)return[];const props=getComputedStyle(node),range=document.createRange();range.selectNodeContents(node);return[[key,{bounds:node.getBoundingClientRect().toJSON(),textBounds:range.getBoundingClientRect().toJSON(),...Object.fromEntries(fields.map(field=>[field,props[field]]))}]]}))})()`;
const click = (key, index = 0) => async ({ kind, browser, evaluate }) => {
  const selector = selectors[kind][key];
  const position = await evaluate(`(()=>{const node=document.querySelectorAll(${JSON.stringify(selector)})[${index}],r=node.getBoundingClientRect();return[Math.round(r.left+r.width/2),Math.round(r.top+r.height/2)]})()`);
  await browser("mouse", "move", ...position.map(String));
  await browser("mouse", "down", "left"); await browser("mouse", "up", "left");
};
const inspect = async ({ browser, evaluate, ready }) => {
  await browser("focus", ".pokepath-shop-egg");
  await browser("press", "Tab");
  await ready();
  assert.equal(await evaluate("document.activeElement?.dataset.shopStock"), "0");
  const tip = await evaluate("document.querySelector('[data-shop-tooltip]')?.innerText");
  assert.match(tip, /PROTEIN/i);
  assert.ok(tip.length > "PROTEIN".length);
  assert.deepEqual(await evaluate("window.shopActions"), [{ type: "egg", quote: 87 }, { type: "buyItem", itemID: "protein", stock: 0 }]);
  return { keyboardTooltip: true, text: tip };
};
await verifyDOMOracle({
  name: "shop-render", command: "node games/pokepath/tests/shop-render-oracle.mjs",
  sources: sources.map(source => source.identity),
  styles: [".local/pokepath/rejected-embedding/assets/src/css/shop.css"],
  input, reference, native: 'import "./games/pokepath/tests/ShopFixture.tsx";',
  platformStyles: true,
  cases: [
    { language: "8", theme: "original", font: "PressStart2P", variant: "hatch" },
    { language: "2", theme: "classic", font: "Lexend", variant: "low" },
    { language: "9", theme: "halloween", font: "Rubik", variant: "sold" },
    { language: "0", theme: "monochrome", font: "PressStart2P", variant: "still" },
    { language: "0", theme: "original", font: "PressStart2P", variant: "rich" }
  ],
  steps: [
    { name: "open" },
    { name: "fixed-motion", animationMS: 9340, when: p => p.variant === "rich" || p.variant === "low" || p.variant === "sold" },
    { name: "closed-hatch", action: click("okay"), when: p => p.variant === "hatch" },
    { name: "egg-quote", action: click("tile"), when: p => p.variant !== "hatch" },
    { name: "item-quote", action: click("tile", 1), when: p => p.variant !== "hatch" }
  ],
  measure: { reference: probe(selectors.reference), native: probe(selectors.native) },
  layout: { reference: layout(selectors.reference), native: layout(selectors.native) },
  verifyNative: inspect
});
