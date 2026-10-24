import React, { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import {
  api,
  type ContestMode,
  type CompareMode,
  type ChoiceQuestion,
  type TestFileInventory,
  type TestCase,
  Problem,
  RankingMode,
  type TaskType,
  type User,
} from '../api/client'
import { Button } from '../components/ui/button'
import { Input, Select, Textarea } from '../components/ui/input'
import { Badge } from '../components/ui/badge'
import { Card, CardHeader, CardTitle, CardContent } from '../components/ui/card'
import { Dialog } from '../components/ui/dialog'
import { useToast } from '../components/ui/toast'
import { Scoreboard } from '../components/Scoreboard'
import { MarkdownView } from '../components/MarkdownView'
import { ChoicePaperEditor } from '../components/ChoicePaperEditor'
import { MonitorBoard } from '../components/MonitorBoard'
import { ProvincesPanel } from '../components/ProvincesPanel'
import { useStore } from '../store/useStore'
import { EditorLoading } from '../components/ui/editor-loading'
import { InlineError } from '../components/ui/empty-state'
import { useSSE } from '../realtime/useSSE'
import {
  Users,
  Layers,
  FileCode,
  LineChart,
  Activity,
  Play,
  Square,
  Plus,
  ArrowUp,
  ArrowDown,
  Printer,
  FileDown,
  CheckCircle2,
  XCircle,
  FileEdit,
  GitCompare,
  Upload,
  Download,
  ShieldAlert,
  ShieldCheck,
  Settings,
  KeyRound,
  Trash2,
  RefreshCw,
  FolderArchive,
  Info,
  MapPin,
  UserPen,
  Radar,
} from 'lucide-react'
import { TeacherSecurityPanel } from '../components/TeacherSecurityPanel'

const CodeDiffViewer = React.lazy(() =>
  import('../components/CodeDiffViewer').then((module) => ({
    default: module.CodeDiffViewer,
  }))
)

type TeacherTab = 'contests' | 'users' | 'provinces' | 'problems' | 'analytics' | 'submissions' | 'security' | 'monitor'

const TAB_ITEMS: Array<{
  key: TeacherTab
  label: string
  icon: typeof Layers
}> = [
  { key: 'contests', label: '比赛概览', icon: Layers },
  { key: 'users', label: '用户与编组', icon: Users },
  { key: 'provinces', label: '省份与考号', icon: MapPin },
  { key: 'problems', label: '试题工坊', icon: FileCode },
  { key: 'analytics', label: '学情分析', icon: GitCompare },
  { key: 'submissions', label: '实时提交', icon: Activity },
  { key: 'security', label: '考务安全', icon: ShieldCheck },
  { key: 'monitor', label: '监考大屏', icon: Radar },
]

const errorText = (cause: unknown) =>
  cause instanceof Error ? cause.message : '服务暂时不可用，请稍后重试。'

export function generateInitialPassword(length = 16): string {
  const alphabet = 'ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789!@#$%'
  const bytes = new Uint8Array(Math.max(10, length))
  if (typeof crypto !== 'undefined' && 'getRandomValues' in crypto) {
    crypto.getRandomValues(bytes)
  } else {
    for (let i = 0; i < bytes.length; i += 1) bytes[i] = Math.floor(Math.random() * 256)
  }
  return Array.from(bytes, (byte) => alphabet[byte % alphabet.length]).join('')
}

function choicePaperError(questions: ChoiceQuestion[]): string | null {
  if (!questions.length) return '客观题至少需要一道小题。'
  const seen = new Set<number>()
  for (const [index, question] of questions.entries()) {
    const label = `第 ${index + 1} 题`
    if (!Number.isInteger(question.id) || question.id <= 0) return `${label}的题号无效。`
    if (seen.has(question.id)) return `${label}的题号重复。`
    seen.add(question.id)
    if (question.answer == null) return `${label}缺少标准答案。`
    if (!question.stem.trim()) return `${label}缺少题干。`
    if (!Number.isFinite(question.score) || question.score <= 0) return `${label}分值必须大于 0。`
    if (question.type === 'judge') {
      if (question.answer !== 'T' && question.answer !== 'F') return `${label}的判断题答案必须是 T 或 F。`
      continue
    }
    const options = question.options || []
    if (options.length < 2 || options.some((option) => !option.trim())) {
      return `${label}至少需要两个非空选项。`
    }
    const answers = Array.isArray(question.answer) ? question.answer : [question.answer]
    if (!answers.length || answers.some((answer) => !String(answer).trim())) {
      return `${label}缺少正确答案。`
    }
    if (question.type === 'single' && answers.length !== 1) {
      return `${label}是单选题，只能设置一个答案。`
    }
    const valid = new Set(options.map((_, optionIndex) => String.fromCharCode(65 + optionIndex)))
    if (answers.some((answer) => !valid.has(String(answer).toUpperCase()))) {
      return `${label}的答案不在选项范围内。`
    }
  }
  return null
}

interface SubtaskDraft {
  subtask: number
  cases: number
  score: number
  timeLimitMs: number
  memoryMib: number
  dependsOn: string
}

function groupSubtaskDrafts(testCases: TestCase[] = []): SubtaskDraft[] {
  const groups = new Map<number, SubtaskDraft & { dependencySet: Set<number> }>()
  for (const testCase of testCases) {
    const subtask = Number.isInteger(testCase.subtask) ? testCase.subtask : 0
    const current = groups.get(subtask) ?? {
      subtask,
      cases: 0,
      score: 0,
      timeLimitMs: 0,
      memoryMib: 0,
      dependsOn: '',
      dependencySet: new Set<number>(),
    }
    current.cases += 1
    current.score += Number(testCase.score) || 0
    current.timeLimitMs ||= Number(testCase.timeLimitMs) || 0
    current.memoryMib ||= Number(testCase.memoryMib) || 0
    for (const dependency of testCase.dependsOn || []) {
      current.dependencySet.add(dependency)
    }
    groups.set(subtask, current)
  }
  return [...groups.values()]
    .sort((left, right) => left.subtask - right.subtask)
    .map(({ dependencySet, ...group }) => ({
      ...group,
      dependsOn: [...dependencySet].sort((left, right) => left - right).join(','),
    }))
}

export function TeacherConsole() {
  const toast = useToast()
  const session = useStore((s) => s.session)
  const isAdmin = session?.role === 'admin'
  // 支持托盘/菜单深链：/teacher?tab=monitor 直达监考大屏。
  const [activeTab, setActiveTab] = useState<TeacherTab>(() => {
    if (typeof window === 'undefined') return 'contests'
    const tab = new URLSearchParams(window.location.search).get('tab')
    return (TAB_ITEMS.some(({ key }) => key === tab) ? (tab as TeacherTab) : 'contests')
  })
  const [pending, setPending] = useState<string | null>(null)

  // Selected contest
  const [cid, setCid] = useState<string>(() => {
    if (typeof window === 'undefined') return ''
    return new URLSearchParams(window.location.search).get('contest') || localStorage.getItem('forge_teacher_cid') || ''
  })

  // Sub-tab / Modal states
  const [showCreateContest, setShowCreateContest] = useState(false)
  const [showImportBundle, setShowImportBundle] = useState(false)
  const [bundleName, setBundleName] = useState('')
  const [bundlePreview, setBundlePreview] = useState<{
    name: string
    description: string
    problems: number
  } | null>(null)
  const [bundleRaw, setBundleRaw] = useState<unknown>(null)
  const [bundleError, setBundleError] = useState('')
  const [bundleBusy, setBundleBusy] = useState(false)
  const bundleInputRef = React.useRef<HTMLInputElement>(null)

  const handleTabKeyDown = (
    event: React.KeyboardEvent<HTMLButtonElement>,
    index: number
  ) => {
    let next: number
    if (event.key === 'ArrowRight') next = (index + 1) % TAB_ITEMS.length
    else if (event.key === 'ArrowLeft') next = (index - 1 + TAB_ITEMS.length) % TAB_ITEMS.length
    else if (event.key === 'Home') next = 0
    else if (event.key === 'End') next = TAB_ITEMS.length - 1
    else return
    event.preventDefault()
    const key = TAB_ITEMS[next].key
    setActiveTab(key)
    requestAnimationFrame(() => {
      document.getElementById(`teacher-tab-${key}`)?.focus()
    })
  }
  const [languageDraft, setLanguageDraft] = useState<string[]>([])
  const [newContestForm, setNewContestForm] = useState<{
    name: string
    description: string
    mode: ContestMode
    rankingMode: RankingMode
  }>({ name: '', description: '', rankingMode: 'oi', mode: 'online' })

  // Problem Editor State
  const [editingProblem, setEditingProblem] = useState<Partial<Problem> | null>(null)
  const [problemValidationMsg, setProblemValidationMsg] = useState<{ ok: boolean; msg: string } | null>(null)

  // User Management State
  const [newGroupForm, setNewGroupForm] = useState({ name: '' })
  const [groupMemberDraft, setGroupMemberDraft] = useState<Record<string, string>>({})
  const [newUserForm, setNewUserForm] = useState({ name: '', username: '', role: 'student', password: '', province: '' })
  const [selectedUserIds, setSelectedUserIds] = useState<string[]>([])
  const [editingUser, setEditingUser] = useState<User | null>(null)
  const [editUserForm, setEditUserForm] = useState({ name: '', username: '', role: '', province: '' })
  const [editUserBusy, setEditUserBusy] = useState(false)
  const [showBatchProvince, setShowBatchProvince] = useState(false)
  const [batchProvinceCode, setBatchProvinceCode] = useState('')
  const [batchProvinceBusy, setBatchProvinceBusy] = useState(false)
  const [csvContent, setCsvContent] = useState('')
  const [showImportCsv, setShowImportCsv] = useState(false)
  const [examUserPrefix, setExamUserPrefix] = useState('NOIP2026-')
  const [examUserCount, setExamUserCount] = useState(20)
  const [examUserPassword, setExamUserPassword] = useState('')
  const [isGeneratingUsers, setIsGeneratingUsers] = useState(false)
  const [seatMode, setSeatMode] = useState<'prefix' | 'csp'>('csp')
  const [cspProvince, setCspProvince] = useState('')
  const [cspGroup, setCspGroup] = useState<'J' | 'S'>('J')
  const [cspDigits, setCspDigits] = useState(5)
  const [cspCount, setCspCount] = useState(20)
  const [isGeneratingCSP, setIsGeneratingCSP] = useState(false)
  const [showSeatCardModal, setShowSeatCardModal] = useState(false)
  const [generatedCredentials, setGeneratedCredentials] = useState<
    Array<{ username: string; password: string; name: string; ticket?: string }>
  >([])
  const [resetUser, setResetUser] = useState<User | null>(null)
  const [resetPassword, setResetPassword] = useState('')
  const [resetBusy, setResetBusy] = useState(false)
  const [testFiles, setTestFiles] = useState<Record<string, TestFileInventory>>({})
  const [testFileBusy, setTestFileBusy] = useState('')
  const [testFileError, setTestFileError] = useState('')
  const [answerKeyDraft, setAnswerKeyDraft] = useState('')
  const [answerKeyBusy, setAnswerKeyBusy] = useState(false)
  const [answerKeyError, setAnswerKeyError] = useState('')
  const [subtaskDrafts, setSubtaskDrafts] = useState<SubtaskDraft[]>([])
  const [subtaskBusy, setSubtaskBusy] = useState(false)
  const [subtaskError, setSubtaskError] = useState('')
  const testFileInputRef = React.useRef<HTMLInputElement>(null)
  const [judgeDrafts, setJudgeDrafts] = useState<Record<string, { verdict: string; score: number }>>({})
  const [judgeBusy, setJudgeBusy] = useState('')
  const [expandedSubmissionId, setExpandedSubmissionId] = useState('')

  // Diff / Progression State
  const [selectedStudentId, setSelectedStudentId] = useState<string>('')
  const [selectedProblemId, setSelectedProblemId] = useState<string>('')
  const [diffVerA, setDiffVerA] = useState<number>(0)
  const [diffVerB, setDiffVerB] = useState<number>(0)

  // Queries
  const {
    data: contests = [],
    refetch: refetchContests,
    isError: contestsError,
    error: contestsErrorCause,
  } = useQuery({
    queryKey: ['contests'],
    queryFn: api.contests,
  })

  // Set default contest
  React.useEffect(() => {
    if (contests.length > 0 && (!cid || !contests.some((contest) => contest.id === cid))) {
      setCid(contests[0].id)
    }
  }, [contests, cid])

  const currentContest = contests.find((c) => c.id === cid) || contests[0]
  const paperEditable = currentContest?.status === 'draft'

  React.useEffect(() => {
    setLanguageDraft(currentContest?.allowedLanguages || [])
  }, [currentContest?.id, currentContest?.allowedLanguages])

  React.useEffect(() => {
    if (!paperEditable) setEditingProblem(null)
  }, [paperEditable])

  React.useEffect(() => {
    if (!cid || typeof window === 'undefined') return
    localStorage.setItem('forge_teacher_cid', cid)
    const url = new URL(window.location.href)
    if (url.searchParams.get('contest') !== cid) {
      url.searchParams.set('contest', cid)
      window.history.replaceState(null, '', `${url.pathname}${url.search}${url.hash}`)
    }
  }, [cid])

  const {
    data: problems = [],
    refetch: refetchProblems,
    isError: problemsError,
    error: problemsErrorCause,
  } = useQuery({
    queryKey: ['problems', cid],
    queryFn: () => api.problems(cid),
    enabled: !!cid,
  })

  const editingProblemId = editingProblem?.id
  const editingTestFiles = editingProblemId ? testFiles[editingProblemId] : undefined

  React.useEffect(() => {
    if (!cid || !editingProblemId) return
    let cancelled = false
    setTestFileError('')
    api
      .testFiles(cid, editingProblemId)
      .then((inventory) => {
        if (!cancelled) {
          setTestFiles((prev) => ({ ...prev, [editingProblemId]: inventory }))
        }
      })
      .catch((cause) => {
        if (!cancelled) setTestFileError(errorText(cause))
      })
    return () => {
      cancelled = true
    }
  }, [cid, editingProblemId, editingProblem?.taskType])

  const editingAnswerKey = editingProblem?.testCases?.[0]?.outputFile || ''
  const editingAnswerKeyName =
    editingAnswerKey.split(/[\\/]/).pop() || editingAnswerKey
  const editingAnswerKeyIsFile = Boolean(
    editingAnswerKey &&
      editingTestFiles?.files.some((name) => name === editingAnswerKeyName)
  )
  React.useEffect(() => {
    if (editingProblem?.taskType !== 'answers_only') {
      setAnswerKeyDraft('')
      setAnswerKeyError('')
      return
    }
    if (!cid || !editingProblemId || !editingAnswerKey) {
      setAnswerKeyDraft('')
      return
    }
    if (!editingAnswerKeyIsFile) {
      setAnswerKeyDraft(editingAnswerKey)
      setAnswerKeyError('')
      return
    }
    let cancelled = false
    if (!editingAnswerKeyName) {
      setAnswerKeyDraft('')
      return
    }
    api
      .fetchTestFile(cid, editingProblemId, editingAnswerKeyName)
      .then(({ blob }) => blob.text())
      .then((text) => {
        if (!cancelled) setAnswerKeyDraft(text)
      })
      .catch((cause) => {
        if (!cancelled) setAnswerKeyError(errorText(cause))
      })
    return () => {
      cancelled = true
    }
  }, [
    cid,
    editingProblemId,
    editingProblem?.taskType,
    editingAnswerKey,
    editingAnswerKeyIsFile,
    editingAnswerKeyName,
  ])

  React.useEffect(() => {
    setSubtaskDrafts(groupSubtaskDrafts(editingProblem?.testCases))
    setSubtaskError('')
  }, [editingProblemId, editingProblem?.testCases])

  const refreshTestFiles = async (pid: string) => {
    if (!cid) return
    const inventory = await api.testFiles(cid, pid)
    setTestFiles((prev) => ({ ...prev, [pid]: inventory }))
  }

  const handleUploadTestFiles = async (fileList: FileList | null) => {
    if (!cid || !editingProblemId || !fileList?.length) return
    setTestFileBusy('upload')
    setTestFileError('')
    try {
      const answerOnly = editingProblem?.taskType === 'answers_only'
      const accepted =
        answerOnly ? ['.out', '.ans'] : ['.in', '.out', '.ans']
      const files = Array.from(fileList)
      const invalid = files.filter(
        (file) => !accepted.some((suffix) => file.name.toLowerCase().endsWith(suffix))
      )
      if (invalid.length) {
        throw new Error(
          answerOnly
            ? `只交答案题仅支持 .out / .ans 答案 key：${invalid.map((f) => f.name).join('、')}`
            : `仅支持 .in / .out / .ans 测试数据：${invalid.map((f) => f.name).join('、')}`
        )
      }
      for (const file of files) {
        if (file.size > 10 * 1024 * 1024) {
          throw new Error(`${file.name} 超过单文件 10 MB 上限。`)
        }
        await api.uploadTestFile(cid, editingProblemId, file.name, file)
      }
      await refreshTestFiles(editingProblemId)
      toast.success('测试数据已上传', `共 ${files.length} 个文件。`)
    } catch (cause) {
      setTestFileError(errorText(cause))
      toast.error('测试数据上传失败', errorText(cause))
    } finally {
      setTestFileBusy('')
      if (testFileInputRef.current) testFileInputRef.current.value = ''
    }
  }

  const handleDeleteTestFile = async (name: string) => {
    if (!cid || !editingProblemId) return
    setTestFileBusy(`delete:${name}`)
    setTestFileError('')
    try {
      await api.deleteTestFile(cid, editingProblemId, name)
      await refreshTestFiles(editingProblemId)
      toast.success('测试文件已删除', name)
    } catch (cause) {
      setTestFileError(errorText(cause))
      toast.error('删除测试文件失败', errorText(cause))
    } finally {
      setTestFileBusy('')
    }
  }

  const handleSyncTestCases = async () => {
    if (!cid || !editingProblemId) return
    if (editingTestFiles?.missingPairs.length) {
      setTestFileError(
        `以下文件缺少配对，尚未同步：${editingTestFiles.missingPairs.join('、')}`
      )
      toast.error('测试数据尚未成对', '请先补齐 .in 与 .out/.ans 文件。')
      return
    }
    setTestFileBusy('sync')
    setTestFileError('')
    try {
      const updated = await api.syncTestCases(cid, editingProblemId)
      setEditingProblem((prev) =>
        prev && prev.id === updated.id ? { ...prev, testCases: updated.testCases } : prev
      )
      await refreshTestFiles(editingProblemId)
      await refetchProblems()
      toast.success(
        editingProblem.taskType === 'answers_only' ? '答案 key 已同步' : '测试点已生成',
        editingProblem.taskType === 'answers_only'
          ? '学生提交的答案将与该标准答案自动比对。自动评测是否执行取决于服务端 FORGE_AUTOJUDGE 配置。'
          : `已同步 ${updated.testCases?.length ?? 0} 组测试点。自动评测是否执行取决于服务端 FORGE_AUTOJUDGE 配置；默认保留人工复核。`
      )
    } catch (cause) {
      setTestFileError(errorText(cause))
      toast.error('同步测试点失败', errorText(cause))
    } finally {
      setTestFileBusy('')
    }
  }

  const handleSaveInlineAnswerKey = async () => {
    if (!cid || !editingProblemId) return
    const value = answerKeyDraft
    if (!value.trim()) {
      setAnswerKeyError('标准答案不能为空。')
      return
    }
    setAnswerKeyBusy(true)
    setAnswerKeyError('')
    try {
      const previous = editingProblem?.testCases?.[0]
      const updated = await api.updateProblem(cid, editingProblemId, {
        testCases: [
          {
            id: previous?.id || 'answer',
            inputFile: '',
            outputFile: value,
            score: 100,
            subtask: previous?.subtask ?? 0,
            timeLimitMs: previous?.timeLimitMs,
            memoryMib: previous?.memoryMib,
            dependsOn: previous?.dependsOn,
          },
        ],
      })
      setEditingProblem((current) =>
        current && current.id === updated.id
          ? { ...current, testCases: updated.testCases }
          : current
      )
      await refetchProblems()
      toast.success('标准答案已保存', '开赛预检会确保该题恰好包含一个答案 key。')
    } catch (cause) {
      setAnswerKeyError(errorText(cause))
      toast.error('保存标准答案失败', errorText(cause))
    } finally {
      setAnswerKeyBusy(false)
    }
  }

  const handleSaveSubtasks = async () => {
    if (!cid || !editingProblemId) return
    const total = subtaskDrafts.reduce((sum, item) => sum + (Number(item.score) || 0), 0)
    if (total !== 100) {
      setSubtaskError(`全部子任务总分必须为 100，当前为 ${total}。`)
      return
    }
    const known = new Set(subtaskDrafts.map((item) => item.subtask))
    setSubtaskBusy(true)
    setSubtaskError('')
    try {
      const parsed = subtaskDrafts.map((item) => {
        const dependsOn = item.dependsOn
          .split(/[\s,，]+/)
          .filter(Boolean)
          .map((value) => Number(value))
        if (dependsOn.some((value) => !Number.isInteger(value))) {
          throw new Error(`子任务 ${item.subtask} 的依赖格式无效。`)
        }
        if (dependsOn.includes(item.subtask)) {
          throw new Error(`子任务 ${item.subtask} 不能依赖自身。`)
        }
        const missing = dependsOn.find((value) => !known.has(value))
        if (missing != null) {
          throw new Error(`子任务 ${item.subtask} 依赖了不存在的子任务 ${missing}。`)
        }
        return {
          subtask: item.subtask,
          score: Number(item.score) || 0,
          timeLimitMs: Number(item.timeLimitMs) || 0,
          memoryMib: Number(item.memoryMib) || 0,
          dependsOn,
        }
      })
      const updated = await api.configureSubtasks(cid, editingProblemId, parsed)
      setEditingProblem((current) =>
        current && current.id === updated.id
          ? { ...current, testCases: updated.testCases }
          : current
      )
      await refetchProblems()
      toast.success('子任务配置已保存', '分数、限额与依赖已经写入全部测试点。')
    } catch (cause) {
      setSubtaskError(errorText(cause))
      toast.error('保存子任务失败', errorText(cause))
    } finally {
      setSubtaskBusy(false)
    }
  }

  const updateSubtaskDraft = (
    subtask: number,
    patch: Partial<Omit<SubtaskDraft, 'subtask' | 'cases'>>
  ) => {
    setSubtaskDrafts((current) =>
      current.map((item) => (item.subtask === subtask ? { ...item, ...patch } : item))
    )
    setSubtaskError('')
  }

  const handleDeleteProblem = async (problem: Problem) => {
    if (!cid) return
    const confirmed = window.confirm(
      `确定删除题目 ${problem.code}. ${problem.title}？已产生的历史提交会保留，便于审计与成绩追溯。此操作不可撤销。`
    )
    if (!confirmed) return
    setPending(`delete-problem:${problem.id}`)
    try {
      await api.deleteProblem(cid, problem.id)
      if (editingProblemId === problem.id) setEditingProblem(null)
      setTestFiles((prev) => {
        const next = { ...prev }
        delete next[problem.id]
        return next
      })
      await refetchProblems()
      await refetchContests()
      toast.success('题目已删除', `${problem.code}. ${problem.title}`)
    } catch (cause) {
      toast.error('删除题目失败', errorText(cause))
    } finally {
      setPending(null)
    }
  }

  const {
    data: users = [],
    refetch: refetchUsers,
    isError: usersError,
    error: usersErrorCause,
  } = useQuery({
    queryKey: ['users'],
    queryFn: api.users,
  })

  const {
    data: provinces = [],
    isLoading: provincesLoading,
    error: provincesError,
    refetch: refetchProvinces,
  } = useQuery({
    queryKey: ['provinces'],
    queryFn: api.provinces,
  })

  const enabledProvinces = provinces.filter((p) => p.enabled)
  const provinceName = (code?: string) =>
    provinces.find((p) => p.code === code)?.name || code || ''

  const seatCardEntries =
    generatedCredentials.length > 0
      ? generatedCredentials
      : users.slice(0, 6).map((user) => ({
          username: user.username || user.id,
          // Existing accounts keep their real password server-side; never
          // print a guessed credential on a seat card.
          password: '密码未知，请由教师重置',
          name: user.name,
          ticket: user.examTicket,
        }))

  const {
    data: groups = [],
    refetch: refetchGroups,
    isError: groupsError,
    error: groupsErrorCause,
  } = useQuery({
    queryKey: ['groups'],
    queryFn: api.groups,
  })

  const {
    data: submissions = [],
    refetch: refetchSubmissions,
    isError: submissionsError,
    error: submissionsErrorCause,
  } = useQuery({
    queryKey: ['submissions', cid],
    queryFn: () => api.submissions(cid),
    enabled: !!cid,
  })

  const {
    data: ranks = [],
    refetch: refetchRanks,
    isError: ranksError,
    error: ranksErrorCause,
  } = useQuery({
    queryKey: ['ranks', cid],
    queryFn: () => api.ranking(cid),
    enabled: !!cid,
  })
  const statisticsQuery = useQuery({
    queryKey: ['statistics', cid],
    queryFn: () => api.statistics(cid),
    enabled: !!cid && activeTab === 'analytics',
  })
  const statisticsProblems = statisticsQuery.data?.problems ?? []
  const queryFailures = [
    contestsError
      ? { label: '比赛列表', cause: contestsErrorCause, retry: refetchContests }
      : null,
    problemsError
      ? { label: '试题列表', cause: problemsErrorCause, retry: refetchProblems }
      : null,
    usersError
      ? { label: '用户列表', cause: usersErrorCause, retry: refetchUsers }
      : null,
    groupsError
      ? { label: '分组列表', cause: groupsErrorCause, retry: refetchGroups }
      : null,
    submissionsError
      ? { label: '提交记录', cause: submissionsErrorCause, retry: refetchSubmissions }
      : null,
    ranksError
      ? { label: '排行榜', cause: ranksErrorCause, retry: refetchRanks }
      : null,
  ].filter((item): item is NonNullable<typeof item> => item !== null)

  // SSE real-time listener
  useSSE(cid, (type) => {
    if (type === 'stream.overflow') {
      void refetchSubmissions()
      void refetchRanks()
      void refetchContests()
      void refetchProblems()
      void refetchUsers()
      void refetchGroups()
      if (activeTab === 'analytics') void statisticsQuery.refetch()
      toast.warning('实时事件积压', '已从服务器重新同步教师工作台数据。')
      return
    }
    if (type === 'submission.judged' || type === 'ranking.updated' || type === 'contest.updated') {
      refetchSubmissions()
      refetchRanks()
      refetchContests()
      if (activeTab === 'analytics') void statisticsQuery.refetch()
    }
    if (
      type === 'code.saved' ||
      type === 'submission.created' ||
      type === 'submission.judged'
    ) {
      window.dispatchEvent(new CustomEvent('forge:monitor-refresh'))
    }
    if (
      type === 'problem.created' ||
      type === 'problem.updated' ||
      type === 'problem.deleted' ||
      type === 'problems.reordered' ||
      type === 'contest.updated'
    ) {
      void refetchProblems()
    }
  })

  // Contest Operations
  const handleCreateContest = async () => {
    if (!newContestForm.name.trim()) {
      toast.error('请填写比赛名称')
      return
    }
    setPending('create-contest')
    try {
      const created = await api.createContest(
        newContestForm.name.trim(),
        newContestForm.description.trim(),
        newContestForm.rankingMode,
        [],
        newContestForm.mode
      )
      setNewContestForm({ name: '', description: '', rankingMode: 'oi', mode: 'online' })
      setShowCreateContest(false)
      setCid(created.id)
      await refetchContests()
      toast.success('比赛已创建', `${created.name} 已进入草稿状态。`)
    } catch (cause) {
      toast.error('创建比赛失败', errorText(cause))
    } finally {
      setPending(null)
    }
  }

  const handleSelectBundle = async (file: File | undefined) => {
    if (!file) return
    setBundleName(file.name)
    setBundleError('')
    setBundlePreview(null)
    setBundleRaw(null)
    try {
      if (file.size > 4 * 1024 * 1024) {
        throw new Error('比赛包超过 4 MB，拒绝载入。')
      }
      const parsed = JSON.parse(await file.text()) as Record<string, unknown>
      const contest = parsed?.contest as Record<string, unknown> | undefined
      const problems = parsed?.problems
      if (!contest || !Array.isArray(problems)) {
        throw new Error('比赛包结构无效：必须包含 contest 与 problems 数组。')
      }
      setBundleRaw(parsed)
      setBundlePreview({
        name: typeof contest.name === 'string' ? contest.name : '未命名比赛',
        description:
          typeof contest.description === 'string' ? contest.description : '',
        problems: problems.length,
      })
    } catch (cause) {
      setBundleError(cause instanceof Error ? cause.message : '比赛包解析失败。')
    }
  }

  const handleImportBundle = async () => {
    if (!bundleRaw) {
      setBundleError('请先选择并解析比赛包。')
      return
    }
    setBundleBusy(true)
    setBundleError('')
    try {
      const imported = await api.importContestBundle(bundleRaw)
      await refetchContests()
      setCid(imported.id)
      setShowImportBundle(false)
      setBundleRaw(null)
      setBundlePreview(null)
      setBundleName('')
      toast.success('比赛包已导入', `${imported.name} 已作为新草稿创建。`)
    } catch (cause) {
      setBundleError(errorText(cause))
    } finally {
      setBundleBusy(false)
    }
  }

  const handleStartContest = async () => {
    if (!cid) return
    setPending('start-contest')
    try {
      await api.startContest(cid)
      await refetchContests()
      toast.success('比赛已启动', '选手现在可以进入考场作答。')
    } catch (cause) {
      toast.error('启动失败', errorText(cause))
    } finally {
      setPending(null)
    }
  }

  const handleFinishContest = async () => {
    if (!cid) return
    setPending('finish-contest')
    try {
      await api.finishContest(cid)
      await refetchContests()
      toast.success('比赛已结束', '提交入口已关闭，榜单已冻结。')
    } catch (cause) {
      toast.error('结束比赛失败', errorText(cause))
    } finally {
      setPending(null)
    }
  }

  // Problem Reordering
  const handleMoveProblem = async (index: number, direction: 'up' | 'down') => {
    if (direction === 'up' && index === 0) return
    if (direction === 'down' && index === problems.length - 1) return

    const newOrder = [...problems]
    const targetIdx = direction === 'up' ? index - 1 : index + 1
    const [moved] = newOrder.splice(index, 1)
    newOrder.splice(targetIdx, 0, moved)

    const ids = newOrder.map((p) => p.id)
    try {
      await api.reorderProblems(cid, ids)
      await refetchProblems()
    } catch (cause) {
      toast.error('调整题序失败', errorText(cause))
    }
  }

  // Save Problem (Create or Update)
  const handleSaveProblem = async () => {
    if (!editingProblem) return
    if (!editingProblem.title?.trim()) {
      toast.error('题目标题不能为空')
      return
    }
    if (editingProblem.taskType === 'choice') {
      const validation = choicePaperError(editingProblem.choiceQuestions || [])
      if (validation) {
        toast.error('客观题试卷不完整', validation)
        return
      }
    }
    if (
      editingProblem.taskType === 'interaction' &&
      !editingProblem.interactorCode?.trim()
    ) {
      toast.error('交互题缺少交互器', '请填写 Interactor 源码后再保存。')
      return
    }
    if (
      (editingProblem.compareMode === 'special_judge' ||
        editingProblem.compareMode === 'custom_checker') &&
      !editingProblem.checkerCode?.trim()
    ) {
      toast.error('特判题缺少检查器', '请填写 Checker / SPJ 源码后再保存。')
      return
    }
    const payload = { ...editingProblem }
    if (payload.taskType === 'choice') {
      if (!payload.input) payload.input = '答题卡 JSON'
      if (!payload.output) payload.output = '标准比对得分'
      if (!payload.constraints) payload.constraints = '客观选择与判断题'
    } else {
      // Do not leave a previously authored objective key on a traditional
      // problem after the teacher changes the task type.
		payload.choiceQuestions = []
		payload.choiceKeyJson = ''
    }
    setPending('save-problem')
    try {
      if (payload.id) {
        await api.updateProblem(cid, payload.id, payload)
      } else {
        await api.createProblem(cid, payload)
      }
      setEditingProblem(null)
      await refetchProblems()
      toast.success('题面已保存')
    } catch (cause) {
      toast.error('保存题面失败', errorText(cause))
    } finally {
      setPending(null)
    }
  }

  const handleValidateProblem = async (pid: string) => {
    try {
      const res = await api.validateProblem(cid, pid)
      if (res.valid) {
        setProblemValidationMsg({ ok: true, msg: '试题结构完整，符合 CCF CSP 命题规范！' })
      } else {
        setProblemValidationMsg({ ok: false, msg: `校验未通过: ${res.error}` })
      }
    } catch (e: unknown) {
      setProblemValidationMsg({ ok: false, msg: errorText(e) })
    }
  }

  // User & Group Operations
  const handleCreateUser = async () => {
    if (!newUserForm.name.trim()) {
      toast.error('请填写姓名')
      return
    }
    if (newUserForm.password.length < 10) {
      toast.error('初始密码过短', '商业版要求密码至少 10 位。建议使用“随机生成”。')
      return
    }
    try {
      const created = await api.createUser(
        newUserForm.name.trim(),
        newUserForm.username.trim(),
        newUserForm.role as 'student' | 'teacher',
        newUserForm.password,
        newUserForm.province
      )
      setNewUserForm({ name: '', username: '', role: 'student', password: '', province: newUserForm.province })
      await refetchUsers()
      toast.success('账号已录入', `登录账号：${created.username}${created.province ? ' · 省份：' + provinceName(created.province) : ''}`)
    } catch (cause) {
      toast.error('录入失败', errorText(cause))
    }
  }

  const handleOpenEditUser = (u: User) => {
    setEditingUser(u)
    setEditUserForm({ name: u.name, username: u.username, role: u.role, province: u.province || '' })
  }

  const handleSaveEditUser = async () => {
    if (!editingUser) return
    if (!editUserForm.name.trim()) {
      toast.error('请填写姓名')
      return
    }
    if (!editUserForm.username.trim()) {
      toast.error('请填写登录名')
      return
    }
    setEditUserBusy(true)
    try {
      await api.updateUser(editingUser.id, {
        name: editUserForm.name.trim(),
        username: editUserForm.username.trim(),
        province: editUserForm.province,
        ...(isAdmin && editUserForm.role !== editingUser.role ? { role: editUserForm.role } : {}),
      })
      await Promise.all([refetchUsers(), refetchProvinces()])
      toast.success('账号已更新', `${editUserForm.name.trim()} · ${editUserForm.username.trim()}`)
      setEditingUser(null)
    } catch (cause) {
      toast.error('更新账号失败', errorText(cause))
    } finally {
      setEditUserBusy(false)
    }
  }

  const handleBatchSetProvince = async () => {
    if (selectedUserIds.length === 0) {
      toast.error('请先在名册中勾选学生')
      return
    }
    if (!batchProvinceCode) {
      toast.error('请选择目标省份')
      return
    }
    setBatchProvinceBusy(true)
    try {
      const res = await api.batchSetProvince(selectedUserIds, batchProvinceCode)
      await refetchUsers()
      toast.success('批量省份已更新', `本次更新 ${res.updated} 名学生`)
      setSelectedUserIds([])
      setShowBatchProvince(false)
    } catch (cause) {
      toast.error('批量设置省份失败', errorText(cause))
    } finally {
      setBatchProvinceBusy(false)
    }
  }

  const toggleUserSelected = (uid: string) => {
    setSelectedUserIds((current) =>
      current.includes(uid) ? current.filter((id) => id !== uid) : [...current, uid]
    )
  }

  const handleGenerateCSPTickets = async () => {
    if (!cspProvince) {
      toast.error('请选择省份', 'CSP 准考证号以省份编号开头，如 HA-J00001。')
      return
    }
    if (examUserPassword && examUserPassword.length < 10) {
      toast.error('统一初始密码过短', '至少 10 位；留空则每个账号生成独立随机密码。')
      return
    }
    setIsGeneratingCSP(true)
    try {
      const res = await api.generateCSPTickets({
        province: cspProvince,
        group: cspGroup,
        numberLength: cspDigits,
        ...(selectedUserIds.length > 0
          ? { userIds: selectedUserIds }
          : { createCount: Math.max(1, Math.min(500, cspCount || 1)) }),
        ...(examUserPassword ? { password: examUserPassword } : {}),
      })
      const credentials = res.assignments.map((a) => ({
        username: a.user.username || a.user.id,
        password: examUserPassword || generateInitialPassword(),
        name: a.user.name,
        ticket: a.ticket,
      }))
      await Promise.all([refetchUsers(), refetchProvinces()])
      setGeneratedCredentials(credentials)
      setSelectedUserIds([])
      toast.success(
        `已发放 ${credentials.length} 个准考证号`,
        `号段 ${credentials[0]?.ticket ?? ''} ~ ${credentials[credentials.length - 1]?.ticket ?? ''}，可在座位卡中打印。`
      )
    } catch (cause) {
      toast.error('准考证号发放失败', errorText(cause))
    } finally {
      setIsGeneratingCSP(false)
    }
  }

  const handleCreateGroup = async () => {
    if (!newGroupForm.name.trim()) return
    try {
      await api.createGroup(newGroupForm.name.trim())
      setNewGroupForm({ name: '' })
      await refetchGroups()
      toast.success('组别已创建')
    } catch (cause) {
      toast.error('创建组别失败', errorText(cause))
    }
  }

  const handleBindGroupToContest = async (gid: string) => {
    if (!cid) return
    try {
      await api.bindGroupToContest(cid, gid)
      await refetchContests()
      toast.success('组别已授权到当前比赛')
    } catch (cause) {
      toast.error('授权失败', errorText(cause))
    }
  }

  const handleBindParticipant = async (uid: string) => {
    if (!cid) return
    setPending(`participant:${uid}`)
    try {
      await api.bindParticipantToContest(cid, uid)
      await refetchContests()
      toast.success('学生已加入本场个人授权名单')
    } catch (cause) {
      toast.error('个人授权失败', errorText(cause))
    } finally {
      setPending(null)
    }
  }

  const handleAddGroupMember = async (gid: string) => {
    const uid = groupMemberDraft[gid]
    if (!uid) {
      toast.error('请选择要加入组别的学生')
      return
    }
    setPending(`group-add:${gid}`)
    try {
      await api.addGroupMember(gid, uid)
      setGroupMemberDraft((current) => ({ ...current, [gid]: '' }))
      await Promise.all([refetchGroups(), refetchUsers()])
      toast.success('学生已加入组别')
    } catch (cause) {
      toast.error('加入组别失败', errorText(cause))
    } finally {
      setPending(null)
    }
  }

  const handleRemoveGroupMember = async (gid: string, uid: string) => {
    setPending(`group-remove:${gid}:${uid}`)
    try {
      await api.removeGroupMember(gid, uid)
      await Promise.all([refetchGroups(), refetchUsers()])
      toast.success('已从组别移除学生')
    } catch (cause) {
      toast.error('移除失败', errorText(cause))
    } finally {
      setPending(null)
    }
  }

  const handleImportCsv = async () => {
    if (!csvContent.trim()) return
    try {
      const res = await api.importUsersCsv(csvContent)
      setCsvContent('')
      setShowImportCsv(false)
      await refetchUsers()
      const failed = Array.isArray(res.errors) ? res.errors : []
      if (failed.length) {
        toast.notify(
          'warning',
          `已导入 ${res.created} 名学生`,
          `${failed.length} 行未导入：${failed.slice(0, 3).join('；')}`
        )
      } else {
        toast.success(`已导入 ${res.created} 名学生`)
      }
    } catch (cause) {
      toast.error('CSV 导入失败', errorText(cause))
    }
  }

  const handleBatchGenerateExamUsers = async () => {
    if (!examUserPrefix.trim()) {
      toast.error('缺少准考证前缀', '请填写学号或准考证号前缀。')
      return
    }
    if (examUserPassword && examUserPassword.length < 10) {
      toast.error('初始密码过短', '统一密码至少 10 位；留空则每个账号生成独立随机密码。')
      return
    }
    if (examUserCount < 1 || examUserCount > 500) {
      toast.error('人数超出范围', '单次批量生成支持 1 至 500 人。')
      return
    }
    setIsGeneratingUsers(true)
    const credentials: Array<{ username: string; password: string; name: string }> = []
    try {
      for (let i = 1; i <= examUserCount; i++) {
        const num = String(i).padStart(3, '0')
        const uname = `${examUserPrefix}${num}`
        const password = examUserPassword || generateInitialPassword()
        const created = await api.createUser(
          `考生${num}`,
          uname,
          'student',
          password
        )
        // The server appends a suffix when an account already exists. The
        // printable seat card must show the actual login, not the requested one.
        credentials.push({
          username: created.username,
          password,
          name: created.name,
        })
      }
      await refetchUsers()
      setGeneratedCredentials(credentials)
      setShowSeatCardModal(true)
      toast.success(`已生成 ${credentials.length} 个准考证账号`, '请在座位卡中打印或导出凭证。')
    } catch (e: unknown) {
      setGeneratedCredentials(credentials)
      toast.error(
        '批量生成中断',
        `${errorText(e)}${credentials.length ? ` 已成功创建 ${credentials.length} 个账号。` : ''}`
      )
    } finally {
      setIsGeneratingUsers(false)
    }
  }

  const submissionDraft = (submission: (typeof submissions)[number]) =>
    judgeDrafts[submission.id] || {
      verdict: submission.verdict === 'queued' ? 'accepted' : submission.verdict,
      score: submission.score,
    }

  const handleJudgeSubmission = async (submission: (typeof submissions)[number]) => {
    const draft = submissionDraft(submission)
    if (!draft.verdict.trim()) {
      toast.error('请选择判题结论')
      return
    }
    if (!Number.isFinite(draft.score) || draft.score < 0) {
      toast.error('分数不能为负数')
      return
    }
    setJudgeBusy(submission.id)
    try {
      await api.judge(submission.id, draft.verdict, draft.score)
      await Promise.all([refetchSubmissions(), refetchRanks()])
      setJudgeDrafts((current) => {
        const next = { ...current }
        delete next[submission.id]
        return next
      })
      toast.success('判定已保存', `${submission.userName || submission.userId} · ${draft.score} 分`)
    } catch (cause) {
      toast.error('保存判定失败', errorText(cause))
    } finally {
      setJudgeBusy('')
    }
  }

  // Code Diff Versions for selected student & problem

  // 做题演进升级为服务端代码版本仓：保存/提交都是版本（seq 连续、内容完整）。
  const versionsQuery = useQuery({
    queryKey: ['code-versions', cid, selectedStudentId, selectedProblemId],
    queryFn: () => api.codeVersions(cid, selectedProblemId, selectedStudentId),
    enabled: Boolean(cid) && Boolean(selectedStudentId) && Boolean(selectedProblemId),
  })
  const evolutionVersions = versionsQuery.data ?? []
  const versionLabel = (v: (typeof evolutionVersions)[number]) =>
    `v${v.seq}${v.source === 'submit' ? ' · 提交' : ' · 保存'}`

  // Detect duplicate student names to highlight them
  const duplicateNames = React.useMemo(() => {
    const counts: Record<string, number> = {}
    users.forEach((u) => {
      counts[u.name] = (counts[u.name] || 0) + 1
    })
    return new Set(Object.keys(counts).filter((name) => counts[name] > 1))
  }, [users])

  return (
    <div className="space-y-6">
      <section className="relative overflow-hidden rounded-[1.75rem] border border-[#e4e7ec] bg-[#fffefa] px-5 py-6 shadow-[0_20px_50px_-36px_rgba(20,40,61,0.35)] sm:px-7">
        <div className="pointer-events-none absolute inset-y-0 right-0 w-1/2 forge-grid opacity-40" />
        <div className="relative flex flex-col gap-5 xl:flex-row xl:items-end xl:justify-between">
          <div className="min-w-0">
            <p className="app-eyebrow">Instructor command room</p>
            <div className="mt-2 flex flex-wrap items-center gap-2.5">
              <h1 className="font-display text-3xl font-semibold tracking-[-0.04em] text-[#14283d] sm:text-[2.35rem]">
                教师工作台
              </h1>
              {currentContest && (
                <Badge
                  variant={
                    currentContest.status === 'running'
                      ? 'success'
                      : currentContest.status === 'finished'
                        ? 'default'
                        : 'warning'
                  }
                  dot
                >
                  {currentContest.status === 'running'
                    ? '进行中'
                    : currentContest.status === 'finished'
                      ? '已结束'
                      : '草稿'}
                </Badge>
              )}
            </div>
            <p className="mt-2 max-w-2xl text-sm leading-6 text-slate-500">
              编排试题、管理考场身份，并在同一条时间线上完成评测复核与安全调度。
            </p>
          </div>

          <div className="flex flex-wrap items-center gap-2">
            {currentContest?.status === 'draft' && (
              <Button
                variant="success"
                onClick={() => void handleStartContest()}
                loading={pending === 'start-contest'}
              >
                <Play className="h-4 w-4" /> 启动比赛
              </Button>
            )}
            {currentContest?.status === 'running' && (
              <Button
                variant="danger"
                onClick={() => void handleFinishContest()}
                loading={pending === 'finish-contest'}
              >
                <Square className="h-4 w-4" /> 结束比赛
              </Button>
            )}
            <Button variant="outline" onClick={() => setShowImportBundle(true)}>
              <Upload className="h-4 w-4" /> 导入比赛包
            </Button>
            <Button onClick={() => setShowCreateContest(true)}>
              <Plus className="h-4 w-4" /> 新建比赛
            </Button>
          </div>
        </div>

        <div className="relative mt-6 flex flex-col gap-3 border-t border-[#eaecf0] pt-5 lg:flex-row lg:items-center lg:justify-between">
          <div className="flex min-w-0 flex-1 flex-col gap-2 sm:flex-row sm:items-center">
            <label htmlFor="teacher-contest" className="shrink-0 text-xs font-bold text-slate-500">
              当前比赛
            </label>
            <Select
              id="teacher-contest"
              value={cid}
              onChange={(event) => setCid(event.target.value)}
              disabled={!contests.length}
              className="h-9 max-w-xl font-semibold"
            >
              {!contests.length && <option value="">暂无比赛</option>}
              {contests.map((contest) => (
                <option key={contest.id} value={contest.id}>
                  {contest.name} ·{' '}
                  {contest.status === 'running'
                    ? '进行中'
                    : contest.status === 'finished'
                      ? '已结束'
                      : '草稿'}
                </option>
              ))}
            </Select>
          </div>

          {cid && (
            <div className="flex flex-wrap items-center gap-2">
              <Button
                size="sm"
                variant="outline"
                onClick={() => {
                  void api.downloadContestHtml(cid).catch((cause) => {
                    toast.error('全卷预览下载失败', errorText(cause))
                  })
                }}
              >
                <Printer className="h-3.5 w-3.5" /> CCF 全卷
              </Button>
              <Button
                size="sm"
                variant="secondary"
                onClick={() => {
                  void api.downloadContestPdf(cid).catch((cause) => {
                    toast.error('PDF 导出失败', errorText(cause))
                  })
                }}
              >
                <FileDown className="h-3.5 w-3.5" /> 全卷 PDF
              </Button>
              <Button
                size="sm"
                variant="ghost"
                onClick={() => {
                  void api.downloadBundle(cid).catch((cause) => {
                    toast.error('比赛包导出失败', errorText(cause))
                  })
                }}
              >
                <Download className="h-3.5 w-3.5" /> 比赛包
              </Button>
            </div>
          )}
        </div>
      </section>

      {queryFailures.length > 0 && (
        <div className="space-y-2">
          {queryFailures.map((failure) => (
            <InlineError
              key={failure.label}
              title={`${failure.label}加载失败`}
              message={errorText(failure.cause)}
              action={
                <Button
                  size="sm"
                  variant="outline"
                  onClick={() => {
                    void failure.retry()
                  }}
                >
                  重试
                </Button>
              }
            />
          ))}
        </div>
      )}

      <Dialog
        open={showCreateContest}
        onClose={() => setShowCreateContest(false)}
        title="创建新比赛"
        eyebrow="New contest"
        footer={
          <>
            <Button variant="ghost" onClick={() => setShowCreateContest(false)}>
              取消
            </Button>
            <Button
              onClick={() => void handleCreateContest()}
              loading={pending === 'create-contest'}
            >
              保存并继续配置
            </Button>
          </>
        }
      >
        <div className="space-y-4">
          <label className="block space-y-1.5">
            <span className="text-xs font-bold text-slate-700">比赛名称</span>
            <Input
              autoFocus
              placeholder="如：2026 CSP-J/S 模拟复赛"
              value={newContestForm.name}
              onChange={(event) =>
                setNewContestForm({ ...newContestForm, name: event.target.value })
              }
            />
          </label>
          <label className="block space-y-1.5">
            <span className="text-xs font-bold text-slate-700">说明</span>
            <Textarea
              rows={3}
              placeholder="面向谁、考试范围或机房注意事项"
              value={newContestForm.description}
              onChange={(event) =>
                setNewContestForm({ ...newContestForm, description: event.target.value })
              }
            />
          </label>
          <label className="block space-y-1.5">
            <span className="text-xs font-bold text-slate-700">榜单赛制</span>
            <Select
              value={newContestForm.rankingMode}
              onChange={(event) =>
                setNewContestForm({
                  ...newContestForm,
                  rankingMode: event.target.value as RankingMode,
                })
              }
            >
              <option value="oi">OI · 按题得分累计</option>
              <option value="acm">ACM · 通过题数与罚时</option>
            </Select>
          </label>
          <label className="block space-y-1.5">
            <span className="text-xs font-bold text-slate-700">比赛方式</span>
            <Select
              value={newContestForm.mode}
              onChange={(event) =>
                setNewContestForm({
                  ...newContestForm,
                  mode: event.target.value as ContestMode,
                })
              }
            >
              <option value="online">在线提交 · 边赛边评实时反馈</option>
              <option value="noi">NOI 全真模拟 · 赛中屏蔽反馈，赛后统一公布</option>
              <option value="gesp">GESP 机考 · 客观题交卷即判</option>
            </Select>
          </label>
          <p className="rounded-xl bg-slate-50 px-4 py-3 text-[11px] leading-5 text-slate-500">
            新比赛以草稿创建。添加题目与考生后，再从顶部启动比赛。
          </p>
        </div>
      </Dialog>

      <Dialog
        open={showImportBundle}
        onClose={() => {
          if (!bundleBusy) setShowImportBundle(false)
        }}
        title="导入 Forge 比赛包"
        eyebrow="Bundle import"
        width="md"
        footer={
          <>
            <Button
              variant="ghost"
              disabled={bundleBusy}
              onClick={() => setShowImportBundle(false)}
            >
              取消
            </Button>
            <Button
              onClick={() => void handleImportBundle()}
              loading={bundleBusy}
              disabled={!bundleRaw}
            >
              导入为新草稿
            </Button>
          </>
        }
      >
        <div className="space-y-4">
          <input
            ref={bundleInputRef}
            type="file"
            accept="application/json,.json"
            aria-label="选择比赛包文件"
            className="sr-only"
            onChange={(event) => {
              void handleSelectBundle(event.target.files?.[0])
              event.target.value = ''
            }}
          />
          <button
            type="button"
            onClick={() => bundleInputRef.current?.click()}
            className="flex w-full flex-col items-center rounded-2xl border border-dashed border-[#cfd6df] bg-[#fbfaf6] px-5 py-8 text-center transition hover:border-forge-300 hover:bg-forge-50/40"
          >
            <FolderArchive className="h-8 w-8 text-forge-600" />
            <span className="mt-2 text-sm font-extrabold text-[#14283d]">
              {bundleName || '选择 .json 比赛包'}
            </span>
            <span className="mt-1 text-[11px] text-slate-500">
              导入会创建全新比赛和题目 ID，不覆盖当前数据；上限 4 MB。
            </span>
          </button>

          {bundlePreview && (
            <div className="rounded-xl border border-emerald-200 bg-emerald-50/70 p-4">
              <div className="flex flex-wrap items-center justify-between gap-2">
                <p className="text-sm font-extrabold text-emerald-950">
                  {bundlePreview.name}
                </p>
                <Badge variant="success">{bundlePreview.problems} 道题</Badge>
              </div>
              <p className="mt-1 line-clamp-2 text-[11px] leading-5 text-emerald-900/70">
                {bundlePreview.description || '比赛包未附说明。'}
              </p>
            </div>
          )}

          {bundleError && (
            <p
              role="alert"
              className="rounded-xl border border-rose-200 bg-rose-50 px-3 py-2 text-xs leading-5 text-rose-800"
            >
              {bundleError}
            </p>
          )}
        </div>
      </Dialog>

      <nav
        role="tablist"
        aria-label="教师工作台分区"
        className="scrollbar-slim flex gap-1 overflow-x-auto rounded-2xl border border-[#e4e7ec] bg-white/75 p-1.5 shadow-[0_1px_2px_rgba(16,24,40,0.03)]"
      >
        {TAB_ITEMS.map(({ key, label, icon: Icon }, index) => {
          const selected = activeTab === key
          return (
            <button
              key={key}
              type="button"
              role="tab"
              id={`teacher-tab-${key}`}
              aria-selected={selected}
              aria-controls={`teacher-panel-${key}`}
              tabIndex={selected ? 0 : -1}
              onClick={() => setActiveTab(key)}
              onKeyDown={(event) => handleTabKeyDown(event, index)}
              className={
                selected
                  ? 'flex h-10 shrink-0 items-center gap-2 rounded-xl bg-[#14283d] px-4 text-xs font-bold text-white shadow-[0_8px_20px_-12px_rgba(20,40,61,0.9)]'
                  : 'flex h-10 shrink-0 items-center gap-2 rounded-xl px-4 text-xs font-bold text-slate-500 transition hover:bg-slate-100 hover:text-slate-900'
              }
            >
              <Icon className={selected ? 'h-4 w-4 text-forge-200' : 'h-4 w-4'} />
              {label}
            </button>
          )
        })}
      </nav>

      {/* TAB: 考务安全与机房调度 (IP强绑定/时间窗口/加时/广播/Zip) */}
      {activeTab === 'security' && currentContest && (
        <div
          role="tabpanel"
          id="teacher-panel-security"
          aria-labelledby="teacher-tab-security"
        >
          <TeacherSecurityPanel
            contest={currentContest}
            users={users}
            refetchContest={refetchContests}
          />
        </div>
      )}

      {/* TAB: 监考大屏 */}
      {activeTab === 'monitor' && (
        <div
          role="tabpanel"
          id="teacher-panel-monitor"
          aria-labelledby="teacher-tab-monitor"
          className="rounded-[1.75rem] bg-[#0b1520] p-5"
        >
          <MonitorBoard cid={cid} />
        </div>
      )}

      {/* TAB 1: 比赛概览与设置 */}
      {activeTab === 'contests' && (
        <div
          role="tabpanel"
          id="teacher-panel-contests"
          aria-labelledby="teacher-tab-contests"
          className="grid gap-5 md:grid-cols-2"
        >
          <Card>
            <CardHeader>
              <CardTitle className="text-base flex items-center justify-between">
                <span>比赛信息配置</span>
                <Badge variant={currentContest?.status === 'running' ? 'success' : 'default'}>
                  {currentContest?.status}
                </Badge>
              </CardTitle>
            </CardHeader>
            <CardContent className="space-y-4 text-sm">
              <div>
                <span className="text-slate-400 text-xs block mb-1">比赛名称:</span>
                <p className="font-semibold text-slate-800">{currentContest?.name}</p>
              </div>
              <div>
                <span className="text-slate-400 text-xs block mb-1">说明与要求:</span>
                <p className="text-slate-600 bg-slate-50 p-3 rounded-lg border border-slate-100">
                  {currentContest?.description || '暂无详细描述'}
                </p>
              </div>
              <div className="grid grid-cols-2 gap-3 pt-2">
                <div>
                  <span className="text-slate-400 text-xs block mb-1">榜单赛制:</span>
                  <Select
                    aria-label="榜单赛制"
                    value={currentContest?.rankingMode || 'oi'}
                    disabled={!paperEditable}
                    title={paperEditable ? undefined : '比赛启动后赛制已冻结'}
                    onChange={async (event) => {
                      try {
                        await api.updateContestSettings(cid, {
                          rankingMode: event.target.value as RankingMode,
                        })
                        await refetchContests()
                        toast.success('榜单赛制已更新')
                      } catch (cause) {
                        toast.error('更新赛制失败', errorText(cause))
                      }
                    }}
                    className="h-9 text-xs"
                  >
                    <option value="oi">OI · 总分排行榜</option>
                    <option value="acm">ACM · 题数与罚时</option>
                  </Select>
                </div>
                <div>
                  <span className="text-slate-400 text-xs block mb-1">试题数量:</span>
                  <p className="font-bold text-slate-800">{problems.length} 道题目</p>
                </div>
              </div>
              <div className="rounded-xl border border-slate-200 bg-slate-50 p-3">
                <div className="flex flex-wrap items-center justify-between gap-2">
                  <div>
                    <p className="text-xs font-bold text-slate-800">允许提交语言</p>
                    <p className="mt-0.5 text-[10px] text-slate-500">
                      空选表示不限制；比赛启动后白名单冻结。
                    </p>
                  </div>
                  <Button
                    size="sm"
                    variant="outline"
                    disabled={!paperEditable}
                    loading={pending === 'save-languages'}
                    onClick={async () => {
                      if (!cid) return
                      setPending('save-languages')
                      try {
                        await api.updateContestSettings(cid, { allowedLanguages: languageDraft })
                        await refetchContests()
                        toast.success('语言白名单已保存')
                      } catch (cause) {
                        toast.error('保存语言白名单失败', errorText(cause))
                      } finally {
                        setPending(null)
                      }
                    }}
                    className="text-xs"
                  >
                    保存语言设置
                  </Button>
                </div>
                <div className="mt-2 flex flex-wrap gap-3">
                  {[
                    ['cpp', 'C++'],
                    ['c', 'C'],
                    ['python', 'Python'],
                    ['go', 'Go'],
                  ].map(([value, label]) => (
                    <label key={value} className="inline-flex items-center gap-1.5 text-xs text-slate-600">
                      <input
                        type="checkbox"
                        className="h-3.5 w-3.5 rounded border-slate-300"
                        checked={languageDraft.includes(value)}
                        disabled={!paperEditable}
                        onChange={(event) =>
                          setLanguageDraft((current) =>
                            event.target.checked
                              ? [...current, value].filter((item, index, list) => list.indexOf(item) === index)
                              : current.filter((item) => item !== value)
                          )
                        }
                      />
                      {label}
                    </label>
                  ))}
                </div>
              </div>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle className="text-base">实时榜单预览</CardTitle>
            </CardHeader>
            <CardContent>
              <Scoreboard ranks={ranks} rankingMode={currentContest?.rankingMode} />
            </CardContent>
          </Card>
        </div>
      )}

      {/* TAB 2: 全局用户与分组管理 */}
      {activeTab === 'users' && (
        <div
          role="tabpanel"
          id="teacher-panel-users"
          aria-labelledby="teacher-tab-users"
          className="space-y-5"
        >
          {/* Group Management Section */}
          <div className="grid gap-5 md:grid-cols-3">
            <Card className="md:col-span-1">
              <CardHeader>
                <CardTitle className="text-sm">新建全局班级 / 组别</CardTitle>
              </CardHeader>
              <CardContent className="space-y-3">
                <Input
                  placeholder="如: 高一竞赛A班、CSP强化组"
                  value={newGroupForm.name}
                  onChange={(e) => setNewGroupForm({ name: e.target.value })}
                />
                <Button size="sm" onClick={handleCreateGroup} className="w-full">
                  添加组别
                </Button>

                <div className="pt-3 border-t border-slate-100 space-y-2">
                  <span className="text-xs font-bold text-slate-400 uppercase tracking-wider block">
                    已创建组别 ({groups.length})
                  </span>
                  <div className="space-y-2 max-h-56 overflow-y-auto pr-1">
                    {groups.map((g) => (
                      <div
                        key={g.id}
                        className="rounded-lg border border-slate-200 bg-slate-50 p-2.5 text-xs"
                      >
                        <div className="flex items-start justify-between gap-2">
                          <div>
                            <p className="font-semibold text-slate-800">{g.name}</p>
                            <span className="text-slate-400 text-[10px]">
                              {g.userIds?.length || 0} 名学生
                            </span>
                          </div>
                          <Button
                            size="sm"
                            variant="outline"
                            disabled={!paperEditable || currentContest?.groupIds?.includes(g.id)}
                            onClick={() => handleBindGroupToContest(g.id)}
                            className="text-[10px] h-6 px-2 text-blue-600"
                          >
                            {currentContest?.groupIds?.includes(g.id) ? '已授权' : '授权本场'}
                          </Button>
                        </div>
                        <div className="mt-2 space-y-1">
                          {(g.userIds || []).map((uid) => {
                            const member = users.find((user) => user.id === uid)
                            return (
                              <div key={uid} className="flex items-center justify-between rounded bg-white px-2 py-1">
                                <span className="truncate text-[10px] text-slate-600">
                                  {member ? `${member.name} · ${member.username}` : uid}
                                </span>
                                <button
                                  type="button"
                                  aria-label={`从 ${g.name} 移除 ${member?.name || uid}`}
                                  disabled={pending === `group-remove:${g.id}:${uid}`}
                                  onClick={() => void handleRemoveGroupMember(g.id, uid)}
                                  className="ml-2 text-slate-400 hover:text-rose-600 disabled:opacity-40"
                                >
                                  <XCircle className="h-3.5 w-3.5" />
                                </button>
                              </div>
                            )
                          })}
                        </div>
                        <div className="mt-2 flex gap-1.5">
                          <select
                            aria-label={`添加学生到 ${g.name}`}
                            value={groupMemberDraft[g.id] || ''}
                            onChange={(event) =>
                              setGroupMemberDraft((current) => ({ ...current, [g.id]: event.target.value }))
                            }
                            className="h-7 min-w-0 flex-1 rounded border border-slate-300 bg-white px-1.5 text-[10px]"
                          >
                            <option value="">选择未加入的学生…</option>
                            {users
                              .filter(
                                (user) =>
                                  user.role === 'student' && !(g.userIds || []).includes(user.id)
                              )
                              .map((user) => (
                                <option key={user.id} value={user.id}>
                                  {user.name} · {user.username}
                                </option>
                              ))}
                          </select>
                          <Button
                            size="sm"
                            variant="secondary"
                            loading={pending === `group-add:${g.id}`}
                            disabled={!groupMemberDraft[g.id]}
                            onClick={() => void handleAddGroupMember(g.id)}
                            className="h-7 px-2 text-[10px]"
                          >
                            加入
                          </Button>
                        </div>
                      </div>
                    ))}
                  </div>
                </div>
              </CardContent>
            </Card>

            {/* Users List & Creation */}
            <Card className="md:col-span-2">
              <CardHeader className="flex flex-row items-center justify-between pb-3">
                <CardTitle className="text-sm">
                  全局学员名册 ({users.length} 人)
                </CardTitle>
                <div className="flex items-center gap-2">
                  <Button
                    size="sm"
                    variant="outline"
                    disabled={selectedUserIds.length === 0}
                    onClick={() => {
                      setBatchProvinceCode('')
                      setShowBatchProvince(true)
                    }}
                    title={selectedUserIds.length === 0 ? '先在名册中勾选学生' : `为选中的 ${selectedUserIds.length} 名学生设置省份`}
                    className="text-xs gap-1"
                  >
                    <MapPin className="w-3.5 h-3.5" /> 批量改省份 ({selectedUserIds.length})
                  </Button>
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() => setShowSeatCardModal(true)}
                    className="text-xs gap-1 border-amber-300 text-amber-800 bg-amber-50 hover:bg-amber-100"
                  >
                    <KeyRound className="w-3.5 h-3.5" /> 智能准考证生成
                  </Button>
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() => setShowImportCsv(!showImportCsv)}
                    className="text-xs gap-1"
                  >
                    <Upload className="w-3.5 h-3.5" /> CSV 批量导入
                  </Button>
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() => {
                      void api.downloadUsersCsv().catch((cause) => {
                        toast.error('名单导出失败', errorText(cause))
                      })
                    }}
                    className="gap-1 text-xs"
                  >
                    <Download className="w-3.5 h-3.5" /> 导出名单
                  </Button>
                </div>
              </CardHeader>
              <CardContent className="space-y-4">
                {/* Single User Add Row */}
                <div className="grid gap-2 sm:grid-cols-4 p-3 bg-slate-50 rounded-lg border border-slate-200">
                  <Input
                    placeholder="姓名 (如: 张伟)"
                    value={newUserForm.name}
                    onChange={(e) => setNewUserForm({ ...newUserForm, name: e.target.value })}
                  />
                  <Input
                    placeholder="学号/账号 (唯一识别码)"
                    value={newUserForm.username}
                    onChange={(e) => setNewUserForm({ ...newUserForm, username: e.target.value })}
                  />
                  <Input
                    type="password"
                    placeholder="初始密码（至少 10 位）"
                    value={newUserForm.password}
                    onChange={(e) => setNewUserForm({ ...newUserForm, password: e.target.value })}
                  />
                  <Select
                    aria-label="录入省份"
                    value={newUserForm.province}
                    onChange={(e) => setNewUserForm({ ...newUserForm, province: e.target.value })}
                    className="h-9 text-xs"
                    title="CSP 准考证号的省份编号来源"
                  >
                    <option value="">省份（可选）…</option>
                    {enabledProvinces.map((p) => (
                      <option key={p.code} value={p.code}>
                        {p.code} · {p.name}
                      </option>
                    ))}
                  </Select>
                  <div className="flex gap-2">
                    <Button
                      size="sm"
                      variant="outline"
                      type="button"
                      onClick={() => setNewUserForm({ ...newUserForm, password: generateInitialPassword() })}
                      className="shrink-0 text-xs"
                    >
                      随机
                    </Button>
                    <Button size="sm" onClick={handleCreateUser} className="flex-1">
                      录入学员
                    </Button>
                  </div>
                </div>

                {showImportCsv && (
                  <div className="p-3 bg-blue-50/50 rounded-lg border border-blue-200 space-y-2">
                    <span className="text-xs font-semibold text-blue-800">
                      粘贴 CSV 内容 (格式: name,role,group,password,username,province；省份可填编号或中文全名，如 HA 或 河南):
                    </span>
                    <textarea
                      rows={3}
                      value={csvContent}
                      onChange={(e) => setCsvContent(e.target.value)}
                      placeholder="张伟,student,高一1班,请使用至少10位密码&#10;李想,student,高一1班,可留空由系统生成"
                      className="w-full text-xs font-mono p-2 rounded border border-blue-300 bg-white"
                    />
                    <Button size="sm" onClick={handleImportCsv} className="text-xs">
                      执行导入
                    </Button>
                  </div>
                )}

                {/* Users Table */}
                <div className="overflow-x-auto rounded-lg border border-slate-200">
                  <table className="w-full text-left text-xs">
                    <thead className="bg-slate-50 font-semibold text-slate-600 border-b border-slate-200">
                      <tr>
                        <th className="px-3 py-2">
                          <input
                            type="checkbox"
                            aria-label="全选学生"
                            checked={selectedUserIds.length > 0 && selectedUserIds.length === users.filter((u2) => u2.role === 'student').length}
                            onChange={(event) =>
                              setSelectedUserIds(
                                event.target.checked
                                  ? users.filter((u2) => u2.role === 'student').map((u2) => u2.id)
                                  : []
                              )
                            }
                          />
                        </th>
                        <th className="px-3 py-2">姓名</th>
                        <th className="px-3 py-2">学号 / 唯一账号</th>
                        <th className="px-3 py-2">角色</th>
                        <th className="px-3 py-2">省份</th>
                        <th className="px-3 py-2">准考证号</th>
                        <th className="px-3 py-2">所属组别</th>
                        <th className="px-3 py-2 text-right">账号操作</th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-slate-100">
                      {users.map((u) => {
                        const isDup = duplicateNames.has(u.name)
                        return (
                          <tr key={u.id} className="hover:bg-slate-50">
                            <td className="px-3 py-2">
                              <input
                                type="checkbox"
                                aria-label={`选择 ${u.name}`}
                                disabled={u.role !== 'student'}
                                checked={selectedUserIds.includes(u.id)}
                                onChange={() => toggleUserSelected(u.id)}
                              />
                            </td>
                            <td className="px-3 py-2 font-medium">
                              <span className="text-slate-900 font-semibold">{u.name}</span>
                              {isDup && (
                                <span className="ml-2 inline-flex items-center gap-0.5 text-[10px] bg-amber-50 text-amber-700 px-1.5 py-0.5 rounded border border-amber-200">
                                  <ShieldAlert className="w-3 h-3 text-amber-500" />
                                  同名已区分: #{u.username}
                                </span>
                              )}
                            </td>
                            <td className="px-3 py-2 font-mono text-slate-500">
                              {u.username || u.id}
                            </td>
                            <td className="px-3 py-2 text-slate-600">{u.role}</td>
                            <td className="px-3 py-2 text-slate-600">
                              {u.province ? (
                                <span className="whitespace-nowrap">
                                  <span className="font-mono font-semibold">{u.province}</span>
                                  <span className="ml-1 text-slate-400">{provinceName(u.province)}</span>
                                </span>
                              ) : (
                                <span className="text-slate-500">—</span>
                              )}
                            </td>
                            <td className="px-3 py-2 font-mono text-blue-800">
                              {u.examTicket || <span className="text-slate-500">—</span>}
                            </td>
                            <td className="px-3 py-2 text-slate-500">
                              {u.groups && u.groups.length > 0 ? (
                                u.groups.map((gid) => {
                                  const grp = groups.find((g) => g.id === gid)
                                  return (
                                    <span
                                      key={gid}
                                      className="inline-block bg-slate-100 px-1.5 py-0.5 rounded text-[10px] mr-1"
                                    >
                                      {grp?.name || gid}
                                    </span>
                                  )
                                })
                              ) : (
                                <span className="text-slate-500">未分班</span>
                              )}
                            </td>
                            <td className="px-3 py-2 text-right">
                              <div className="flex items-center justify-end gap-1">
                                {u.role === 'student' && (
                                  <Button
                                    size="sm"
                                    variant="ghost"
                                    disabled={
                                      !paperEditable ||
                                      Boolean(currentContest?.participantUserIds?.includes(u.id))
                                    }
                                    loading={pending === `participant:${u.id}`}
                                    onClick={() => void handleBindParticipant(u.id)}
                                    className="h-7 px-2 text-[10px] text-sky-700"
                                  >
                                    {currentContest?.participantUserIds?.includes(u.id)
                                      ? '已个人授权'
                                      : '个人授权'}
                                  </Button>
                                )}
                                <Button
                                  size="sm"
                                  variant="ghost"
                                  onClick={() => handleOpenEditUser(u)}
                                  className="h-7 gap-1 px-2 text-[10px] text-indigo-700"
                                >
                                  <UserPen className="h-3 w-3" /> 编辑
                                </Button>
                                <Button
                                  size="sm"
                                  variant="ghost"
                                  onClick={() => {
                                    setResetUser(u)
                                    setResetPassword(generateInitialPassword())
                                  }}
                                  className="h-7 gap-1 px-2 text-[10px] text-slate-600"
                                >
                                  <KeyRound className="h-3 w-3" /> 重置密码
                                </Button>
                              </div>
                            </td>
                          </tr>
                        )
                      })}
                    </tbody>
                  </table>
                </div>
              </CardContent>
            </Card>
          </div>
        </div>
      )}

      {/* TAB: 省份与考号（内置省份表 + CSP 准考证号说明） */}
      {activeTab === 'provinces' && (
        <div
          role="tabpanel"
          id="teacher-panel-provinces"
          aria-labelledby="teacher-tab-provinces"
          className="space-y-5"
        >
          <ProvincesPanel
            provinces={provinces}
            loading={provincesLoading}
            error={provincesError}
            refetch={refetchProvinces}
            isAdmin={isAdmin}
          />
        </div>
      )}

      {/* TAB 3: 试题工坊、排序与 4 题复赛 PDF */}
      {activeTab === 'problems' && (
        <div
          role="tabpanel"
          id="teacher-panel-problems"
          aria-labelledby="teacher-tab-problems"
          className="space-y-5"
        >
          {!paperEditable && currentContest && (
            <div className="flex items-start gap-3 rounded-xl border border-amber-200 bg-amber-50 px-4 py-3 text-xs leading-5 text-amber-950">
              <ShieldAlert className="mt-0.5 h-4 w-4 shrink-0 text-amber-700" />
              <div>
                <p className="font-extrabold">试卷已发布，命题内容已冻结</p>
                <p className="mt-0.5 text-amber-800">
                  题面、题序、题型、测试数据、语言白名单和榜单赛制均不可再修改。公告、广播、加时、IP 解绑与人工复核仍可在“考务安全”和“实时提交”中完成。
                </p>
              </div>
            </div>
          )}
          <div className="flex flex-col md:flex-row items-start md:items-center justify-between gap-3 p-4 bg-white rounded-xl border border-slate-200 shadow-sm">
            <div>
              <h3 className="font-bold text-slate-900 text-sm">
                试题顺序编排与整卷试卷合订本 (当前 {problems.length} 道试题)
              </h3>
              <p className="text-xs text-slate-500 mt-0.5">
                支持上下移动调整题目排序，代号将自动按照 A, B, C, D 顺序规范排版。
              </p>
            </div>
            <div className="flex items-center gap-2">
              <Button
                size="sm"
                disabled={!paperEditable}
                title={paperEditable ? undefined : '比赛启动后试卷已冻结'}
                onClick={() =>
                  setEditingProblem({
                    code: String.fromCharCode(65 + problems.length),
                    title: '',
                    statement: '',
                    input: '',
                    output: '',
                    examples: '',
                    constraints: '',
                    timeLimitMs: 1000,
                    memoryLimitMib: 512,
                  })
                }
                className="gap-1.5 text-xs"
              >
                <Plus className="w-3.5 h-3.5" /> 新增试题
              </Button>
            </div>
          </div>

          {/* Problem List with Reorder Buttons */}
          <div className="space-y-3">
            {problems.map((p, idx) => (
              <Card key={p.id} className="p-4 flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
                <div className="flex items-center gap-3">
                  <div className="flex flex-col gap-1">
                    <button
                      disabled={!paperEditable || idx === 0}
                      aria-label={`上移 ${p.code}. ${p.title}`}
                      onClick={() => handleMoveProblem(idx, 'up')}
                      className="p-1 rounded hover:bg-slate-100 disabled:opacity-20 text-slate-500"
                    >
                      <ArrowUp className="w-4 h-4" />
                    </button>
                    <button
                      disabled={!paperEditable || idx === problems.length - 1}
                      aria-label={`下移 ${p.code}. ${p.title}`}
                      onClick={() => handleMoveProblem(idx, 'down')}
                      className="p-1 rounded hover:bg-slate-100 disabled:opacity-20 text-slate-500"
                    >
                      <ArrowDown className="w-4 h-4" />
                    </button>
                  </div>
                  <div>
                    <div className="flex items-center gap-2">
                      <span className="font-black text-blue-600 font-mono text-base">{p.code}.</span>
                      <h4 className="font-bold text-slate-900 text-sm">{p.title}</h4>
                    </div>
                    <span className="text-xs text-slate-400">
                      时限: {p.timeLimitMs}ms · 内存: {p.memoryLimitMib}MiB · 样例: {p.examples ? '已录入' : '无'}
                    </span>
                  </div>
                </div>

                <div className="flex items-center gap-2">
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() => handleValidateProblem(p.id)}
                    className="text-xs"
                  >
                    校验结构
                  </Button>
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={async () => {
                      setPending(`rejudge:${p.id}`)
                      try {
                        const result = await api.rejudgeProblem(cid, p.id)
                        toast.success(
                          '重判任务已提交',
                          `共 ${result.total} 份提交，成功入队 ${result.enqueued} 份。`
                        )
                        if (result.enqueued === 0) {
                          toast.notify(
                            'info',
                            '没有自动评测 worker 入队',
                            '当前部署可能关闭了 FORGE_AUTOJUDGE；历史结果保持不变，可继续人工复核。'
                          )
                        }
                      } catch (cause) {
                        toast.error('重判失败', errorText(cause))
                      } finally {
                        setPending(null)
                      }
                    }}
                    loading={pending === `rejudge:${p.id}`}
                    className="text-xs"
                  >
                    <RefreshCw className="w-3.5 h-3.5" /> 重判
                  </Button>
                  <Button
                    size="sm"
                    variant="secondary"
                    disabled={!paperEditable}
                    title={paperEditable ? undefined : '比赛启动后试卷已冻结'}
                    onClick={() => setEditingProblem(p)}
                    className="text-xs gap-1"
                  >
                    <FileEdit className="w-3.5 h-3.5" /> 编辑题面
                  </Button>
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() => {
                      void api.downloadProblemHtml(cid, p.id, p.code).catch((cause) => {
                        toast.error('单题预览下载失败', errorText(cause))
                      })
                    }}
                    className="text-xs"
                  >
                    单题 CCF
                  </Button>
                  <Button
                    size="sm"
                    variant="ghost"
                    disabled={!paperEditable}
                    title={paperEditable ? undefined : '比赛启动后试卷已冻结'}
                    onClick={() => void handleDeleteProblem(p)}
                    loading={pending === `delete-problem:${p.id}`}
                    className="gap-1 text-xs text-rose-600 hover:bg-rose-50 hover:text-rose-700"
                  >
                    <Trash2 className="w-3.5 h-3.5" /> 删除
                  </Button>
                </div>
              </Card>
            ))}
          </div>

          {problemValidationMsg && (
            <div
              className={`p-3 rounded-lg text-xs font-semibold flex items-center gap-2 ${
                problemValidationMsg.ok
                  ? 'bg-emerald-50 text-emerald-800 border border-emerald-200'
                  : 'bg-rose-50 text-rose-800 border border-rose-200'
              }`}
            >
              {problemValidationMsg.ok ? <CheckCircle2 className="w-4 h-4" /> : <XCircle className="w-4 h-4" />}
              {problemValidationMsg.msg}
            </div>
          )}

          {/* Split-screen Markdown / KaTeX Editor Drawer */}
          {editingProblem && (
            <Card className="p-5 border-blue-300 shadow-md space-y-4">
              <div className="flex items-center justify-between pb-2 border-b border-slate-100">
                <h3 className="font-bold text-base text-slate-900 flex items-center gap-2">
                  <FileEdit className="w-4 h-4 text-blue-600" />
                  在线编辑题面: {editingProblem.code || '新题'} · {editingProblem.title || '无标题'}
                </h3>
                <div className="flex gap-2">
                  <Button size="sm" variant="outline" onClick={() => setEditingProblem(null)}>
                    取消
                  </Button>
                  <Button size="sm" onClick={handleSaveProblem}>
                    保存题面
                  </Button>
                </div>
              </div>

              {/* Task Type selector */}
              <div className="flex items-center gap-3 bg-slate-50 p-2.5 rounded-lg border border-slate-200">
                <span className="text-xs font-bold text-slate-700">题型分类:</span>
                <select
                  aria-label="题型分类"
                  value={editingProblem.taskType || 'traditional'}
                  onChange={(e) => {
                    const taskType = e.target.value as TaskType
                    setEditingProblem({
                      ...editingProblem,
                      taskType,
                      input:
                        taskType === 'choice'
                          ? editingProblem.input || '答题卡 JSON'
                          : editingProblem.input,
                      output:
                        taskType === 'choice'
                          ? editingProblem.output || '标准比对得分'
                          : editingProblem.output,
                      constraints:
                        taskType === 'choice'
                          ? editingProblem.constraints || '客观选择与判断题'
                          : editingProblem.constraints,
                      interactorLang:
                        taskType === 'interaction'
                          ? editingProblem.interactorLang || 'cpp'
                          : editingProblem.interactorLang,
                      choiceQuestions:
                        taskType === 'choice'
                          ? editingProblem.choiceQuestions || []
                          : editingProblem.choiceQuestions,
                    })
                  }}
                  className="rounded-md border border-slate-300 px-3 py-1 text-xs bg-white font-semibold"
                >
                  <option value="traditional">传统算法编程题 (C++/Python/Go)</option>
                  <option value="choice">CCF GESP 客观选择/判断题 (纯内存秒判)</option>
                  <option value="answers_only">只交答案题 (文件 / 问答评测)</option>
                  <option value="interaction">交互题 (Interactor 通信评测)</option>
                </select>
                {editingProblem.taskType === 'choice' && (
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() => {
                      const templateQuestions: ChoiceQuestion[] = [
                        { id: 1, type: 'single', stem: '下列哪个是 C++ 的关键字？', options: ['A. while', 'B. integer', 'C. main', 'D. string'], answer: 'A', score: 2 },
                        { id: 2, type: 'single', stem: '在 8 位二进制补码表示中，-1 的编码是？', options: ['A. 10000001', 'B. 11111111', 'C. 11111110', 'D. 00000001'], answer: 'B', score: 2 },
                        { id: 3, type: 'judge', stem: '表达式 5 / 2 在 C++ 中的运算结果为 2.5。', answer: 'F', score: 2 },
                        { id: 4, type: 'judge', stem: 'C++ 语言中数组下标默认从 0 开始编号。', answer: 'T', score: 2 },
                        { id: 5, type: 'multiple', stem: '下列属于 C++ 基础整型类型的有（多选）：', options: ['A. int', 'B. float', 'C. short', 'D. double'], answer: ['A', 'C'], score: 4 },
                      ]
                      setEditingProblem({
                        ...editingProblem,
                        choiceQuestions: templateQuestions,
                        statement: '### CCF GESP 客观选择与判断题全卷\n\n请仔细阅读各题题干并在答题卡上作答，交卷后系统将纯内存微秒级判分。',
                        input: '答题卡 JSON',
                        output: '标准比对得分',
                        constraints: '单选 2分 / 判断 2分 / 多选 4分',
                      })
                    }}
                    className="text-xs text-blue-600 border-blue-200 hover:bg-blue-50"
                  >
                    载入 GESP 样卷（可继续编辑）
                  </Button>
                )}
              </div>

              {editingProblem.taskType === 'choice' && (
                <ChoicePaperEditor
                  questions={editingProblem.choiceQuestions || []}
                  disabled={!paperEditable}
                  onChange={(choiceQuestions) =>
                    setEditingProblem((current) =>
                      current ? { ...current, choiceQuestions } : current
                    )
                  }
                />
              )}

              {/* Metadata inputs */}
              <div className="grid gap-3 sm:grid-cols-4">
                <Input
                  placeholder="代号 (A/B/C/D)"
                  value={editingProblem.code || ''}
                  onChange={(e) => setEditingProblem({ ...editingProblem, code: e.target.value })}
                />
                <Input
                  placeholder="题目名称 (如: 距离之和)"
                  value={editingProblem.title || ''}
                  onChange={(e) => setEditingProblem({ ...editingProblem, title: e.target.value })}
                  className="sm:col-span-2"
                />
                <div className="flex gap-2">
                  <Input
                    type="number"
                    placeholder="时限 (ms)"
                    value={editingProblem.timeLimitMs || 1000}
                    onChange={(e) =>
                      setEditingProblem({ ...editingProblem, timeLimitMs: parseInt(e.target.value) || 1000 })
                    }
                  />
                  <Input
                    type="number"
                    placeholder="内存 (MiB)"
                    value={editingProblem.memoryLimitMib || 512}
                    onChange={(e) =>
                      setEditingProblem({ ...editingProblem, memoryLimitMib: parseInt(e.target.value) || 512 })
                    }
                  />
                </div>
              </div>

              {/* 文件式测试数据与自动评测点 */}
              {editingProblem.taskType !== 'choice' && editingProblem.id && (
                <div className="rounded-xl border border-slate-200 bg-white">
                  <div className="flex flex-wrap items-start justify-between gap-3 border-b border-slate-100 px-4 py-3">
                    <div>
                      <p className="flex items-center gap-1.5 text-xs font-bold text-slate-800">
                        <FolderArchive className="h-4 w-4 text-blue-600" />
                        {editingProblem.taskType === 'answers_only'
                          ? '只交答案题 · 标准答案'
                          : '测试数据文件仓'}
                      </p>
                      <p className="mt-0.5 text-[11px] text-slate-500">
                        {editingProblem.taskType === 'answers_only'
                          ? '直接编辑标准答案，或上传唯一的 .out / .ans 答案 key。单文件上限 10 MB。'
                          : '批量上传 .in / .out / .ans，成对同步为测试点。单文件上限 10 MB。'}
                      </p>
                    </div>
                    <div className="flex flex-wrap items-center gap-2">
                      <input
                        ref={testFileInputRef}
                        type="file"
            aria-label="选择测试数据文件"
                        multiple
                        accept={
                          editingProblem.taskType === 'answers_only'
                            ? '.out,.ans'
                            : '.in,.out,.ans'
                        }
                        className="sr-only"
                        onChange={(event) => {
                          void handleUploadTestFiles(event.target.files)
                        }}
                      />
                      <Button
                        size="sm"
                        variant="outline"
                        onClick={() => testFileInputRef.current?.click()}
                        loading={testFileBusy === 'upload'}
                        className="gap-1.5 text-xs"
                      >
                        <Upload className="h-3.5 w-3.5" /> 批量上传
                      </Button>
                      <Button
                        size="sm"
                        variant="secondary"
                        onClick={() => void refreshTestFiles(editingProblem.id!).catch((cause) => {
                          setTestFileError(errorText(cause))
                        })}
                        disabled={Boolean(testFileBusy)}
                        className="gap-1.5 text-xs"
                      >
                        <RefreshCw className="h-3.5 w-3.5" /> 刷新
                      </Button>
                      <Button
                        size="sm"
                        onClick={() => void handleSyncTestCases()}
                        loading={testFileBusy === 'sync'}
                        disabled={
                          Boolean(testFileBusy) ||
                          !editingTestFiles ||
                          editingTestFiles.files.length === 0 ||
                          editingTestFiles.missingPairs.length > 0
                        }
                        className="gap-1.5 text-xs"
                      >
                        <FolderArchive className="h-3.5 w-3.5" /> 同步为测试点
                      </Button>
                    </div>
                  </div>

                  <div className="space-y-3 px-4 py-3">
                    {editingProblem.taskType === 'answers_only' && (
                      <div className="space-y-2 rounded-xl border border-blue-200 bg-blue-50/45 p-3">
                        <div className="flex flex-wrap items-center justify-between gap-2">
                          <div>
                            <p className="text-xs font-bold text-blue-950">标准答案文本</p>
                            <p className="mt-0.5 text-[11px] leading-5 text-blue-800/70">
                              答案按题目的比对模式判分。保存后会替换当前唯一答案 key。
                            </p>
                          </div>
                          <Button
                            size="sm"
                            onClick={() => void handleSaveInlineAnswerKey()}
                            loading={answerKeyBusy}
                            disabled={!paperEditable || !answerKeyDraft.trim()}
                            className="text-xs"
                          >
                            保存标准答案
                          </Button>
                        </div>
                        <Textarea
                          aria-label="只交答案题标准答案"
                          rows={5}
                          value={answerKeyDraft}
                          onChange={(event) => {
                            setAnswerKeyDraft(event.target.value)
                            setAnswerKeyError('')
                          }}
                          placeholder="输入允许学生提交比对的标准答案；也可以上传 .ans/.out 文件后在下方同步。"
                          className="min-h-28 bg-white font-mono text-xs"
                        />
                        {answerKeyError && (
                          <p role="alert" className="text-[11px] font-semibold text-rose-700">
                            {answerKeyError}
                          </p>
                        )}
                      </div>
                    )}

                    <div className="flex flex-wrap items-center gap-2 text-[11px]">
                      <Badge variant={editingTestFiles?.files.length ? 'info' : 'default'}>
                        {editingTestFiles?.files.length ?? 0} 个文件
                      </Badge>
                      {editingProblem.taskType === 'answers_only' ? (
                        <Badge
                          variant={
                            (editingTestFiles?.files || []).filter((name) =>
                              /\.(?:out|ans)$/i.test(name)
                            ).length === 1
                              ? 'success'
                              : 'warning'
                          }
                        >
                          {(editingTestFiles?.files || []).filter((name) =>
                            /\.(?:out|ans)$/i.test(name)
                          ).length}{' '}
                          个答案 key
                        </Badge>
                      ) : (
                        <Badge
                          variant={
                            editingTestFiles?.missingPairs.length ? 'warning' : 'success'
                          }
                        >
                          {editingTestFiles?.missingPairs.length
                            ? `${editingTestFiles.missingPairs.length} 个未配对`
                            : '全部成对'}
                        </Badge>
                      )}
                      <span className="text-slate-400">
                        {editingProblem.taskType === 'answers_only'
                          ? `当前答案 key ${editingProblem.testCases?.length ?? 0} 个`
                          : `当前测试点 ${editingProblem.testCases?.length ?? 0} 组`}
                      </span>
                    </div>

                    {editingTestFiles?.missingPairs.length ? (
                      <div className="rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-[11px] leading-5 text-amber-900">
                        缺少配对：{editingTestFiles.missingPairs.join('、')}。补齐后即可同步。
                      </div>
                    ) : null}

                    {testFileError && (
                      <div
                        role="alert"
                        className="rounded-lg border border-rose-200 bg-rose-50 px-3 py-2 text-[11px] leading-5 text-rose-800"
                      >
                        {testFileError}
                      </div>
                    )}

                    {!editingTestFiles || editingTestFiles.files.length === 0 ? (
                      <div className="rounded-lg border border-dashed border-slate-300 bg-slate-50 px-3 py-6 text-center text-[11px] text-slate-400">
                        {editingProblem.taskType === 'answers_only'
                          ? '暂无答案 key 文件。可直接编辑上方标准答案，或上传一个 .out/.ans 文件。'
                          : '暂无测试数据文件。上传同名 a01.in 与 a01.out 即可生成一组测试点。'}
                      </div>
                    ) : (
                      <div className="max-h-52 overflow-y-auto rounded-lg border border-slate-200">
                        <table className="w-full text-left text-[11px]">
                          <thead className="sticky top-0 bg-slate-50 font-semibold text-slate-600">
                            <tr>
                              <th className="px-3 py-2">文件名</th>
                              <th className="px-3 py-2">
                                {editingProblem.taskType === 'answers_only'
                                  ? '文件用途'
                                  : '配对状态'}
                              </th>
                              <th className="px-3 py-2 text-right">操作</th>
                            </tr>
                          </thead>
                          <tbody className="divide-y divide-slate-100 bg-white">
                            {editingTestFiles.files.map((name) => {
                              const missing =
                                editingTestFiles.missingPairs.includes(name)
                              return (
                                <tr key={name}>
                                  <td className="px-3 py-1.5 font-mono text-slate-700">
                                    {name}
                                  </td>
                                  <td className="px-3 py-1.5">
                                    {editingProblem.taskType === 'answers_only' ? (
                                      <Badge
                                        variant={
                                          /\.(?:out|ans)$/i.test(name) ? 'success' : 'default'
                                        }
                                      >
                                        {/\.(?:out|ans)$/i.test(name)
                                          ? '答案 key'
                                          : '.in 文件将在同步时忽略'}
                                      </Badge>
                                    ) : (
                                      <Badge variant={missing ? 'warning' : 'success'}>
                                        {missing ? '缺少配对' : '已配对'}
                                      </Badge>
                                    )}
                                  </td>
                                  <td className="px-3 py-1.5 text-right">
                                    <Button
                                      size="sm"
                                      variant="ghost"
                                      onClick={() => void handleDeleteTestFile(name)}
                                      loading={testFileBusy === `delete:${name}`}
                                      disabled={Boolean(testFileBusy)}
                                      className="h-7 gap-1 px-2 text-[10px] text-rose-600 hover:bg-rose-50 hover:text-rose-700"
                                    >
                                      <Trash2 className="h-3 w-3" /> 删除
                                    </Button>
                                  </td>
                                </tr>
                              )
                            })}
                          </tbody>
                        </table>
                      </div>
                    )}
                  </div>
                </div>
              )}

              {/* LemonNext 级高级评测与 IO 约束配置 */}
              {editingProblem.taskType !== 'choice' && (
                <div className="p-4 rounded-xl border border-slate-200 bg-slate-50/70 space-y-3">
                  <div className="flex items-center justify-between">
                    <span className="text-xs font-bold text-slate-800 flex items-center gap-1.5">
                      <Settings className="w-4 h-4 text-blue-600" />
                      高级评测与 IO 模式约束 (对标 LemonNext 全功能)
                    </span>
                    <span className="text-[11px] text-slate-400">
                      支持标准/文件 IO、多模式比对、浮点误差与特判
                    </span>
                  </div>

                  <div className="grid gap-3 sm:grid-cols-3">
                    {/* IO 模式 */}
                    <div>
                      <label className="block text-[11px] font-semibold text-slate-600 mb-1">
                        IO 模式
                      </label>
                      <select
                        value={editingProblem.ioMode || 'stdio'}
                        onChange={(e) => {
                          const val = e.target.value as 'stdio' | 'file'
                          const baseCode = (editingProblem.code || 'task').toLowerCase()
                          setEditingProblem({
                            ...editingProblem,
                            ioMode: val,
                            inFile: val === 'file' ? (editingProblem.inFile || `${baseCode}.in`) : '',
                            outFile: val === 'file' ? (editingProblem.outFile || `${baseCode}.out`) : '',
                          })
                        }}
                        className="w-full rounded-md border border-slate-300 px-2.5 py-1.5 text-xs bg-white"
                      >
                        <option value="stdio">标准 IO (stdin / stdout - 洛谷/CF格式)</option>
                        <option value="file">文件 IO (freopen 考场格式)</option>
                      </select>
                    </div>

                    {/* 比对模式 */}
                    <div>
                      <label className="block text-[11px] font-semibold text-slate-600 mb-1">
                        结果比对模式
                      </label>
                      <select
                        value={editingProblem.compareMode || 'ignore_space'}
                        onChange={(e) =>
                          setEditingProblem({
                            ...editingProblem,
                            compareMode: e.target.value as CompareMode,
                          })
                        }
                        className="w-full rounded-md border border-slate-300 px-2.5 py-1.5 text-xs bg-white"
                      >
                        <option value="ignore_space">忽略多余空白 (NOI/CSP 官方默认)</option>
                        <option value="line_by_line">逐行严格比对 (diff / 格式敏感)</option>
                        <option value="real_number">浮点数精度容差 (实数判断)</option>
                        <option value="special_judge">自定义特判器 (SPJ / testlib.h)</option>
                      </select>
                    </div>

                    {/* 浮点容差或编译超时 */}
                    {(editingProblem.compareMode === 'float_eps' ||
                      editingProblem.compareMode === 'real_number') ? (
                      <div>
                        <label className="block text-[11px] font-semibold text-slate-600 mb-1">
                          浮点容差 Epsilon
                        </label>
                        <Input
                          type="number"
                          step="0.000001"
                          placeholder="1e-6 (0.000001)"
                          value={editingProblem.realEps || 0.000001}
                          onChange={(e) =>
                            setEditingProblem({
                              ...editingProblem,
                              realEps: parseFloat(e.target.value) || 0.000001,
                            })
                          }
                          className="h-8 text-xs"
                        />
                      </div>
                    ) : (
                      <div className="grid grid-cols-2 gap-2">
                        <div>
                          <label className="block text-[11px] font-semibold text-slate-600 mb-1">
                            输出上限 (KiB)
                          </label>
                          <Input
                            type="number"
                            placeholder="65536"
                            value={editingProblem.outputLimitKib || 65536}
                            onChange={(e) =>
                              setEditingProblem({
                                ...editingProblem,
                                outputLimitKib: parseInt(e.target.value) || 65536,
                              })
                            }
                            className="h-8 text-xs"
                          />
                        </div>
                        <div>
                          <label className="block text-[11px] font-semibold text-slate-600 mb-1">
                            编译超时 (s)
                          </label>
                          <Input
                            type="number"
                            placeholder="15"
                            value={editingProblem.compileTimeoutSec || 15}
                            onChange={(e) =>
                              setEditingProblem({
                                ...editingProblem,
                                compileTimeoutSec: parseInt(e.target.value) || 15,
                              })
                            }
                            className="h-8 text-xs"
                          />
                        </div>
                      </div>
                    )}
                  </div>

                  {/* 文件 IO 详情 */}
                  {editingProblem.ioMode === 'file' && (
                    <div className="p-2.5 rounded-lg bg-blue-50/60 border border-blue-200/60 grid sm:grid-cols-2 gap-3 text-xs">
                      <div>
                        <span className="font-semibold text-blue-900 block mb-1">输入重定向文件 (.in)</span>
                        <Input
                          placeholder="如: add.in"
                          value={editingProblem.inFile || ''}
                          onChange={(e) => setEditingProblem({ ...editingProblem, inFile: e.target.value })}
                          className="h-8 text-xs bg-white"
                        />
                      </div>
                      <div>
                        <span className="font-semibold text-blue-900 block mb-1">输出重定向文件 (.out)</span>
                        <Input
                          placeholder="如: add.out"
                          value={editingProblem.outFile || ''}
                          onChange={(e) => setEditingProblem({ ...editingProblem, outFile: e.target.value })}
                          className="h-8 text-xs bg-white"
                        />
                      </div>
                    </div>
                  )}

                  {editingProblem.taskType === 'interaction' && (
                    <div className="space-y-2 rounded-lg border border-violet-200/70 bg-violet-50/55 p-2.5 text-xs">
                      <div className="flex flex-wrap items-center justify-between gap-2">
                        <div>
                          <span className="font-bold text-violet-950">
                            Interactor 交互器源码
                          </span>
                          <p className="mt-0.5 text-[10px] leading-5 text-violet-800/70">
                            源码只保存到服务端评测配置，浏览器不会执行。
                          </p>
                        </div>
                        <label className="flex items-center gap-1.5 text-[10px] font-bold text-violet-900">
                          语言
                          <select
                            aria-label="交互器语言"
                            value={editingProblem.interactorLang || 'cpp'}
                            onChange={(event) =>
                              setEditingProblem({
                                ...editingProblem,
                                interactorLang: event.target.value,
                              })
                            }
                            className="h-7 rounded border border-violet-300 bg-white px-2 text-[10px]"
                          >
                            <option value="cpp">C++</option>
                            <option value="c">C</option>
                            <option value="python">Python 3</option>
                            <option value="go">Go</option>
                          </select>
                        </label>
                      </div>
                      <Textarea
                        aria-label="交互器源码"
                        rows={6}
                        placeholder="// Interactor：通过 stdin/stdout 与选手程序交互，并按协议返回 AC/WA"
                        value={editingProblem.interactorCode || ''}
                        onChange={(event) =>
                          setEditingProblem({
                            ...editingProblem,
                            interactorCode: event.target.value,
                          })
                        }
                        className="min-h-32 bg-white font-mono text-xs"
                      />
                    </div>
                  )}

                  {/* 自定义特判 SPJ 详情 */}
                  {(editingProblem.compareMode === 'custom_checker' ||
                    editingProblem.compareMode === 'special_judge') && (
                    <div className="p-2.5 rounded-lg bg-amber-50/60 border border-amber-200/60 space-y-2 text-xs">
                      <div className="flex flex-wrap items-center justify-between gap-2">
                        <div>
                          <span className="font-bold text-amber-900">Custom Special Judge 源码</span>
                          <p className="text-[10px] text-amber-700">
                            基于 testlib.h / 传统 argv[1,2,3] 返回 0 为 AC；浏览器不会执行。
                          </p>
                        </div>
                        <label className="flex items-center gap-1.5 text-[10px] font-bold text-amber-900">
                          语言
                          <select
                            aria-label="检查器语言"
                            value={editingProblem.checkerLang || 'cpp'}
                            onChange={(event) =>
                              setEditingProblem({
                                ...editingProblem,
                                checkerLang: event.target.value,
                              })
                            }
                            className="h-7 rounded border border-amber-300 bg-white px-2 text-[10px]"
                          >
                            <option value="cpp">C++</option>
                            <option value="c">C</option>
                            <option value="python">Python 3</option>
                            <option value="go">Go</option>
                          </select>
                        </label>
                      </div>
                      <Textarea
                        aria-label="检查器源码"
                        rows={4}
                        placeholder="// #include <testlib.h> 或标准 SPJ 判断..."
                        value={editingProblem.checkerCode || ''}
                        onChange={(e) => setEditingProblem({ ...editingProblem, checkerCode: e.target.value })}
                        className="min-h-24 bg-white font-mono text-xs"
                      />
                    </div>
                  )}

                  {subtaskDrafts.length > 0 && (
                    <div className="space-y-2 rounded-lg border border-slate-200 bg-white p-2.5">
                      <div className="flex flex-wrap items-center justify-between gap-2">
                        <div>
                          <span className="text-xs font-bold text-slate-800">
                            子任务分值、限额与依赖
                          </span>
                          <p className="mt-0.5 text-[10px] text-slate-500">
                            分数按组总分写入测试点；依赖组未通过时，后续组自动跳过。
                          </p>
                        </div>
                        <Button
                          size="sm"
                          variant="outline"
                          loading={subtaskBusy}
                          disabled={!paperEditable}
                          onClick={() => void handleSaveSubtasks()}
                          className="text-xs"
                        >
                          保存子任务配置
                        </Button>
                      </div>
                      <div className="overflow-x-auto rounded-lg border border-slate-200">
                        <table className="w-full text-left text-[11px]">
                          <thead className="bg-slate-50 font-semibold text-slate-600">
                            <tr>
                              <th className="px-2 py-2">子任务</th>
                              <th className="px-2 py-2">测试点</th>
                              <th className="px-2 py-2">组总分</th>
                              <th className="px-2 py-2">时限 ms</th>
                              <th className="px-2 py-2">内存 MiB</th>
                              <th className="px-2 py-2">依赖组</th>
                            </tr>
                          </thead>
                          <tbody className="divide-y divide-slate-100">
                            {subtaskDrafts.map((item) => (
                              <tr key={item.subtask}>
                                <td className="px-2 py-1.5 font-mono font-bold text-slate-700">
                                  #{item.subtask}
                                </td>
                                <td className="px-2 py-1.5 text-slate-500">{item.cases}</td>
                                <td className="px-2 py-1.5">
                                  <Input
                                    aria-label={`子任务 ${item.subtask} 总分`}
                                    type="number"
                                    min={0}
                                    max={100}
                                    value={item.score}
                                    onChange={(event) =>
                                      updateSubtaskDraft(item.subtask, {
                                        score: Number(event.target.value) || 0,
                                      })
                                    }
                                    className="h-7 w-20 text-xs"
                                  />
                                </td>
                                <td className="px-2 py-1.5">
                                  <Input
                                    aria-label={`子任务 ${item.subtask} 时限`}
                                    type="number"
                                    min={0}
                                    value={item.timeLimitMs}
                                    onChange={(event) =>
                                      updateSubtaskDraft(item.subtask, {
                                        timeLimitMs: Number(event.target.value) || 0,
                                      })
                                    }
                                    className="h-7 w-24 text-xs"
                                  />
                                </td>
                                <td className="px-2 py-1.5">
                                  <Input
                                    aria-label={`子任务 ${item.subtask} 内存`}
                                    type="number"
                                    min={0}
                                    value={item.memoryMib}
                                    onChange={(event) =>
                                      updateSubtaskDraft(item.subtask, {
                                        memoryMib: Number(event.target.value) || 0,
                                      })
                                    }
                                    className="h-7 w-20 text-xs"
                                  />
                                </td>
                                <td className="px-2 py-1.5">
                                  <Input
                                    aria-label={`子任务 ${item.subtask} 依赖`}
                                    value={item.dependsOn}
                                    onChange={(event) =>
                                      updateSubtaskDraft(item.subtask, {
                                        dependsOn: event.target.value,
                                      })
                                    }
                                    placeholder="如 1,2"
                                    className="h-7 w-24 font-mono text-xs"
                                  />
                                </td>
                              </tr>
                            ))}
                          </tbody>
                        </table>
                      </div>
                      <div className="flex flex-wrap items-center justify-between gap-2 text-[10px]">
                        <span
                          className={
                            subtaskDrafts.reduce((sum, item) => sum + item.score, 0) === 100
                              ? 'font-semibold text-emerald-700'
                              : 'font-semibold text-amber-700'
                          }
                        >
                          当前总分{' '}
                          {subtaskDrafts.reduce((sum, item) => sum + item.score, 0)} / 100
                        </span>
                        {subtaskError && (
                          <span role="alert" className="font-semibold text-rose-700">
                            {subtaskError}
                          </span>
                        )}
                      </div>
                    </div>
                  )}
                </div>
              )}

              {/* Split Markdown Input vs Real-time KaTeX Preview */}
              <div className="grid gap-4 md:grid-cols-2">
                <div className="space-y-3">
                  <span className="text-xs font-bold text-slate-500 uppercase tracking-wider block">
                    题目描述 (Markdown + LaTeX 公式)
                  </span>
                  <textarea
                    rows={6}
                    value={editingProblem.statement || ''}
                    onChange={(e) => setEditingProblem({ ...editingProblem, statement: e.target.value })}
                    placeholder="输入 Markdown 描述，支持行内公式 $O(n \log n)$ 与块级公式 $$...$$"
                    className="w-full text-xs font-mono p-3 rounded-lg border border-slate-300 focus:ring-2 focus:ring-blue-500"
                  />
                  <div className="grid grid-cols-2 gap-2">
                    <Input
                      placeholder="输入格式"
                      value={editingProblem.input || ''}
                      onChange={(e) => setEditingProblem({ ...editingProblem, input: e.target.value })}
                    />
                    <Input
                      placeholder="输出格式"
                      value={editingProblem.output || ''}
                      onChange={(e) => setEditingProblem({ ...editingProblem, output: e.target.value })}
                    />
                  </div>
                  <textarea
                    rows={3}
                    value={editingProblem.examples || ''}
                    onChange={(e) => setEditingProblem({ ...editingProblem, examples: e.target.value })}
                    placeholder="样例数据 (输入与输出)"
                    className="w-full text-xs font-mono p-2 rounded-lg border border-slate-300"
                  />
                  <Input
                    placeholder="数据范围与约定 (如: 对于 100% 的数据，n <= 10^5)"
                    value={editingProblem.constraints || ''}
                    onChange={(e) => setEditingProblem({ ...editingProblem, constraints: e.target.value })}
                  />
                </div>

                {/* Right: Live Preview */}
                <div className="p-4 bg-slate-50 rounded-lg border border-slate-200 overflow-y-auto max-h-[480px]">
                  <span className="text-xs font-bold text-slate-400 uppercase tracking-wider block mb-2">
                    CCF 实时渲染预览
                  </span>
                  <h4 className="text-base font-bold text-slate-900 mb-2">
                    {editingProblem.code}. {editingProblem.title}
                  </h4>
                  <MarkdownView source={editingProblem.statement} />
                  {editingProblem.examples && (
                    <div className="mt-3">
                      <span className="text-xs font-bold text-slate-500">样例：</span>
                      <pre className="p-2 bg-slate-900 text-slate-100 rounded text-xs mt-1">
                        {editingProblem.examples}
                      </pre>
                    </div>
                  )}
                </div>
              </div>
            </Card>
          )}
        </div>
      )}

      {/* TAB 4: 学情统计与代码版本演进追踪 (Diff Viewer) */}
      {activeTab === 'analytics' && (
        <div
          role="tabpanel"
          id="teacher-panel-analytics"
          aria-labelledby="teacher-tab-analytics"
          className="space-y-5"
        >
          <Card>
            <CardHeader className="flex flex-row flex-wrap items-center justify-between gap-3 pb-3">
              <div>
                <CardTitle className="text-base flex items-center gap-2">
                  <Activity className="w-5 h-5 text-emerald-600" />
                  本场学情总览
                </CardTitle>
                <p className="mt-1 text-[11px] text-slate-500">
                  统计来自服务端实时提交账本，刷新不会受浏览器缓存影响。
                </p>
              </div>
              <Button
                size="sm"
                variant="outline"
                onClick={() => void statisticsQuery.refetch()}
                disabled={statisticsQuery.isFetching}
                className="gap-1.5 text-xs"
              >
                <RefreshCw className="h-3.5 w-3.5" />
                {statisticsQuery.isFetching ? '刷新中' : '刷新统计'}
              </Button>
            </CardHeader>
            <CardContent>
              {statisticsQuery.isLoading ? (
                <div className="rounded-xl border border-dashed border-slate-200 bg-slate-50 px-4 py-10 text-center text-xs text-slate-400">
                  正在汇总本场数据…
                </div>
              ) : statisticsQuery.isError ? (
                <div
                  role="alert"
                  className="flex flex-wrap items-center justify-between gap-3 rounded-xl border border-rose-200 bg-rose-50 px-4 py-3 text-xs text-rose-800"
                >
                  <span>
                    统计加载失败：
                    {statisticsQuery.error instanceof Error
                      ? statisticsQuery.error.message
                      : '服务暂时不可用。'}
                  </span>
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() => void statisticsQuery.refetch()}
                  >
                    重试
                  </Button>
                </div>
              ) : statisticsQuery.data ? (
                <div className="space-y-4">
                  <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
                    {[
                      {
                        label: '参赛选手',
                        value: statisticsQuery.data.users,
                        hint: '已绑定到本场',
                        tone: 'text-blue-900 bg-blue-50 border-blue-200',
                      },
                      {
                        label: '累计提交',
                        value: statisticsQuery.data.submissions,
                        hint: '含所有判定状态',
                        tone: 'text-violet-900 bg-violet-50 border-violet-200',
                      },
                      {
                        label: '题目数量',
                        value: statisticsProblems.length,
                        hint: '当前试卷',
                        tone: 'text-amber-900 bg-amber-50 border-amber-200',
                      },
                      {
                        label: '整体通过率',
                        value: `${
                          statisticsProblems.reduce(
                            (sum, item) => sum + item.accepted,
                            0
                          ) === 0
                            ? 0
                            : Math.round(
                                (statisticsProblems.reduce(
                                  (sum, item) => sum + item.accepted,
                                  0
                                ) /
                                  Math.max(
                                    1,
                                    statisticsQuery.data.submissions
                                  )) *
                                  1000
                              ) / 10
                        }%`,
                        hint: 'AC / 全部提交',
                        tone: 'text-emerald-900 bg-emerald-50 border-emerald-200',
                      },
                    ].map((metric) => (
                      <div
                        key={metric.label}
                        className={`rounded-xl border p-4 ${metric.tone}`}
                      >
                        <p className="text-[10px] font-bold uppercase tracking-[0.14em]">
                          {metric.label}
                        </p>
                        <p className="mt-1 font-mono text-2xl font-black">{metric.value}</p>
                        <p className="mt-0.5 text-[10px]">{metric.hint}</p>
                      </div>
                    ))}
                  </div>

                  <div className="overflow-x-auto rounded-xl border border-slate-200">
                    <table className="w-full text-left text-xs">
                      <thead className="bg-slate-50 font-semibold text-slate-600">
                        <tr>
                          <th className="px-3 py-2.5">题目</th>
                          <th className="px-3 py-2.5 text-right">提交</th>
                          <th className="px-3 py-2.5 text-right">AC</th>
                          <th className="px-3 py-2.5 text-right">通过率</th>
                          <th className="px-3 py-2.5 text-right">最高分</th>
                          <th className="px-3 py-2.5">首 AC</th>
                        </tr>
                      </thead>
                      <tbody className="divide-y divide-slate-100">
                        {statisticsProblems.map((item) => (
                          <tr key={item.problemId} className="hover:bg-slate-50/70">
                            <td className="px-3 py-2.5">
                              <span className="font-mono font-bold text-blue-700">
                                {item.code}
                              </span>
                              <span className="ml-2 font-semibold text-slate-700">
                                {item.title}
                              </span>
                            </td>
                            <td className="px-3 py-2.5 text-right font-mono">
                              {item.submissions}
                            </td>
                            <td className="px-3 py-2.5 text-right font-mono text-emerald-700">
                              {item.accepted}
                            </td>
                            <td className="px-3 py-2.5 text-right font-mono">
                              {item.submissions
                                ? `${(item.acceptRate * 100).toFixed(1)}%`
                                : '—'}
                            </td>
                            <td className="px-3 py-2.5 text-right font-mono font-bold">
                              {item.bestScore}
                            </td>
                            <td className="px-3 py-2.5 text-slate-500">
                              {item.firstACUser || '暂无'}
                            </td>
                          </tr>
                        ))}
                        {statisticsProblems.length === 0 && (
                          <tr>
                            <td
                              colSpan={6}
                              className="px-3 py-8 text-center text-slate-400"
                            >
                              本场暂无题目统计数据。
                            </td>
                          </tr>
                        )}
                      </tbody>
                    </table>
                  </div>
                </div>
              ) : null}
            </CardContent>
          </Card>

          <Card>
            <CardHeader className="flex flex-row items-center justify-between pb-3">
              <CardTitle className="text-base flex items-center gap-2">
                <LineChart className="w-5 h-5 text-blue-600" />
                学生做题连续性与代码版本演进对比 (Monaco Diff Editor)
              </CardTitle>
              {cid && (
                <Button
                  size="sm"
                  variant="outline"
                  onClick={() => {
                    void api.downloadStatisticsCsv(cid).catch((cause) => {
                      toast.error('成绩单导出失败', errorText(cause))
                    })
                  }}
                  className="gap-1.5 text-xs"
                >
                  <Download className="w-3.5 h-3.5" /> 导出本场成绩单 CSV
                </Button>
              )}
            </CardHeader>
            <CardContent className="space-y-4">
              {/* Selectors for Student and Problem */}
              <div className="grid gap-3 sm:grid-cols-2 p-3 bg-slate-50 rounded-lg border border-slate-200">
                <div>
                  <span className="text-xs font-semibold text-slate-600 block mb-1">
                    选择选手 (可按学号/同名区分):
                  </span>
                  <select
                    aria-label="选择学情学生"
                    value={selectedStudentId}
                    onChange={(e) => {
                      setSelectedStudentId(e.target.value)
                      setDiffVerA(0)
                      setDiffVerB(1)
                    }}
                    className="w-full text-xs rounded border border-slate-300 px-3 py-1.5 bg-white font-medium"
                  >
                    <option value="">-- 请选择学生 --</option>
                    {users.map((u) => (
                      <option key={u.id} value={u.id}>
                        {u.name} ({u.username})
                      </option>
                    ))}
                  </select>
                </div>
                <div>
                  <span className="text-xs font-semibold text-slate-600 block mb-1">选择题目:</span>
                  <select
                    aria-label="选择学情题目"
                    value={selectedProblemId}
                    onChange={(e) => {
                      setSelectedProblemId(e.target.value)
                      setDiffVerA(0)
                      setDiffVerB(1)
                    }}
                    className="w-full text-xs rounded border border-slate-300 px-3 py-1.5 bg-white font-medium"
                  >
                    <option value="">-- 请选择题目 --</option>
                    {problems.map((p) => (
                      <option key={p.id} value={p.id}>
                        {p.code}. {p.title}
                      </option>
                    ))}
                  </select>
                </div>
              </div>

              {/* Version Progression Timeline */}
              {selectedStudentId && selectedProblemId && (
                <div className="space-y-4">
                  <div>
                    <h4 className="text-xs font-bold text-slate-500 uppercase tracking-wider mb-2">
                      做题演进历程 (共 {evolutionVersions.length} 个代码版本)
                    </h4>
                    {evolutionVersions.length === 0 ? (
                      <div className="py-8 text-center text-xs text-slate-400 border border-dashed rounded-lg">
                        该学生针对该题目暂无代码版本（保存或提交都会生成版本）
                      </div>
                    ) : (
                      <div className="flex flex-wrap items-center gap-2">
                        {evolutionVersions.map((s, idx) => (
                          <div
                            key={s.id}
                            className={`p-2 rounded-lg border text-xs flex items-center gap-2 cursor-pointer transition-all ${
                              diffVerA === idx
                                ? 'border-rose-400 bg-rose-50/80 ring-1 ring-rose-400'
                                : diffVerB === idx
                                ? 'border-emerald-400 bg-emerald-50/80 ring-1 ring-emerald-400'
                                : 'border-slate-200 bg-white hover:bg-slate-50'
                            }`}
                            onClick={() => {
                              if (diffVerA === idx) {
                                // deselect
                              } else if (diffVerB === idx) {
                                // deselect
                              } else {
                                setDiffVerA(diffVerB)
                                setDiffVerB(idx)
                              }
                            }}
                          >
                            <span className="font-mono font-bold text-blue-700 bg-blue-100 px-1 py-0.5 rounded text-[10px]">
                              {versionLabel(s)}
                            </span>
                            <span className="text-[10px] text-slate-500">
                              {new Date(s.createdAt).toLocaleTimeString()}
                            </span>
                          </div>
                        ))}
                      </div>
                    )}
                  </div>

                  {/* Monaco Diff Viewer */}
                  {evolutionVersions.length >= 2 && (
                    <div className="pt-2">
                      <div className="flex items-center justify-between text-xs mb-2">
                        <span className="font-semibold text-slate-700 flex items-center gap-1.5">
                          <GitCompare className="w-4 h-4 text-blue-600" />
                          对比版本: {versionLabel(evolutionVersions[diffVerA])} vs {versionLabel(evolutionVersions[diffVerB])}
                        </span>
                        <span className="text-slate-400 text-[11px]">
                          左侧为较早版本，右侧为演进后版本，高亮显示算法修改与调试差异
                        </span>
                      </div>
                      <React.Suspense fallback={<EditorLoading />}>
                        <CodeDiffViewer
                          original={evolutionVersions[diffVerA]?.code || ''}
                          modified={evolutionVersions[diffVerB]?.code || ''}
                          originalTitle={versionLabel(evolutionVersions[diffVerA])}
                          modifiedTitle={versionLabel(evolutionVersions[diffVerB])}
                          language={evolutionVersions[diffVerB]?.language || 'cpp'}
                          height="440px"
                        />
                      </React.Suspense>
                    </div>
                  )}
                </div>
              )}
            </CardContent>
          </Card>
        </div>
      )}

      {/* TAB 5: 实时提交流与判题 */}
      {activeTab === 'submissions' && (
        <Card
          role="tabpanel"
          id="teacher-panel-submissions"
          aria-labelledby="teacher-tab-submissions"
        >
          <CardHeader>
            <CardTitle className="text-base flex items-center justify-between">
              <span>提交流监控 (实时 SSE 连接)</span>
              <span className="text-xs text-slate-400 font-normal">
                最近载入 {submissions.length} 条提交记录
              </span>
            </CardTitle>
          </CardHeader>
          <CardContent>
            {submissions.length === 0 ? (
              <div className="py-12 text-center text-sm text-slate-400 border border-dashed rounded-lg">
                暂无提交记录
              </div>
            ) : (
              <div className="space-y-3">
                <div className="flex items-start gap-2 rounded-lg border border-sky-200 bg-sky-50 px-3 py-2 text-[11px] leading-5 text-sky-900">
                  <Info className="mt-0.5 h-3.5 w-3.5 shrink-0" />
                  <span>
                    自动评测默认关闭；状态为 <code className="font-mono font-bold">queued</code> 的提交不会自行执行。教师可用下方
                    “复核判定”录入真实分数、结论和编译信息；若部署了独立评测 worker，结果会自动回写。
                  </span>
                </div>
                <div className="overflow-x-auto rounded-lg border border-slate-200">
                <table className="w-full text-left text-xs">
                  <thead className="bg-slate-50 font-semibold text-slate-600 border-b border-slate-200">
                    <tr>
                      <th className="px-3 py-2.5">提交 ID</th>
                      <th className="px-3 py-2.5">选手 / 学号</th>
                      <th className="px-3 py-2.5">题目</th>
                      <th className="px-3 py-2.5">语言</th>
                      <th className="px-3 py-2.5">状态</th>
                      <th className="px-3 py-2.5 text-center">得分</th>
                      <th className="px-3 py-2.5">提交时间</th>
                      <th className="px-3 py-2.5 text-right">人工复核 / 判题</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-slate-100">
                    {submissions.map((s) => (
                      <tr key={s.id} className="hover:bg-slate-50/80">
                        <td className="px-3 py-2 font-mono text-slate-400 text-[11px]">{s.id}</td>
                        <td className="px-3 py-2 font-semibold text-slate-800">{s.userName || s.userId}</td>
                        <td className="px-3 py-2 font-mono font-medium text-blue-600">{s.problemID}</td>
                        <td className="px-3 py-2 font-mono text-slate-500">{s.language}</td>
                        <td className="px-3 py-2">
                          <span
                            className={`px-2 py-0.5 rounded text-[10px] font-semibold ${
                              s.verdict === 'accepted'
                                ? 'bg-emerald-100 text-emerald-800'
                                : s.verdict === 'wrong_answer'
                                ? 'bg-rose-100 text-rose-800'
                                : 'bg-slate-100 text-slate-800'
                            }`}
                          >
                            {s.verdict}
                          </span>
                        </td>
                        <td className="px-3 py-2 text-center font-bold text-slate-900">{s.score}</td>
                        <td className="px-3 py-2 text-slate-400 text-[11px]">
                          {new Date(s.submittedAt).toLocaleTimeString()}
                        </td>
                        <td className="px-3 py-2 text-right">
                          <div className="flex items-center justify-end gap-1.5">
                            <select
                              aria-label={`${s.id} 判定结论`}
                              value={submissionDraft(s).verdict}
                              onChange={(event) =>
                                setJudgeDrafts((current) => ({
                                  ...current,
                                  [s.id]: { ...submissionDraft(s), verdict: event.target.value },
                                }))
                              }
                              className="h-7 rounded border border-slate-300 bg-white px-1.5 text-[10px]"
                            >
                              <option value="accepted">AC 通过</option>
                              <option value="wrong_answer">WA 错误</option>
                              <option value="time_limit">TLE 超时</option>
                              <option value="memory_limit">MLE 内存</option>
                              <option value="runtime_error">RE 运行错误</option>
                              <option value="compile_error">CE 编译错误</option>
                              <option value="presentation_error">PE 格式错误</option>
                              <option value="skipped">Skipped 跳过</option>
                            </select>
                            <Input
                              aria-label={`${s.id} 得分`}
                              type="number"
                              min={0}
                              value={submissionDraft(s).score}
                              onChange={(event) =>
                                setJudgeDrafts((current) => ({
                                  ...current,
                                  [s.id]: {
                                    ...submissionDraft(s),
                                    score: Number(event.target.value) || 0,
                                  },
                                }))
                              }
                              className="h-7 w-16 px-1.5 text-center text-[10px]"
                            />
                            <Button
                              size="sm"
                              variant="outline"
                              loading={judgeBusy === s.id}
                              onClick={() => void handleJudgeSubmission(s)}
                              className="h-7 px-2 text-[10px] text-emerald-700"
                            >
                              复核判定
                            </Button>
                            <Button
                              size="sm"
                              variant="ghost"
                              onClick={() =>
                                setExpandedSubmissionId(expandedSubmissionId === s.id ? '' : s.id)
                              }
                              className="h-7 px-2 text-[10px] text-slate-600"
                            >
                              {expandedSubmissionId === s.id ? '收起' : '详情'}
                            </Button>
                          </div>
                          {expandedSubmissionId === s.id && (
                            <div className="mt-2 max-w-md rounded-lg border border-slate-200 bg-slate-50 p-2 text-left text-[10px] text-slate-600">
                              <p className="font-semibold text-slate-700">
                                评测模式：{s.verdict === 'queued' ? '等待人工复核 / 自动评测未启用' : '已有判定'}
                              </p>
                              <p className="mt-1">
                                语言：{s.language} · 提交：{new Date(s.submittedAt).toLocaleString()}
                              </p>
                              {s.compileMessage && (
                                <pre className="mt-1 max-h-28 overflow-auto whitespace-pre-wrap rounded bg-white p-2 font-mono text-[10px] text-rose-700">
                                  {s.compileMessage}
                                </pre>
                              )}
                              {s.cases?.length ? (
                                <div className="mt-1 space-y-0.5">
                                  {s.cases.map((item) => (
                                    <p key={`${s.id}-${item.caseIndex}`}>
                                      #{item.caseIndex + 1} · {item.verdict} · {item.score} 分
                                    </p>
                                  ))}
                                </div>
                              ) : (
                                <p className="mt-1 text-slate-400">暂无逐点结果。</p>
                              )}
                            </div>
                          )}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
                </div>
              </div>
            )}
          </CardContent>
        </Card>
      )}

      <Dialog
        open={Boolean(resetUser)}
        onClose={() => {
          if (!resetBusy) setResetUser(null)
        }}
        title="重置账号密码"
        eyebrow="Credential rotation"
        width="sm"
        footer={
          <>
            <Button variant="ghost" disabled={resetBusy} onClick={() => setResetUser(null)}>
              取消
            </Button>
            <Button
              loading={resetBusy}
              disabled={resetPassword.length < 10}
              onClick={async () => {
                if (!resetUser) return
                setResetBusy(true)
                try {
                  await api.resetUserPassword(resetUser.id, resetPassword)
                  toast.success(
                    '密码已重置',
                    `${resetUser.name}（${resetUser.username}）的旧会话已失效。`
                  )
                  setResetUser(null)
                  setResetPassword('')
                } catch (cause) {
                  toast.error('重置密码失败', errorText(cause))
                } finally {
                  setResetBusy(false)
                }
              }}
            >
              确认重置
            </Button>
          </>
        }
      >
        <div className="space-y-4">
          <p className="text-sm text-slate-600">
            为 <strong>{resetUser?.name}</strong>（{resetUser?.username}）设置新密码。后端要求至少 10 位，重置后该账号的全部会话会立即失效。
          </p>
          <label className="block space-y-1.5">
            <span className="text-xs font-bold text-slate-700">新密码</span>
            <div className="flex gap-2">
              <Input
                type="text"
                value={resetPassword}
                onChange={(event) => setResetPassword(event.target.value)}
                className="font-mono"
              />
              <Button
                variant="outline"
                type="button"
                onClick={() => setResetPassword(generateInitialPassword())}
              >
                随机生成
              </Button>
            </div>
          </label>
          <p className="rounded-lg bg-amber-50 px-3 py-2 text-[11px] leading-5 text-amber-900">
            请通过线下安全渠道交给本人，不要写入公开群聊或代码仓库。
          </p>
        </div>
      </Dialog>

      <Dialog
        open={Boolean(editingUser)}
        onClose={() => setEditingUser(null)}
        title="编辑账号"
        eyebrow="Account profile"
        footer={
          <>
            <Button size="sm" variant="outline" onClick={() => setEditingUser(null)}>
              取消
            </Button>
            <Button size="sm" loading={editUserBusy} onClick={() => void handleSaveEditUser()}>
              保存
            </Button>
          </>
        }
      >
        {editingUser && (
          <div className="space-y-3 text-sm">
            <label className="block">
              <span className="mb-1 block text-xs font-semibold text-slate-700">姓名</span>
              <Input
                aria-label="编辑姓名"
                value={editUserForm.name}
                onChange={(e) => setEditUserForm({ ...editUserForm, name: e.target.value })}
              />
            </label>
            <label className="block">
              <span className="mb-1 block text-xs font-semibold text-slate-700">登录名</span>
              <Input
                aria-label="编辑登录名"
                value={editUserForm.username}
                onChange={(e) => setEditUserForm({ ...editUserForm, username: e.target.value })}
              />
            </label>
            <label className="block">
              <span className="mb-1 block text-xs font-semibold text-slate-700">省份</span>
              <Select
                aria-label="编辑省份"
                value={editUserForm.province}
                onChange={(e) => setEditUserForm({ ...editUserForm, province: e.target.value })}
                className="h-9 text-xs"
              >
                <option value="">未设置…</option>
                {provinces.map((p) => (
                  <option key={p.code} value={p.code} disabled={!p.enabled && p.code !== editUserForm.province}>
                    {p.code} · {p.name}
                    {p.enabled ? '' : '（已停用）'}
                  </option>
                ))}
              </Select>
              <span className="mt-1 block text-[11px] text-slate-400">
                已停用省份不可新选；当前省份保持不变不受影响。
              </span>
            </label>
            {isAdmin && (
              <label className="block">
                <span className="mb-1 block text-xs font-semibold text-slate-700">角色（仅管理员可改）</span>
                <Select
                  aria-label="编辑角色"
                  value={editUserForm.role}
                  onChange={(e) => setEditUserForm({ ...editUserForm, role: e.target.value })}
                  className="h-9 text-xs"
                >
                  <option value="student">student · 学生</option>
                  <option value="teacher">teacher · 教师</option>
                  <option value="admin">admin · 管理员</option>
                </Select>
              </label>
            )}
            {editingUser.examTicket && (
              <p className="rounded-lg bg-blue-50 px-3 py-2 text-[11px] text-blue-900">
                准考证号 <span className="font-mono font-bold">{editingUser.examTicket}</span> 已发放，随账号保留。
              </p>
            )}
          </div>
        )}
      </Dialog>

      <Dialog
        open={showBatchProvince}
        onClose={() => setShowBatchProvince(false)}
        title="批量设置省份"
        eyebrow="Batch province"
        footer={
          <>
            <Button size="sm" variant="outline" onClick={() => setShowBatchProvince(false)}>
              取消
            </Button>
            <Button
              size="sm"
              loading={batchProvinceBusy}
              disabled={!batchProvinceCode}
              onClick={() => void handleBatchSetProvince()}
            >
              应用到 {selectedUserIds.length} 名学生
            </Button>
          </>
        }
      >
        <div className="space-y-3 text-sm">
          <p className="text-xs leading-5 text-slate-600">
            将为选中的 {selectedUserIds.length} 名学生统一设置省份。省份编号将作为 CSP 准考证号前缀使用；
            已有准考证号的账号只改省份，号码保持不变。整个批次原子生效。
          </p>
          <Select
            aria-label="批量目标省份"
            value={batchProvinceCode}
            onChange={(e) => setBatchProvinceCode(e.target.value)}
            className="h-9 text-xs"
          >
            <option value="">选择省份…</option>
            {enabledProvinces.map((p) => (
              <option key={p.code} value={p.code}>
                {p.code} · {p.name}
              </option>
            ))}
          </Select>
        </div>
      </Dialog>

      <Dialog
        open={showSeatCardModal}
        onClose={() => setShowSeatCardModal(false)}
        title="智能准考证与机房座位卡"
        eyebrow="Exam credentials"
        width="lg"
        footer={
          <>
            <Button
              size="sm"
              variant="outline"
              onClick={() => window.print()}
              className="text-xs gap-1"
            >
              <Printer className="w-3.5 h-3.5" /> 打印当前名单座位卡
            </Button>
            {seatMode === 'prefix' ? (
              <Button
                size="sm"
                onClick={handleBatchGenerateExamUsers}
                disabled={isGeneratingUsers}
                className="text-xs gap-1 bg-amber-700 hover:bg-amber-800 text-white"
              >
                {isGeneratingUsers ? '正在生成...' : '立即批量创建账号'}
              </Button>
            ) : (
              <Button
                size="sm"
                onClick={() => void handleGenerateCSPTickets()}
                disabled={isGeneratingCSP}
                className="text-xs gap-1 bg-amber-700 hover:bg-amber-800 text-white"
              >
                {isGeneratingCSP ? '正在发号...' : selectedUserIds.length > 0 ? `给选中的 ${selectedUserIds.length} 人发号` : '批量创建考生并发号'}
              </Button>
            )}
          </>
        }
      >
        <div className="space-y-4">
          <div className="flex gap-2" role="radiogroup" aria-label="准考证号模式">
            <Button
              size="sm"
              variant="outline"
              type="button"
              aria-pressed={seatMode === 'csp'}
              onClick={() => setSeatMode('csp')}
              className={seatMode === 'csp' ? 'text-xs border-[#14283d] bg-[#14283d] text-white' : 'text-xs'}
            >
              CSP 准考证号（省份-组别+序号）
            </Button>
            <Button
              size="sm"
              variant="outline"
              type="button"
              aria-pressed={seatMode === 'prefix'}
              onClick={() => setSeatMode('prefix')}
              className={seatMode === 'prefix' ? 'text-xs border-[#14283d] bg-[#14283d] text-white' : 'text-xs'}
            >
              自定义前缀学号
            </Button>
          </div>

          {seatMode === 'csp' && (
            <div className="grid gap-3 rounded-xl border border-blue-200 bg-blue-50/60 p-3 text-xs sm:grid-cols-4">
              <div>
                <label className="mb-1 block font-semibold text-slate-700">省份</label>
                <Select
                  aria-label="CSP 省份"
                  value={cspProvince}
                  onChange={(e) => setCspProvince(e.target.value)}
                  className="h-8 bg-white text-xs"
                >
                  <option value="">选择省份…</option>
                  {enabledProvinces.map((p) => (
                    <option key={p.code} value={p.code}>
                      {p.code} · {p.name}
                    </option>
                  ))}
                </Select>
              </div>
              <div>
                <label className="mb-1 block font-semibold text-slate-700">组别</label>
                <Select
                  aria-label="CSP 组别"
                  value={cspGroup}
                  onChange={(e) => setCspGroup(e.target.value as 'J' | 'S')}
                  className="h-8 bg-white text-xs"
                >
                  <option value="J">J · 入门级（CSP-J）</option>
                  <option value="S">S · 提高级（CSP-S）</option>
                </Select>
              </div>
              <div>
                <label className="mb-1 block font-semibold text-slate-700">序号位数</label>
                <Select
                  aria-label="CSP 序号位数"
                  value={String(cspDigits)}
                  onChange={(e) => setCspDigits(parseInt(e.target.value) || 5)}
                  className="h-8 bg-white text-xs"
                >
                  {[3, 4, 5, 6, 7, 8].map((d) => (
                    <option key={d} value={d}>
                      {d} 位（{`${cspProvince || 'HA'}-${cspGroup}${'0'.repeat(Math.max(0, d - 1))}1`}）
                    </option>
                  ))}
                </Select>
              </div>
              <div>
                <label className="mb-1 block font-semibold text-slate-700">
                  {selectedUserIds.length > 0 ? '发号对象' : '考生人数'}
                </label>
                {selectedUserIds.length > 0 ? (
                  <p className="flex h-8 items-center rounded border border-slate-300 bg-white px-2 text-xs text-slate-600">
                    选中的 {selectedUserIds.length} 名学生
                  </p>
                ) : (
                  <Input
                    aria-label="CSP 考生人数"
                    type="number"
                    min={1}
                    max={500}
                    value={cspCount}
                    onChange={(e) => setCspCount(parseInt(e.target.value) || 1)}
                    className="h-8 bg-white text-xs"
                  />
                )}
              </div>
              <p className="sm:col-span-4 rounded-lg bg-white px-3 py-2 text-[11px] leading-5 text-slate-600">
                预览号段：<span className="font-mono font-bold text-blue-800">
                  {cspProvince || '省份'}-{cspGroup}{'0'.repeat(Math.max(0, cspDigits - 1))}1
                </span>
                ～ 按本省本组当前最大序号续发，永不重号；统一初始密码只作用于新建账号。
              </p>
            </div>
          )}

          <div className="grid gap-3 rounded-xl border border-amber-200 bg-amber-50/60 p-3 text-xs sm:grid-cols-3">
            {seatMode === 'prefix' && (
            <div>
              <label className="mb-1 block font-semibold text-slate-700">准考证号前缀</label>
              <Input
                aria-label="准考证号前缀"
                value={examUserPrefix}
                onChange={(e) => setExamUserPrefix(e.target.value)}
                placeholder="如: NOIP2026- 或 BJ-"
                className="h-8 bg-white text-xs"
              />
            </div>
            )}
            {seatMode === 'prefix' && (
            <div>
              <label className="mb-1 block font-semibold text-slate-700">考生人数</label>
              <Input
                aria-label="考生人数"
                type="number"
                min={1}
                max={500}
                value={examUserCount}
                onChange={(e) => setExamUserCount(parseInt(e.target.value) || 1)}
                className="h-8 bg-white text-xs"
              />
            </div>
            )}
            <div>
              <label className="mb-1 block font-semibold text-slate-700">
                统一初始密码（留空自动生成；仅新建账号生效）
              </label>
              <Input
                aria-label="初始默认密码"
                type="text"
                value={examUserPassword}
                onChange={(e) => setExamUserPassword(e.target.value)}
                placeholder="至少 10 位"
                className="h-8 bg-white text-xs"
              />
            </div>
          </div>

          <div className="space-y-2 rounded-xl border bg-slate-50 p-3">
            <span className="block text-[11px] font-bold uppercase text-slate-500">
              CCF 考场座位标签卡样式预览（A4 打印裁剪）:
            </span>
            <div className="grid gap-2 sm:grid-cols-2">
              {seatCardEntries.map((entry, i) => (
                <div
                  key={`${entry.username}-${i}`}
                  className="space-y-1 rounded-lg border border-dashed border-slate-300 bg-white p-2.5 text-xs"
                >
                  <div className="flex items-center justify-between">
                    <span className="font-mono font-black text-blue-800">
                      机位 #{i + 1}
                    </span>
                    <span className="rounded bg-slate-100 px-1 text-[10px] text-slate-500">
                      {currentContest?.name || '考场认证'}
                    </span>
                  </div>
                  <div className="font-semibold text-slate-800">
                    准考证号: {entry.ticket || entry.username}
                  </div>
                  <div className="font-mono text-[10px] text-slate-500">
                    姓名: {entry.name} · 登录: {entry.username} · 初始密码: {entry.password}
                  </div>
                </div>
              ))}
            </div>
          </div>
        </div>
      </Dialog>
    </div>
  )
}
