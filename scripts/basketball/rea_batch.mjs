// Run one seeded REA/Ghidra session, retaining batch results, dossiers and a snapshot.
// Invoke from the repository root after sourcing scripts/env.sh; the JSON spec
// explicitly selects target, seeds, output, addresses and optional dossiers.
import { Client } from '../../.local/nes-basket/tools/rea-runtime/node_modules/@modelcontextprotocol/client/dist/index.mjs';
import { StdioClientTransport } from '../../.local/nes-basket/tools/rea-runtime/node_modules/@modelcontextprotocol/client/dist/stdio.mjs';
import { readFileSync,writeFileSync,mkdirSync } from 'node:fs';
import { resolve } from 'node:path';
const spec=JSON.parse(readFileSync(process.argv[2],'utf8'));
const out=resolve(spec.output);mkdirSync(out,{recursive:true});
const client=new Client({name:'basketball-original-analysis',version:'1.0.0'},{capabilities:{}});
const transport=new StdioClientTransport({command:'sh',args:['.local/nes-basket/rea.sh','mcp'],env:{...process.env,REA_GHIDRA_SEED_FILE:resolve(spec.seeds)},stderr:'pipe'});
if(transport.stderr)transport.stderr.on('data',chunk=>process.stderr.write(chunk));
async function call(name,args,file){const result=await client.callTool({name,arguments:args},undefined,{timeout:300000});writeFileSync(`${out}/${file}.json`,JSON.stringify(result,null,2));if(result.isError)throw Error(`${name} failed: ${JSON.stringify(result).slice(0,1000)}`);console.log(file);return result;}
try{
 await client.connect(transport);
 await call('open_binary',{path:resolve(spec.target),format:'raw-binary',architecture:'6502',entry_point:'0x8000',provider_id:'ghidra'},'open');
 await call('batch_decompile',{addresses:spec.addresses},'batch');
 for(const address of spec.dossiers??[])await call('analyze_function',{procedure:address},`function-${address}`);
 await call('export_evidence_bundle',{path:`${out}/evidence.json`,overwrite:true},'export');
 await call('close_binary',{snapshot_path:`${out}/snapshot.json`,overwrite:true},'close');
}finally{await client.close();}
