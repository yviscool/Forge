import React, { useState } from 'react'
import { Megaphone, ShieldAlert, X } from 'lucide-react'
import type { BroadcastMessage } from '../api/client'
import { useStore } from '../store/useStore'
import { Badge } from './ui/badge'
import { Button } from './ui/button'
import { Dialog } from './ui/dialog'

interface BroadcastBannerProps {
  notice?: string
  messages?: BroadcastMessage[]
}

const levelLabel: Record<BroadcastMessage['level'], string> = {
  info: '考场通知',
  warning: '重要提醒',
  urgent: '紧急更正',
}

export const BroadcastBanner: React.FC<BroadcastBannerProps> = ({
  notice,
  messages = [],
}) => {
  const { activeAlert, setActiveAlert } = useStore()
  const [historyOpen, setHistoryOpen] = useState(false)

  return (
    <>
      {notice ? (
        <div
          role="status"
          className="flex flex-wrap items-center gap-2 rounded-xl border border-amber-200 bg-amber-50/80 px-3.5 py-2.5 text-xs text-amber-950"
        >
          <Megaphone className="h-3.5 w-3.5 shrink-0 text-amber-600" />
          <span className="shrink-0 font-extrabold">考场置顶公告</span>
          <span className="min-w-0 flex-1 truncate font-medium">{notice}</span>
          <button
            type="button"
            onClick={() => setHistoryOpen(true)}
            className="shrink-0 font-bold text-amber-700 underline-offset-2 hover:underline"
          >
            查看通知（{messages.length}）
          </button>
        </div>
      ) : null}

      <Dialog
        open={Boolean(activeAlert)}
        onClose={() => setActiveAlert(null)}
        title={activeAlert?.title || '考场即时广播'}
        eyebrow={activeAlert ? levelLabel[activeAlert.level] : undefined}
        width="sm"
        footer={
          <Button
            variant={activeAlert?.level === 'urgent' ? 'danger' : 'secondary'}
            onClick={() => setActiveAlert(null)}
          >
            我已知晓
          </Button>
        }
      >
        {activeAlert ? (
          <div className="space-y-4">
            <div className="flex items-center gap-2">
              <ShieldAlert
                className={
                  activeAlert.level === 'urgent'
                    ? 'h-4 w-4 text-rose-600'
                    : 'h-4 w-4 text-amber-600'
                }
              />
              <Badge
                variant={
                  activeAlert.level === 'urgent'
                    ? 'danger'
                    : activeAlert.level === 'warning'
                      ? 'warning'
                      : 'info'
                }
              >
                {levelLabel[activeAlert.level]}
              </Badge>
            </div>
            <p className="whitespace-pre-wrap text-sm leading-6 text-slate-700">
              {activeAlert.content}
            </p>
            <p className="text-[11px] text-slate-400">
              发布人 · {activeAlert.senderName} ·{' '}
              {new Date(activeAlert.createdAt).toLocaleString('zh-CN')}
            </p>
          </div>
        ) : null}
      </Dialog>

      <Dialog
        open={historyOpen}
        onClose={() => setHistoryOpen(false)}
        title="考场通知"
        eyebrow="Announcements"
        width="md"
        footer={
          <Button variant="secondary" onClick={() => setHistoryOpen(false)}>
            关闭
          </Button>
        }
      >
        <div className="space-y-3">
          {notice ? (
            <article className="rounded-xl border border-amber-200 bg-amber-50/70 p-4">
              <p className="text-[10px] font-extrabold uppercase tracking-[0.16em] text-amber-700">
                置顶公告
              </p>
              <p className="mt-2 whitespace-pre-wrap text-sm leading-6 text-amber-950">
                {notice}
              </p>
            </article>
          ) : null}
          {messages.map((message) => (
            <article
              key={message.id}
              className="rounded-xl border border-[#e3e8ee] bg-white p-4"
            >
              <div className="flex flex-wrap items-center justify-between gap-2">
                <div className="flex items-center gap-2">
                  <p className="text-sm font-extrabold text-[#0f2233]">
                    {message.title || '考场通知'}
                  </p>
                  <Badge
                    variant={
                      message.level === 'urgent'
                        ? 'danger'
                        : message.level === 'warning'
                          ? 'warning'
                          : 'info'
                    }
                  >
                    {levelLabel[message.level]}
                  </Badge>
                </div>
                <span className="font-mono text-[10px] text-slate-400">
                  {new Date(message.createdAt).toLocaleTimeString('zh-CN')}
                </span>
              </div>
              <p className="mt-2 whitespace-pre-wrap text-sm leading-6 text-slate-600">
                {message.content}
              </p>
              <p className="mt-2 text-[11px] text-slate-400">
                发布人 · {message.senderName}
              </p>
            </article>
          ))}
          {!notice && messages.length === 0 ? (
            <p className="py-8 text-center text-sm text-slate-400">暂无考场通知。</p>
          ) : null}
        </div>
      </Dialog>
    </>
  )
}

export function DismissBroadcastButton() {
  const setActiveAlert = useStore((state) => state.setActiveAlert)
  return (
    <button
      type="button"
      aria-label="关闭广播"
      onClick={() => setActiveAlert(null)}
      className="rounded-lg p-1 text-slate-400 hover:bg-slate-100 hover:text-slate-700"
    >
      <X className="h-4 w-4" />
    </button>
  )
}
