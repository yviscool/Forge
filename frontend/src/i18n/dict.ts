import { ref } from 'vue'

const dict: Record<string, Record<string, string>> = {
  'zh-CN': {
    brand: 'Forge 竞赛工坊',
    title: '比赛大厅',
    subtitle: '实时竞赛与评测排名系统',
    teacher: '教师控制台',
    problems: '试题列表',
    submissions: '提交流',
    ranking: '实时排名',
    submit_code: '提交评测',
    start_contest: '启动比赛',
    finish_contest: '结束比赛',
  },
  'en-US': {
    brand: 'Forge Arena',
    title: 'Contest Lobby',
    subtitle: 'Real-Time Competition & Evaluation Platform',
    teacher: 'Teacher Console',
    problems: 'Problems',
    submissions: 'Submissions',
    ranking: 'Leaderboard',
    submit_code: 'Submit',
    start_contest: 'Start Contest',
    finish_contest: 'Finish Contest',
  },
}

const locale = ref(localStorage.getItem('forge_locale') || 'zh-CN')

export function useLocale() {
  const t = (k: string) => dict[locale.value]?.[k] ?? k
  const toggle = () => {
    locale.value = locale.value === 'zh-CN' ? 'en-US' : 'zh-CN'
    localStorage.setItem('forge_locale', locale.value)
  }
  return { locale, t, toggle }
}
