import React, { useState } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { ToastProvider } from '../components/ui/toast'
import { UbuntuDesktop } from '../components/UbuntuDesktop'
import type { Contest, Problem } from '../api/client'
import { useStore } from '../store/useStore'

// 开发预览挂载点（仅 import.meta.env.DEV 下可达）：直接渲染考场桌面做目检，
// 不进入生产构建（main.tsx 按 DEV + 查询串门控）。
const contest: Contest = {
  id: 'preview-1',
  name: 'CSP-J 第二轮 全真模拟',
  description: 'NOI Linux 2.0 环境全真模拟',
  status: 'running',
  mode: 'noi',
  season: '2026',
  ticketGroup: 'J',
  participantUserIds: ['u1', 'u2'],
}

const problem: Problem = {
  id: 'prb-1',
  contestId: 'preview-1',
  code: 'A',
  slug: 'poker',
  title: '扑克牌',
  statement: '## 题目描述\n\n小杨现在有 n 张扑克牌，每张牌的点数是 1 到 13 之间的整数。请统计每种点数出现的次数。\n\n## 输入格式\n\n第一行一个整数 n。\n\n## 输出格式\n\n输出 13 行。',
  input: '第一行一个整数 n。',
  output: '输出 13 行。',
  examples: '输入样例 1：\n6\n\n输出样例 1：\n0 1 0 0 0 0 1 0 0 0 0 1 3',
  constraints: '对于所有数据，保证 1 ≤ n ≤ 1,000,000。',
  timeLimitMs: 1000,
  memoryLimitMib: 512,
  ioMode: 'file',
  inFile: 'poker.in',
  outFile: 'poker.out',
  testCases: [],
}

export function DesktopPreview() {
  useStore.setState({
    session: {
      userId: 'u1',
      username: 'haj00001',
      name: '张伟',
      role: 'student',
      examTicket: 'HA-J00001',
    },
    authStatus: 'authenticated',
    desktopMode: true,
  })
  const [code, setCode] = useState(
    '#include <bits/stdc++.h>\nusing namespace std;\n\nint main() {\n    freopen("poker.in", "r", stdin);\n    freopen("poker.out", "w", stdout);\n    int n;\n    cin >> n;\n    vector<int> cnt(14);\n    for (int i = 0; i < n; i++) {\n        int x;\n        cin >> x;\n        cnt[x]++;\n    }\n    for (int i = 1; i <= 13; i++) cout << cnt[i] << "\\n";\n    return 0;\n}\n'
  )
  return (
    <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
      <ToastProvider>
      <UbuntuDesktop
        contest={contest}
        problems={[problem]}
        activeProblem={problem}
        setActiveProblem={() => {}}
        code={code}
        setCode={setCode}
        onExitDesktop={() => {}}
        onSubmit={() => {}}
        submitting={false}
        uploadFile={null}
        uploadMismatch={null}
        onPickUploadFile={() => {}}
        onClearUploadFile={() => {}}
      />
      </ToastProvider>
    </QueryClientProvider>
  )
}
