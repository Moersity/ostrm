import {execFileSync} from 'node:child_process';
import {rmSync,cpSync,existsSync,readdirSync} from 'node:fs';
const npm=process.platform==='win32'?'npm.cmd':'npm';
execFileSync(npm,['ci','--no-audit','--no-fund'],{cwd:'frontend',stdio:'inherit',shell:process.platform==='win32'});
execFileSync(npm,['run','generate'],{cwd:'frontend',stdio:'inherit',shell:process.platform==='win32'});
const src='frontend/.output/public';
if(!existsSync(`${src}/index.html`)||!readdirSync(`${src}/_nuxt`).some(x=>x.endsWith('.js')))throw Error('Incomplete frontend');
rmSync('internal/web/dist',{recursive:true,force:true});cpSync(src,'internal/web/dist',{recursive:true});
