import {execFileSync} from 'node:child_process';
import {mkdirSync,copyFileSync,readFileSync,readdirSync,writeFileSync} from 'node:fs';
import {createHash} from 'node:crypto';
const version=process.env.OSTRM_VERSION||'3.0.0-dev';
if(!/^[0-9]+\.[0-9]+\.[0-9]+(?:-[a-zA-Z0-9.-]+)?$/.test(version))throw Error('Invalid version');
const commit=execFileSync('git',['rev-parse','HEAD'],{encoding:'utf8'}).trim();
const target=process.argv[2];const targets=target?[target]:['darwin/amd64','darwin/arm64','linux/amd64','linux/arm64','windows/amd64','windows/arm64'];
if(!readdirSync('internal/web/dist/_nuxt').some(p=>p.endsWith('.js')))throw Error('Generate frontend first');
for(const t of targets){const [goos,goarch]=t.split('/');const name=`ostrm_${version}_${goos}_${goarch}`;const dir=`dist/${name}`;mkdirSync(dir,{recursive:true});const binary=`${dir}/ostrm${goos==='windows'?'.exe':''}`;
execFileSync('go',['build','-trimpath','-ldflags',`-s -w -X github.com/Moersity/ostrm/internal/app.Version=${version} -X github.com/Moersity/ostrm/internal/app.Commit=${commit}`,'-o',binary,'./cmd/ostrm'],{stdio:'inherit',env:{...process.env,CGO_ENABLED:'0',GOOS:goos,GOARCH:goarch}});
copyFileSync('LICENSE',`${dir}/LICENSE`);copyFileSync('README-GO.md',`${dir}/README.md`);
writeFileSync(`${dir}/build-info.json`,JSON.stringify({version,commit,goos,goarch,sha256:createHash('sha256').update(readFileSync(binary)).digest('hex'),signed:false,nativeTested:false},null,2));
}
