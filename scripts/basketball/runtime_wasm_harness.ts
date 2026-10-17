import { loadBasketballWasm } from '../../games/basketball/web/wasm';

const engine = await loadBasketballWasm('/runtime');
Object.assign(window,{runtimeEngine:engine});
document.querySelector('#status')!.textContent='Go/Wasm 已连接';
