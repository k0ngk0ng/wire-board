// Private stdio REA session: never changes the shared MCP target.
import { Client } from '../../.local/nes-basket/tools/rea-runtime/node_modules/@modelcontextprotocol/client/dist/index.mjs';
import { StdioClientTransport } from '../../.local/nes-basket/tools/rea-runtime/node_modules/@modelcontextprotocol/client/dist/stdio.mjs';
import { readFileSync, writeFileSync, mkdirSync } from 'node:fs';
import { createHash } from 'node:crypto';
import { resolve, sep } from 'node:path';
const root=process.cwd();
const spec=JSON.parse(readFileSync(process.argv[2],'utf8'));
const local=p=>{const q=resolve(p);if(!q.startsWith(root+sep))throw Error('Research paths must remain in workspace');return q;};
const out=local(spec.output), tmp=local('dist/campaign-scene-tmp');
mkdirSync(out,{recursive:true});mkdirSync(tmp,{recursive:true});
const target=local(spec.target);
if(createHash('sha256').update(readFileSync(target)).digest('hex')!==spec.file_sha256)throw Error('Target digest differs');
const client=new Client({name:'contra-campaign-scene-recovery',version:'1.0.0'},{capabilities:{}});
const transport=new StdioClientTransport({command:'rea',args:['--mcp'],env:{...process.env,
 TMPDIR:tmp,REA_DARWIN_MODULE_CACHE_PATH:local('.local/contra-campaign-scene/swift-cache'),
 GHIDRA_INSTALL_DIR:process.env.GHIDRA_INSTALL_DIR??local('.local/nes-basket/tools/ghidra_12.1.4_PUBLIC'),
 JAVA_HOME:process.env.JAVA_HOME??'/opt/homebrew/opt/openjdk@26/libexec/openjdk.jdk/Contents/Home',
 REA_GHIDRA_SEED_FILE:local(spec.seeds),REA_LOG_LEVEL:'silent'},stderr:'pipe'});
if(transport.stderr)transport.stderr.on('data',c=>process.stderr.write(c));
const read=r=>r.structuredContent??JSON.parse(r.content.find(x=>x.type==='text').text);
async function call(name,args,file){
 const result=await client.callTool({name,arguments:args},undefined,{timeout:300000});
 if(result.isError)throw Error(JSON.stringify(result));
 const data=read(result);writeFileSync(`${out}/${file}.json`,JSON.stringify(data,null,2)+'\n');
 console.log(JSON.stringify({operation:name,file,evidence_id:data.evidence_id}));return data;
}
try{
 await client.connect(transport);
 await call('open_binary',{path:target,format:'raw-binary',architecture:'6502',entry_point:spec.entry,provider_id:'ghidra'},'open');
 for(const address of spec.addresses)await call('analyze_function',{procedure:address},`function-${address}`);
 await call('export_evidence_bundle',{path:`${out}/evidence.json`,overwrite:true},'export');
 await call('close_binary',{},'close');
}finally{await client.close();}
