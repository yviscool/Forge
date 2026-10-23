import { useStore } from '../store/useStore'

export const dict: Record<string, Record<string, string>> = {
  'zh-CN': {
    brand: 'Forge 竞赛工坊',
    title: '比赛大厅',
    subtitle: '现代化信息学竞赛与评测教学平台',
    teacher: '教师控制台',
    student_lobby: '学生比赛大厅',
    problems: '试题列表',
    submissions: '提交流',
    ranking: '实时排名',
    submit_code: '提交评测',
    start_contest: '启动比赛',
    finish_contest: '结束比赛',
    reorder_problems: '试题排序',
    export_pdf: '导出复赛 A4 PDF',
    export_html: 'CCF 排版预览',
    user_management: '用户与分组',
    analytics_diff: '学情与代码演进',
    live_submissions: '实时提交流',
    contest_settings: '比赛设置',
    save: '保存',
    cancel: '取消',
    create: '创建',
    delete: '删除',
    edit: '编辑',
    view: '查看',
  },
  'en-US': {
    brand: 'Forge Arena',
    title: 'Contest Lobby',
    subtitle: 'Modern Competitive Programming & Evaluation Platform',
    teacher: 'Teacher Console',
    student_lobby: 'Student Lobby',
    problems: 'Problems',
    submissions: 'Submissions',
    ranking: 'Leaderboard',
    submit_code: 'Submit',
    start_contest: 'Start Contest',
    finish_contest: 'Finish Contest',
    reorder_problems: 'Reorder Problems',
    export_pdf: 'Export Contest PDF',
    export_html: 'CCF Print Preview',
    user_management: 'Users & Groups',
    analytics_diff: 'Analytics & Code Diff',
    live_submissions: 'Live Submissions',
    contest_settings: 'Contest Settings',
    save: 'Save',
    cancel: 'Cancel',
    create: 'Create',
    delete: 'Delete',
    edit: 'Edit',
    view: 'View',
  },
}

export function useLocale() {
  const { locale, toggleLocale, setLocale } = useStore()
  const t = (k: string) => dict[locale]?.[k] ?? k
  return { locale, t, toggleLocale, setLocale }
}
