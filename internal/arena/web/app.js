const zh={title:'比赛大厅',subtitle:'实时竞赛与排名',teacher:'教师控制台',manage:'配置比赛、题面与实时评测',create:'创建比赛'};
const en={title:'Contest lobby',subtitle:'Live contests and rankings',teacher:'Teacher console',manage:'Configure contests, problems and judging',create:'Create contest'};
let locale=localStorage.locale||'zh'; const t=()=>locale==='zh'?zh:en;
function renderText(){document.querySelectorAll('[data-i18n]').forEach(x=>x.textContent=t()[x.dataset.i18n]);}
async function load(){const cs=await fetch('/api/contests').then(r=>r.json());const box=document.querySelector('#contests');box.innerHTML=cs.map(c=>`<article class="card"><div class="pill ${c.status}">${c.status}</div><h2>${c.name}</h2><p>${c.description||''}</p>${window.teacher?`<button onclick="start('${c.id}')">启动比赛</button>`:`<a href="#${c.id}">查看排名</a>`}</article>`).join('')||'<p class="muted">暂无比赛</p>';}
async function start(id){await fetch('/api/contests/'+id+'/start',{method:'POST'});load();}
document.querySelector('#create')?.addEventListener('submit',async e=>{e.preventDefault();const f=new FormData(e.target);await fetch('/api/contests',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(Object.fromEntries(f))});e.target.reset();load();});
renderText();load(); const es=new EventSource('/api/events'); es.onmessage=load;
