import { useState } from 'react'
import { MapPin, ShieldAlert } from 'lucide-react'
import type { ProvinceView } from '../api/client'
import { api } from '../api/client'
import { Badge } from './ui/badge'
import { Button } from './ui/button'
import { Card, CardContent, CardHeader, CardTitle } from './ui/card'
import { useToast } from './ui/toast'

const errorText = (cause: unknown) =>
  cause instanceof Error ? cause.message : '服务暂时不可用，请稍后重试。'

interface ProvincesPanelProps {
  provinces: ProvinceView[]
  loading: boolean
  error: unknown
  refetch: () => void
  /** 停用/启用属考务策略，仅管理员可见可操作。 */
  isAdmin: boolean
}

/**
 * 内置省份表：编号 / 名称 / 排序 / 用户数 / 状态 / 操作。
 * 编号是 CSP 准考证号前缀（HA-J00001 的 HA）；停用省份只阻止新分配，
 * 已关联用户的既有准考证号不受影响。
 */
export function ProvincesPanel({ provinces, loading, error, refetch, isAdmin }: ProvincesPanelProps) {
  const toast = useToast()
  const [busyCode, setBusyCode] = useState('')

  const toggle = async (p: ProvinceView) => {
    setBusyCode(p.code)
    try {
      await api.setProvinceEnabled(p.code, !p.enabled)
      refetch()
      toast.success(
        p.enabled ? `已停用 ${p.name}` : `已启用 ${p.name}`,
        p.enabled
          ? '新用户不能再关联该省份，既有准考证号不受影响。'
          : '该省份恢复可分配。'
      )
    } catch (cause) {
      toast.error('省份状态更新失败', errorText(cause))
    } finally {
      setBusyCode('')
    }
  }

  if (loading) {
    return (
      <Card>
        <CardContent className="py-8 text-center text-xs text-slate-400">
          正在加载内置省份表…
        </CardContent>
      </Card>
    )
  }
  if (error) {
    return (
      <Card>
        <CardContent className="space-y-2 py-8 text-center text-xs text-slate-500">
          <ShieldAlert className="mx-auto h-5 w-5 text-amber-500" aria-hidden="true" />
          <p>省份表加载失败。</p>
          <Button size="sm" variant="outline" onClick={refetch} className="text-xs">
            重试
          </Button>
        </CardContent>
      </Card>
    )
  }

  const linked = provinces.filter((p) => p.users > 0).length

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between pb-3">
        <CardTitle className="flex items-center gap-2 text-sm">
          <MapPin className="h-4 w-4 text-slate-400" aria-hidden="true" />
          内置省份表 ({provinces.length} 个行政区，{linked} 个已关联考生)
        </CardTitle>
        <Badge variant="default">CSP 准考证号来源</Badge>
      </CardHeader>
      <CardContent>
        <p className="mb-3 text-xs leading-5 text-slate-500">
          准考证号格式：<span className="font-mono font-semibold text-slate-700">省份编号-组别+定长序号</span>
          （如 HA-J00001、GD-S00042）。省份编号即本表「编号」列；停用后编号保留但不再分配新号。
        </p>
        <div className="max-h-[26rem] overflow-y-auto rounded-lg border border-slate-200">
          <table className="w-full text-left text-xs">
            <thead className="sticky top-0 bg-slate-50 font-semibold text-slate-600">
              <tr>
                <th scope="col" className="px-3 py-2">编号</th>
                <th scope="col" className="px-3 py-2">名称</th>
                <th scope="col" className="px-3 py-2">排序</th>
                <th scope="col" className="px-3 py-2">用户数</th>
                <th scope="col" className="px-3 py-2">状态</th>
                {isAdmin && <th scope="col" className="px-3 py-2 text-right">操作</th>}
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100">
              {provinces.map((p) => (
                <tr key={p.code} className="hover:bg-slate-50">
                  <td className="px-3 py-2 font-mono font-semibold text-slate-800">{p.code}</td>
                  <td className="px-3 py-2 text-slate-700">{p.name}</td>
                  <td className="px-3 py-2 text-slate-500">{p.sort}</td>
                  <td className="px-3 py-2 text-slate-600">{p.users}</td>
                  <td className="px-3 py-2">
                    <span
                      className={
                        p.enabled
                          ? 'inline-flex rounded border border-emerald-200 bg-emerald-50 px-1.5 py-0.5 text-[10px] font-semibold text-emerald-700'
                          : 'inline-flex rounded border border-slate-200 bg-slate-100 px-1.5 py-0.5 text-[10px] font-semibold text-slate-600'
                      }
                    >
                      {p.enabled ? '启用' : '停用'}
                    </span>
                  </td>
                  {isAdmin && (
                    <td className="px-3 py-2 text-right">
                      <Button
                        size="sm"
                        variant="ghost"
                        loading={busyCode === p.code}
                        disabled={busyCode === p.code}
                        onClick={() => void toggle(p)}
                        aria-label={`${p.enabled ? '停用' : '启用'}省份${p.name}`}
                        className={
                          p.enabled
                            ? 'h-7 px-2 text-[10px] font-semibold text-rose-700 hover:bg-rose-100'
                            : 'h-7 px-2 text-[10px] font-semibold text-emerald-700 hover:bg-emerald-100'
                        }
                      >
                        {p.enabled ? '停用' : '启用'}
                      </Button>
                    </td>
                  )}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </CardContent>
    </Card>
  )
}
