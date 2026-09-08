import {spawn,execFileSync} from 'node:child_process';
import {mkdtempSync,readFileSync,existsSync,mkdirSync,statSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {resolve,join} from 'node:path';
import http from 'node:http';
const binary=resolve(process.argv[2]);const dir=process.env.OSTRM_SMOKE_DATA||mkdtempSync(join(tmpdir(),'ostrm-smoke-'));mkdirSync(dir,{recursive:true});let child;
const mock=http.createServer((req,res)=>{let body='';req.on('data',c=>body+=c);req.on('end',()=>{const q=JSON.parse(body||'{}');res.setHeader('Content-Type','application/json');res.end(JSON.stringify({code:200,data:{content:q.path==='/media'?[{name:'movie.mkv',is_dir:false,size:10,modified:'2026-01-01',sign:'test=:0'}]:[],total:q.path==='/media'?1:0}}));});});
await new Promise(r=>mock.listen(0,'127.0.0.1',r));
let base='';const start=async()=>{child=spawn(binary,['serve','--listen','127.0.0.1:0','--data-dir',dir],{cwd:tmpdir(),stdio:['ignore','pipe','inherit']});base=await new Promise((resolve,reject)=>{const timer=setTimeout(()=>reject(Error('Startup timeout')),15000);child.once('exit',c=>{clearTimeout(timer);reject(Error(`Exited ${c}`))});child.stdout.on('data',b=>{const m=String(b).match(/http:\/\/127\.0\.0\.1:\d+/);if(m){clearTimeout(timer);resolve(m[0])}})});};
const stop=async()=>{if(child&&child.exitCode===null){const done=new Promise(r=>child.once('exit',r));child.kill('SIGTERM');await done;}};
try{await start();const html=await(await fetch(base)).text();if(!html.includes('_nuxt'))throw Error('Embedded UI missing');const js=html.match(/(?:src|href)="([^\"]*\/_nuxt\/[^\"]+\.js)"/);if(!js)throw Error('No JS asset');if(!(await fetch(base+js[1])).ok)throw Error('JS missing');
let token='';const api=async(method,p,data)=>{const r=await fetch(base+'/api'+p,{method,headers:{'Content-Type':'application/json',...(token?{Authorization:'Bearer '+token}:{})},body:data?JSON.stringify(data):undefined});const m=await r.json();if(m.code!==200)throw Error(JSON.stringify(m));return m.data};
const check=await(await fetch(base+'/api/auth/check-user')).json();if(check.code===404)await api('POST','/auth/sign-up',{username:'smoke',password:'smoke-secret'});token=(await api('POST','/auth/sign-in',{username:'smoke',password:'smoke-secret'})).token;
const existing=await api('GET','/openlist-config');const c=await api(existing.length?'PUT':'POST','/openlist-config'+(existing.length?'/'+existing[0].id:''),{username:'mock',baseUrl:`http://127.0.0.1:${mock.address().port}`,token:'mock'});const tasks=await api('GET','/task-config');const t=tasks[0]||await api('POST','/task-config',{taskName:'movies',path:'/media',openlistConfigId:c.id,isIncrement:true});await api('POST',`/task-config/${t.id}/submit`,{});
let r;for(let i=0;i<100;i++){r=await api('GET',`/task-config/${t.id}/runs/latest`);if(r&&['SUCCESS','FAILED','PARTIAL_SUCCESS'].includes(r.status))break;await new Promise(r=>setTimeout(r,100))}if(r?.status!=='SUCCESS')throw Error(JSON.stringify(r));const content=readFileSync(join(t.strmPath,'movie.strm'),'utf8');if(!content.includes('sign=test%3D%3A0'))throw Error('STRM mismatch');const before=statSync(join(t.strmPath,'movie.strm')).mtimeMs;
await api('POST',`/task-config/${t.id}/submit`,{isIncremental:true});
for(let i=0;i<100;i++){r=await api('GET',`/task-config/${t.id}/runs/latest`);if(r&&['SUCCESS','FAILED','PARTIAL_SUCCESS'].includes(r.status))break;await new Promise(r=>setTimeout(r,100))}
if(r.status!=='SUCCESS'||r.skipped!==1||statSync(join(t.strmPath,'movie.strm')).mtimeMs!==before)throw Error('Incremental rewrite regression');
await stop();await start();token=(await api('POST','/auth/sign-in',{username:'smoke',password:'smoke-secret'})).token;if((await api('GET','/task-config')).length!==1)throw Error('Restart lost data');
console.log(JSON.stringify({binary,platform:process.platform,arch:process.arch,embeddedUI:true,auth:true,strm:true,incremental:true,restart:true}));
}finally{await stop();mock.close();}
