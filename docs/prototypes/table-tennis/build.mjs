import { build } from '../../../web/node_modules/esbuild/lib/main.js';
import { readFileSync, writeFileSync } from 'node:fs';
const root = new URL('./', import.meta.url);
const result = await build({ entryPoints: [new URL('scene.js',root).pathname], bundle:true, minify:true, format:'iife', write:false, target:'es2022' });
writeFileSync(new URL('index.html',root),readFileSync(new URL('shell.html',root),'utf8').replace('/* PROTOTYPE_SCRIPT */',()=>result.outputFiles[0].text.replaceAll('</script','<\\/script')));
console.log('Built standalone table-tennis art prototype');
