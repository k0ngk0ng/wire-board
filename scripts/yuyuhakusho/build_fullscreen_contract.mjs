import {build} from '../../web/node_modules/esbuild/lib/main.js';
import {readFile,mkdir} from 'node:fs/promises';
import {fileURLToPath} from 'node:url';
import path from 'node:path';
const root=fileURLToPath(new URL('../../',import.meta.url));
const out=path.join(root,'.local/md-yuyuhakusho/fullscreen-contract');
await mkdir(out,{recursive:true});
const source=path.join(root,'web/src/ui/room-fullscreen.tsx');
await build({entryPoints:[path.join(root,'scripts/yuyuhakusho/fullscreen_contract.tsx')],outfile:path.join(out,'fixture.js'),bundle:true,format:'iife',platform:'browser',jsx:'automatic',loader:{'.css':'empty'},alias:{react:path.join(root,'web/node_modules/react'),'react-dom':path.join(root,'web/node_modules/react-dom'),'lucide-react':path.join(root,'web/node_modules/lucide-react')},plugins:[{name:'frozen-fullscreen',setup(b){b.onResolve({filter:/^fullscreen-before$/},()=>({path:source,namespace:'frozen'}));b.onLoad({filter:/.*/,namespace:'frozen'},async()=>({contents:await readFile(path.join(root,'games/yuyuhakusho/verification/platform-web/room-fullscreen.before-toolbar-slot.tsx.txt'),'utf8'),loader:'tsx',resolveDir:path.dirname(source)}));}}]});
