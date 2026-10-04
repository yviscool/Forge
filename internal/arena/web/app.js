// Forge Web Application Logic
let currentLocale = localStorage.getItem('forge_locale') || 'zh-CN';
let activeContestId = null;
let contests = [];
let users = [];
let groups = [];
let problems = [];
let i18nDict = {};

async function initI18n() {
  try {
    const res = await fetch('/api/i18n');
    i18nDict = await res.json();
  } catch (e) {
    console.error('Failed to load i18n', e);
  }
}

function t(key) {
  const dict = i18nDict[currentLocale] || {};
  return dict[key] || key;
}

function updateLocaleUI() {
  document.querySelectorAll('[data-i18n]').forEach(el => {
    const key = el.dataset.i18n;
    if (t(key)) el.textContent = t(key);
  });
  const btn = document.getElementById('toggleLocale');
  if (btn) {
    btn.textContent = currentLocale === 'zh-CN' ? 'English' : '中文';
  }
}

document.getElementById('toggleLocale')?.addEventListener('click', () => {
  currentLocale = currentLocale === 'zh-CN' ? 'en-US' : 'zh-CN';
  localStorage.setItem('forge_locale', currentLocale);
  updateLocaleUI();
});

// Tab navigation for Teacher console
document.querySelectorAll('.tab-btn').forEach(btn => {
  btn.addEventListener('click', () => {
    document.querySelectorAll('.tab-btn').forEach(b => b.classList.remove('active'));
    document.querySelectorAll('.tab-panel').forEach(p => p.classList.remove('active'));
    btn.classList.add('active');
    const panelId = btn.dataset.tab;
    document.getElementById(panelId)?.classList.add('active');
  });
});

async function loadData() {
  await Promise.all([loadContests(), loadUsers(), loadGroups()]);
  if (activeContestId) {
    await Promise.all([loadProblems(activeContestId), loadSubmissions(activeContestId), loadRanking(activeContestId)]);
  }
}

async function loadContests() {
  const res = await fetch('/api/contests');
  contests = await res.json();

  if (window.teacherMode) {
    const select = document.getElementById('contestSelect');
    if (select) {
      select.innerHTML = contests.map(c => `<option value="${c.id}">${c.name} [${c.status}]</option>`).join('');
      if (!activeContestId && contests.length > 0) {
        activeContestId = contests[0].id;
      }
      select.value = activeContestId || '';
      updateContestStatusBar();
    }
  } else {
    const container = document.getElementById('studentContestList');
    if (container) {
      container.innerHTML = contests.map(c => `
        <div class="contest-card">
          <div>
            <span class="badge ${c.status}">${c.status}</span>
            <h3 class="mt-2">${c.name}</h3>
            <p>${c.description || '无附加说明'}</p>
          </div>
          <button class="btn-primary mt-2" onclick="enterContest('${c.id}')">进入比赛</button>
        </div>
      `).join('') || '<p class="muted">当前暂无发布的比赛</p>';
    }
  }
}

function updateContestStatusBar() {
  const current = contests.find(c => c.id === activeContestId);
  const badge = document.getElementById('contestStatusBadge');
  if (current && badge) {
    badge.className = `badge ${current.status}`;
    badge.textContent = current.status;
  }
}

document.getElementById('contestSelect')?.addEventListener('change', async (e) => {
  activeContestId = e.target.value;
  updateContestStatusBar();
  await Promise.all([loadProblems(activeContestId), loadSubmissions(activeContestId), loadRanking(activeContestId)]);
});

document.getElementById('startContestBtn')?.addEventListener('click', async () => {
  if (!activeContestId) return;
  await fetch(`/api/contests/${activeContestId}/start`, { method: 'POST' });
  await loadContests();
});

document.getElementById('finishContestBtn')?.addEventListener('click', async () => {
  if (!activeContestId) return;
  await fetch(`/api/contests/${activeContestId}/finish`, { method: 'POST' });
  await loadContests();
});

// Users & Groups
async function loadUsers() {
  const res = await fetch('/api/users');
  users = await res.json();

  const userList = document.getElementById('userList');
  if (userList) {
    userList.innerHTML = `
      <table>
        <thead><tr><th>姓名</th><th>角色</th><th>已加入组</th></tr></thead>
        <tbody>
          ${users.map(u => `<tr><td>${u.name}</td><td>${u.role}</td><td>${(u.groups || []).join(', ') || '-'}</td></tr>`).join('')}
        </tbody>
      </table>
    `;
  }

  const assignUserSelect = document.getElementById('assignUserSelect');
  if (assignUserSelect) {
    assignUserSelect.innerHTML = users.map(u => `<option value="${u.id}">${u.name} (${u.role})</option>`).join('');
  }

  const submitUserSelect = document.getElementById('submitUserSelect');
  if (submitUserSelect) {
    submitUserSelect.innerHTML = users.map(u => `<option value="${u.id}">${u.name}</option>`).join('');
  }
}

async function loadGroups() {
  const res = await fetch('/api/groups');
  groups = await res.json();

  const groupList = document.getElementById('groupList');
  if (groupList) {
    groupList.innerHTML = `
      <table>
        <thead><tr><th>分组名称</th><th>成员数量</th></tr></thead>
        <tbody>
          ${groups.map(g => `<tr><td>${g.name}</td><td>${(g.userIds || []).length} 人</td></tr>`).join('')}
        </tbody>
      </table>
    `;
  }

  const assignGroupSelect = document.getElementById('assignGroupSelect');
  if (assignGroupSelect) {
    assignGroupSelect.innerHTML = groups.map(g => `<option value="${g.id}">${g.name}</option>`).join('');
  }
}

// Problems
async function loadProblems(cid) {
  if (!cid) return;
  const res = await fetch(`/api/contests/${cid}/problems`);
  problems = await res.json();

  if (window.teacherMode) {
    const list = document.getElementById('problemList');
    if (list) {
      list.innerHTML = problems.map(p => `
        <div class="problem-card">
          <div class="card-header">
            <h4>${p.code}. ${p.title}</h4>
            <div class="inline-actions">
              <button class="btn-secondary" onclick="validateProblem('${cid}', '${p.id}')">校验题面</button>
              <a href="/api/contests/${cid}/problems/${p.id}/export" target="_blank" class="btn-secondary">CCF 预览/导出</a>
            </div>
          </div>
          <p class="muted">${p.statement.substring(0, 100)}...</p>
        </div>
      `).join('') || '<p class="muted">当前比赛暂无试题，请从左侧表单添加</p>';
    }
  } else {
    const pills = document.getElementById('problemPills');
    if (pills) {
      pills.innerHTML = problems.map((p, idx) => `
        <button class="pill-btn ${idx === 0 ? 'active' : ''}" onclick="selectStudentProblem('${p.id}')">${p.code}. ${p.title}</button>
      `).join('');
      if (problems.length > 0) {
        selectStudentProblem(problems[0].id);
      }
    }
  }
}

window.validateProblem = async function(cid, pid) {
  const res = await fetch(`/api/contests/${cid}/problems/${pid}/validate`, { method: 'POST' });
  const data = await res.json();
  if (data.valid) {
    alert('✅ 题面校验通过：满足 CCF 规范结构！');
  } else {
    alert('❌ 题面校验失败：' + data.error);
  }
};

window.selectStudentProblem = function(pid) {
  const p = problems.find(x => x.id === pid);
  if (!p) return;
  const detail = document.getElementById('problemDetailView');
  const title = document.getElementById('currentProblemTitle');
  if (title) title.textContent = `${p.code}. ${p.title}`;
  if (detail) {
    detail.innerHTML = `
      <div class="mb-4">
        <span class="badge running">时限: ${p.timeLimitMs}ms</span>
        <span class="badge running">空间: ${p.memoryLimitMib}MiB</span>
        <a href="/api/contests/${activeContestId}/problems/${p.id}/export" target="_blank" class="btn-secondary" style="float: right; padding: 2px 8px; font-size: 12px;">查看规范排版</a>
      </div>
      <div class="bold">【题目描述】</div>
      <p>${p.statement}</p>
      <div class="bold mt-2">【输入格式】</div>
      <p>${p.input}</p>
      <div class="bold mt-2">【输出格式】</div>
      <p>${p.output}</p>
      <div class="sample-box mt-2">
        <div class="bold">样例</div>
        <pre>${p.examples || '暂无样例'}</pre>
      </div>
      <div class="bold mt-2">【数据范围】</div>
      <p class="muted">${p.constraints}</p>
    `;
  }
};

// Submissions & Ranking
async function loadSubmissions(cid) {
  if (!cid) return;
  const res = await fetch(`/api/contests/${cid}/submissions`);
  const subs = await res.json();

  const container = window.teacherMode
    ? document.getElementById('submissionTableBox')
    : document.getElementById('studentSubmissionsBox');

  if (container) {
    container.innerHTML = `
      <table>
        <thead><tr><th>ID</th><th>选手</th><th>语言</th><th>状态</th><th>得分</th>${window.teacherMode ? '<th>评测操作</th>' : ''}</tr></thead>
        <tbody>
          ${subs.map(s => `
            <tr>
              <td><code>${s.id}</code></td>
              <td>${s.userName || s.userId}</td>
              <td>${s.language}</td>
              <td><span class="badge ${s.verdict}">${s.verdict}</span></td>
              <td><strong>${s.score}</strong></td>
              ${window.teacherMode ? `
                <td>
                  <button class="btn-secondary" onclick="manualJudge('${s.id}', 'accepted', 100)">通过 (100)</button>
                  <button class="btn-secondary" onclick="manualJudge('${s.id}', 'wrong_answer', 0)">错误 (0)</button>
                </td>
              ` : ''}
            </tr>
          `).join('') || '<tr><td colspan="6" class="muted">暂无提交记录</td></tr>'}
        </tbody>
      </table>
    `;
  }
}

window.manualJudge = async function(id, verdict, score) {
  await fetch(`/api/submissions/${id}/judge`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ verdict, score }),
  });
  if (activeContestId) {
    await Promise.all([loadSubmissions(activeContestId), loadRanking(activeContestId)]);
  }
};

async function loadRanking(cid) {
  if (!cid) return;
  const res = await fetch(`/api/contests/${cid}/ranking`);
  const ranks = await res.json();

  const container = window.teacherMode
    ? document.getElementById('rankingTableBox')
    : document.getElementById('studentRankingBox');

  if (container) {
    container.innerHTML = `
      <table>
        <thead><tr><th>名次</th><th>选手</th><th>总分</th><th>AC 数</th></tr></thead>
        <tbody>
          ${ranks.map((r, i) => `
            <tr>
              <td><strong>#${i + 1}</strong></td>
              <td>${r.userName}</td>
              <td><strong style="color: var(--primary);">${r.score}</strong></td>
              <td>${r.accepted}</td>
            </tr>
          `).join('') || '<tr><td colspan="4" class="muted">当前暂无排名数据</td></tr>'}
        </tbody>
      </table>
    `;
  }
}

// Student enter contest
window.enterContest = async function(cid) {
  activeContestId = cid;
  document.getElementById('contestWorkspace').style.display = 'block';
  await Promise.all([loadProblems(cid), loadSubmissions(cid), loadRanking(cid)]);
  window.scrollTo({ top: 300, behavior: 'smooth' });
};

// Form Handlers
document.getElementById('createContestForm')?.addEventListener('submit', async (e) => {
  e.preventDefault();
  const f = new FormData(e.target);
  const res = await fetch('/api/contests', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(Object.fromEntries(f)),
  });
  const data = await res.json();
  activeContestId = data.id;
  e.target.reset();
  await loadContests();
});

document.getElementById('problemForm')?.addEventListener('submit', async (e) => {
  e.preventDefault();
  if (!activeContestId) {
    alert('请先选择或创建比赛！');
    return;
  }
  const f = new FormData(e.target);
  const data = Object.fromEntries(f);
  data.timeLimitMs = parseInt(data.timeLimitMs, 10) || 1000;
  data.memoryLimitMib = parseInt(data.memoryLimitMib, 10) || 512;

  await fetch(`/api/contests/${activeContestId}/problems`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(data),
  });
  e.target.reset();
  await loadProblems(activeContestId);
});

document.getElementById('createUserForm')?.addEventListener('submit', async (e) => {
  e.preventDefault();
  const f = new FormData(e.target);
  await fetch('/api/users', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(Object.fromEntries(f)),
  });
  e.target.reset();
  await loadUsers();
});

document.getElementById('createGroupForm')?.addEventListener('submit', async (e) => {
  e.preventDefault();
  const f = new FormData(e.target);
  await fetch('/api/groups', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(Object.fromEntries(f)),
  });
  e.target.reset();
  await loadGroups();
});

document.getElementById('assignGroupBtn')?.addEventListener('click', async () => {
  if (!activeContestId) return;
  const gid = document.getElementById('assignGroupSelect').value;
  await fetch(`/api/contests/${activeContestId}/groups`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ groupId: gid }),
  });
  alert('已将该分组绑定到比赛！');
});

document.getElementById('assignUserBtn')?.addEventListener('click', async () => {
  if (!activeContestId) return;
  const uid = document.getElementById('assignUserSelect').value;
  await fetch(`/api/contests/${activeContestId}/participants`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ userId: uid }),
  });
  alert('已将该个人绑定到比赛！');
});

document.getElementById('studentSubmitForm')?.addEventListener('submit', async (e) => {
  e.preventDefault();
  if (!activeContestId || problems.length === 0) return;
  const f = new FormData(e.target);
  const data = Object.fromEntries(f);
  data.problemID = problems[0].id; // submits to current active problem
  const activePill = document.querySelector('.pill-btn.active');
  if (activePill) {
    const p = problems.find(x => activePill.textContent.includes(x.code));
    if (p) data.problemID = p.id;
  }

  const res = await fetch(`/api/contests/${activeContestId}/submissions`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(data),
  });
  if (res.ok) {
    alert('提交成功！等待系统评测...');
    await loadSubmissions(activeContestId);
  } else {
    const err = await res.json();
    alert('提交失败: ' + (err.error || '未知错误'));
  }
});

// SSE Real-time events
const es = new EventSource('/api/events');
es.onmessage = async (e) => {
  try {
    const evt = JSON.parse(e.data);
    if (activeContestId && evt.contestId && evt.contestId !== activeContestId) {
      return; // filter events for other contests
    }
    if (evt.type.startsWith('submission') || evt.type.startsWith('ranking')) {
      if (activeContestId) {
        await Promise.all([loadSubmissions(activeContestId), loadRanking(activeContestId)]);
      }
    } else if (evt.type.startsWith('contest')) {
      await loadContests();
    }
  } catch (err) {
    console.error('SSE Error:', err);
  }
};

// Initialize
initI18n().then(() => {
  updateLocaleUI();
  loadData();
});
