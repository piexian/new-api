/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Bot, Plus, Send, Star } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Dialog } from '@/components/dialog'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { IconBadge } from '@/components/ui/icon-badge'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { Textarea } from '@/components/ui/textarea'
import { formatTimestamp } from '@/lib/format'
import { cn } from '@/lib/utils'

import {
  createLoanApplication,
  getLoanApplicationDetail,
  getLoanApplications,
  rateLoanApplication,
  replyLoanApplication,
} from '../api'
import {
  LOAN_TOPIC_KEYS,
  type LoanApplication,
  type LoanTopic,
} from '../types'

const PAGE_SIZE = 10

function useTopicLabel() {
  const { t } = useTranslation()
  return (topic: LoanTopic) => {
    if (topic === 'credit') return t('Credit Limit Increase')
    if (topic === 'rate') return t('Interest Rate Reduction')
    if (topic === 'grace') return t('Grace Period Extension')
    return t('Other')
  }
}

function StarRating({
  value,
  onChange,
  disabled,
}: {
  value: number
  onChange?: (value: number) => void
  disabled?: boolean
}) {
  const { t } = useTranslation()
  return (
    <div className='flex items-center gap-1'>
      {[1, 2, 3, 4, 5].map((star) => (
        <button
          key={star}
          type='button'
          disabled={disabled || !onChange}
          onClick={() => onChange?.(star)}
          aria-label={t('Rate {{count}} star', { count: star })}
          className={cn(
            'rounded p-0.5 outline-none focus-visible:ring-2',
            onChange && !disabled
              ? 'cursor-pointer'
              : 'cursor-default'
          )}
        >
          <Star
            className={cn(
              'h-5 w-5',
              star <= value
                ? 'fill-amber-400 text-amber-400'
                : 'text-muted-foreground/40'
            )}
          />
        </button>
      ))}
    </div>
  )
}

function NewApplicationDialog({
  open,
  onOpenChange,
  onCreated,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  onCreated: () => void
}) {
  const { t } = useTranslation()
  const topicLabel = useTopicLabel()
  const [topic, setTopic] = useState<LoanTopic>('credit')
  const [content, setContent] = useState('')
  const [submitting, setSubmitting] = useState(false)

  const handleSubmit = async () => {
    if (!content.trim()) {
      toast.error(t('Please describe your request'))
      return
    }
    setSubmitting(true)
    try {
      const res = await createLoanApplication(topic, content.trim())
      if (res.success) {
        toast.success(t('Application submitted'))
        setContent('')
        setTopic('credit')
        onOpenChange(false)
        onCreated()
        return
      }
      // 首轮 AI 对话失败时工单可能已创建：刷新列表并引导用户到详情继续，
      // 具体错误信息已由 api 拦截器弹出
      onCreated()
      toast.info(
        t(
          'If the application was created, open it from the list to continue the conversation.'
        )
      )
    } catch {
      toast.error(t('Failed to submit application'))
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title={t('New Application')}
      description={t(
        'Describe your request and the AI officer will review it with you.'
      )}
      contentClassName='sm:max-w-lg'
      bodyClassName='space-y-4'
      footer={
        <Button
          onClick={handleSubmit}
          disabled={submitting || !content.trim()}
          className='w-full sm:w-auto'
        >
          {submitting ? t('Submitting...') : t('Submit')}
        </Button>
      }
    >
      <div className='space-y-2'>
        <Label>{t('Topic')}</Label>
        <Select
          value={topic}
          onValueChange={(value) => {
            if (value !== null) setTopic(value as LoanTopic)
          }}
        >
          <SelectTrigger className='w-full'>
            <SelectValue />
          </SelectTrigger>
          <SelectContent alignItemWithTrigger={false}>
            <SelectGroup>
              {LOAN_TOPIC_KEYS.map((key) => (
                <SelectItem key={key} value={key}>
                  {topicLabel(key)}
                </SelectItem>
              ))}
            </SelectGroup>
          </SelectContent>
        </Select>
      </div>
      <div className='space-y-2'>
        <Label>{t('Request Details')}</Label>
        <Textarea
          value={content}
          onChange={(e) => setContent(e.target.value)}
          placeholder={t('Describe your request...')}
          rows={5}
        />
      </div>
    </Dialog>
  )
}

function ApplicationDetailDialog({
  applicationId,
  open,
  onOpenChange,
}: {
  applicationId: number | null
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const topicLabel = useTopicLabel()
  const [reply, setReply] = useState('')
  const [sending, setSending] = useState(false)
  const [rating, setRating] = useState(0)
  const [ratingComment, setRatingComment] = useState('')
  const [ratingSubmitting, setRatingSubmitting] = useState(false)

  const { data, isLoading } = useQuery({
    queryKey: ['loan-application-detail', applicationId],
    queryFn: async () => {
      const res = await getLoanApplicationDetail(applicationId as number)
      if (res.success && res.data) {
        return res.data
      }
      throw new Error(res.message || 'Failed to fetch application detail')
    },
    enabled: open && applicationId !== null,
    staleTime: 0,
  })

  const application = data?.application
  const messages = data?.messages ?? []
  const isOpen = application?.status === 'open'
  const showRatingWidget =
    application?.status === 'closed' && application.rating === 0

  const invalidate = () => {
    queryClient.invalidateQueries({
      queryKey: ['loan-application-detail', applicationId],
    })
    queryClient.invalidateQueries({ queryKey: ['loan-applications'] })
  }

  const handleSend = async () => {
    if (applicationId === null || !reply.trim()) return
    setSending(true)
    try {
      const res = await replyLoanApplication(applicationId, reply.trim())
      if (res.success) {
        setReply('')
        invalidate()
        return
      }
      // Backend message is already toasted by the api interceptor
    } catch {
      toast.error(t('Failed to send reply'))
    } finally {
      setSending(false)
    }
  }

  const handleRate = async () => {
    if (applicationId === null || rating < 1) return
    setRatingSubmitting(true)
    try {
      const res = await rateLoanApplication(
        applicationId,
        rating,
        ratingComment.trim()
      )
      if (res.success) {
        toast.success(t('Thanks for your feedback'))
        invalidate()
        return
      }
      // Backend message is already toasted by the api interceptor
    } catch {
      toast.error(t('Failed to submit rating'))
    } finally {
      setRatingSubmitting(false)
    }
  }

  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title={
        application
          ? `${topicLabel(application.topic)} #${application.id}`
          : t('Application Detail')
      }
      contentClassName='sm:max-w-2xl'
      bodyClassName='space-y-4'
    >
      {isLoading || !application ? (
        <div className='space-y-2'>
          {['a', 'b', 'c'].map((slot) => (
            <Skeleton key={slot} className='h-14 rounded-lg' />
          ))}
        </div>
      ) : (
        <>
          <div className='flex items-center gap-2'>
            <Badge variant={isOpen ? 'default' : 'secondary'}>
              {isOpen ? t('Open') : t('Closed')}
            </Badge>
            <span className='text-muted-foreground text-xs'>
              {formatTimestamp(application.created_at)}
            </span>
          </div>

          {/* 对话串 */}
          <div className='max-h-[min(50vh,420px)] space-y-3 overflow-y-auto rounded-lg border p-3'>
            {messages.length === 0 ? (
              <p className='text-muted-foreground py-6 text-center text-sm'>
                {t('No messages yet. Send a reply to start the conversation.')}
              </p>
            ) : (
              messages.map((msg) => {
                if (msg.role === 'system') {
                  return (
                    <p
                      key={msg.id}
                      className='text-muted-foreground text-center text-xs'
                    >
                      {msg.content}
                    </p>
                  )
                }
                const isUser = msg.role === 'user'
                return (
                  <div
                    key={msg.id}
                    className={cn(
                      'flex',
                      isUser ? 'justify-end' : 'justify-start'
                    )}
                  >
                    <div
                      className={cn(
                        'max-w-[85%] rounded-lg px-3 py-2 text-sm break-words whitespace-pre-wrap',
                        isUser
                          ? 'bg-primary text-primary-foreground'
                          : 'bg-muted'
                      )}
                    >
                      {msg.content}
                      <div
                        className={cn(
                          'mt-1 text-[10px]',
                          isUser
                            ? 'text-primary-foreground/70'
                            : 'text-muted-foreground'
                        )}
                      >
                        {formatTimestamp(msg.created_at)}
                      </div>
                    </div>
                  </div>
                )
              })
            )}
          </div>

          {/* open 工单可继续回复 */}
          {isOpen ? (
            <div className='flex items-end gap-2'>
              <Textarea
                value={reply}
                onChange={(e) => setReply(e.target.value)}
                placeholder={t('Type your reply...')}
                rows={2}
                className='min-h-0 flex-1'
              />
              <Button
                onClick={handleSend}
                disabled={sending || !reply.trim()}
                size='icon'
                aria-label={t('Send')}
              >
                <Send className='h-4 w-4' />
              </Button>
            </div>
          ) : null}

          {/* closed 且未评分时显示评分组件 */}
          {showRatingWidget ? (
            <div className='space-y-3 rounded-lg border p-3'>
              <p className='text-sm font-medium'>
                {t('How would you rate this service?')}
              </p>
              <StarRating value={rating} onChange={setRating} />
              <Textarea
                value={ratingComment}
                onChange={(e) => setRatingComment(e.target.value)}
                placeholder={t('Optional comment...')}
                rows={2}
              />
              <Button
                onClick={handleRate}
                disabled={ratingSubmitting || rating < 1}
                size='sm'
              >
                {ratingSubmitting ? t('Submitting...') : t('Submit Rating')}
              </Button>
            </div>
          ) : null}

          {!isOpen && application.rating > 0 ? (
            <div className='space-y-1 rounded-lg border p-3'>
              <p className='text-muted-foreground text-xs font-medium tracking-wider uppercase'>
                {t('Your Rating')}
              </p>
              <StarRating value={application.rating} disabled />
              {application.rating_comment ? (
                <p className='text-muted-foreground text-sm'>
                  {application.rating_comment}
                </p>
              ) : null}
            </div>
          ) : null}
        </>
      )}
    </Dialog>
  )
}

export function OfficerApplications() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const topicLabel = useTopicLabel()
  const [page, setPage] = useState(1)
  const [newDialogOpen, setNewDialogOpen] = useState(false)
  const [detailId, setDetailId] = useState<number | null>(null)

  const { data, isLoading } = useQuery({
    queryKey: ['loan-applications', page, PAGE_SIZE],
    queryFn: async () => {
      const res = await getLoanApplications(page, PAGE_SIZE)
      if (res.success && res.data) {
        return res.data
      }
      throw new Error(res.message || 'Failed to fetch loan applications')
    },
    staleTime: 10000,
  })

  const applications = data?.items ?? []
  const total = data?.total ?? 0
  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE))

  const refreshList = () => {
    queryClient.invalidateQueries({ queryKey: ['loan-applications'] })
  }

  const listContent = (() => {
    if (isLoading) {
      return (
        <div className='space-y-2'>
          {['a', 'b', 'c'].map((slot) => (
            <Skeleton key={slot} className='h-14 rounded-lg' />
          ))}
        </div>
      )
    }

    if (applications.length === 0) {
      return (
        <div className='text-muted-foreground flex min-h-32 items-center justify-center rounded-lg border border-dashed text-sm'>
          {t('No applications yet')}
        </div>
      )
    }

    return (
      <>
        <div className='space-y-2'>
          {applications.map((app: LoanApplication) => (
            <button
              key={app.id}
              type='button'
              onClick={() => setDetailId(app.id)}
              className='hover:bg-muted/50 grid w-full gap-2 rounded-lg border p-3 text-left transition-colors sm:grid-cols-[minmax(0,1fr)_auto] sm:items-center'
            >
              <div className='min-w-0'>
                <div className='flex flex-wrap items-center gap-2'>
                  <span className='text-sm font-medium'>
                    {topicLabel(app.topic)} #{app.id}
                  </span>
                  <Badge
                    variant={app.status === 'open' ? 'default' : 'secondary'}
                  >
                    {app.status === 'open' ? t('Open') : t('Closed')}
                  </Badge>
                </div>
                <p className='text-muted-foreground mt-0.5 text-xs'>
                  {formatTimestamp(app.created_at)}
                </p>
              </div>
              {app.rating > 0 ? (
                <div className='flex items-center gap-0.5'>
                  {[1, 2, 3, 4, 5]
                    .filter((star) => star <= app.rating)
                    .map((star) => (
                      <Star
                        key={star}
                        className='h-3.5 w-3.5 fill-amber-400 text-amber-400'
                      />
                    ))}
                </div>
              ) : null}
            </button>
          ))}
        </div>

        <div className='mt-4 flex flex-col items-center gap-3 border-t pt-4 sm:flex-row sm:items-center sm:justify-between'>
          <div className='text-muted-foreground text-xs sm:text-sm'>
            {t('Showing')} {(page - 1) * PAGE_SIZE + 1}-
            {Math.min(page * PAGE_SIZE, total)} {t('of')} {total}
          </div>
          <div className='flex items-center gap-2'>
            <Button
              variant='outline'
              size='sm'
              onClick={() => setPage((p) => p - 1)}
              disabled={page <= 1}
            >
              {t('Previous')}
            </Button>
            <div className='text-muted-foreground flex items-center gap-1 text-sm'>
              <span className='font-medium'>{page}</span>
              <span>/</span>
              <span>{totalPages}</span>
            </div>
            <Button
              variant='outline'
              size='sm'
              onClick={() => setPage((p) => p + 1)}
              disabled={page >= totalPages}
            >
              {t('Next')}
            </Button>
          </div>
        </div>
      </>
    )
  })()

  return (
    <>
      <Card className='gap-0 py-0'>
        <CardHeader className='border-b p-4 sm:p-5'>
          <div className='flex items-center justify-between gap-3'>
            <div className='flex items-center gap-3'>
              <IconBadge tone='neutral' size='lg'>
                <Bot className='h-4 w-4 sm:h-5 sm:w-5' strokeWidth={2} />
              </IconBadge>
              <div className='min-w-0'>
                <CardTitle className='text-lg'>{t('AI Loan Officer')}</CardTitle>
                <p className='text-muted-foreground mt-0.5 text-xs sm:text-sm'>
                  {t(
                    'Apply for a higher limit, a lower rate, or a grace period'
                  )}
                </p>
              </div>
            </div>
            <Button
              size='sm'
              onClick={() => setNewDialogOpen(true)}
              className='shrink-0'
            >
              <Plus className='h-4 w-4' />
              {t('New Application')}
            </Button>
          </div>
        </CardHeader>
        <CardContent className='p-4 sm:p-5'>{listContent}</CardContent>
      </Card>

      <NewApplicationDialog
        open={newDialogOpen}
        onOpenChange={setNewDialogOpen}
        onCreated={refreshList}
      />

      <ApplicationDetailDialog
        applicationId={detailId}
        open={detailId !== null}
        onOpenChange={(open) => {
          if (!open) setDetailId(null)
        }}
      />
    </>
  )
}
