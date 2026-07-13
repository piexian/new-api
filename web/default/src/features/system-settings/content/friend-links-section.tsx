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
import { useEffect, useState } from 'react'
import * as z from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { ArrowDown, ArrowUp, Edit, Plus, Save, Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'

type FriendLinkItem = {
  id: number
  name: string
  url: string
  icon: string
  description: string
  order: number
  enabled: boolean
}

type FriendLinksSectionProps = {
  enabled: boolean
  data: string
}

const friendLinkSchema = z.object({
  name: z.string().min(1, 'Name is required').max(100),
  url: z.string().url('Must be a valid URL').max(500),
  icon: z.string().max(500),
  description: z.string().max(200),
  order: z.number().int(),
  enabled: z.boolean(),
})

type FriendLinkFormValues = z.infer<typeof friendLinkSchema>

function isHttpIcon(icon?: string) {
  if (!icon) return false
  const v = icon.trim().toLowerCase()
  return v.startsWith('http://') || v.startsWith('https://')
}

function FriendLinkIconPreview(props: { icon?: string; name: string }) {
  const icon = (props.icon || '').trim()
  if (icon && isHttpIcon(icon)) {
    return <img src={icon} alt='' className='size-8 rounded object-cover' />
  }
  if (icon) {
    return (
      <div className='bg-muted flex size-8 items-center justify-center rounded text-base leading-none'>
        {icon}
      </div>
    )
  }
  return (
    <div className='bg-muted flex size-8 items-center justify-center rounded text-xs font-bold'>
      {(props.name || '?').slice(0, 1).toUpperCase()}
    </div>
  )
}

function normalizeFriendLinks(parsed: unknown[]): FriendLinkItem[] {
  const usedIds = new Set<number>()
  let nextId =
    Math.max(
      0,
      ...parsed.map((item) => {
        if (!item || typeof item !== 'object' || !('id' in item)) return 0
        return typeof item.id === 'number' && Number.isInteger(item.id)
          ? item.id
          : 0
      })
    ) + 1

  return parsed.map((item, idx) => {
    const row =
      item && typeof item === 'object' ? (item as Record<string, unknown>) : {}
    let id =
      typeof row.id === 'number' &&
      Number.isInteger(row.id) &&
      row.id > 0 &&
      !usedIds.has(row.id)
        ? row.id
        : 0
    while (id === 0 || usedIds.has(id)) id = nextId++
    usedIds.add(id)

    return {
      id,
      name: typeof row.name === 'string' ? row.name : '',
      url: typeof row.url === 'string' ? row.url : '',
      icon: typeof row.icon === 'string' ? row.icon : '',
      description: typeof row.description === 'string' ? row.description : '',
      order: typeof row.order === 'number' ? row.order : idx,
      enabled: row.enabled !== false,
    }
  })
}

export function FriendLinksSection({ enabled, data }: FriendLinksSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const [list, setList] = useState<FriendLinkItem[]>([])
  const [isEnabled, setIsEnabled] = useState(enabled)
  const [hasChanges, setHasChanges] = useState(false)
  const [selectedIds, setSelectedIds] = useState<number[]>([])
  const [showDialog, setShowDialog] = useState(false)
  const [showDeleteDialog, setShowDeleteDialog] = useState(false)
  const [editing, setEditing] = useState<FriendLinkItem | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<'single' | 'batch'>('single')

  const form = useForm<FriendLinkFormValues>({
    resolver: zodResolver(friendLinkSchema),
    defaultValues: {
      name: '',
      url: '',
      icon: '',
      description: '',
      order: 0,
      enabled: true,
    },
  })

  useEffect(() => {
    try {
      const parsed: unknown = JSON.parse(data || '[]')
      if (!Array.isArray(parsed)) {
        setList([])
        return
      }
      setList(normalizeFriendLinks(parsed))
    } catch {
      setList([])
    }
  }, [data])

  useEffect(() => {
    setIsEnabled(enabled)
  }, [enabled])

  const handleToggleEnabled = async (checked: boolean) => {
    try {
      await updateOption.mutateAsync({
        key: 'console_setting.friend_links_enabled',
        value: checked,
      })
      setIsEnabled(checked)
      toast.success(t('Setting saved'))
    } catch {
      toast.error(t('Failed to update setting'))
    }
  }

  const handleAdd = () => {
    const values = {
      name: '',
      url: '',
      icon: '',
      description: '',
      order: list.length,
      enabled: true,
    }
    setEditing(null)
    form.reset(values, { keepDefaultValues: false })
    setShowDialog(true)
    queueMicrotask(() => {
      form.reset(values, { keepDefaultValues: false })
    })
  }

  const handleEdit = (item: FriendLinkItem) => {
    const values = {
      name: item.name || '',
      url: item.url || '',
      icon: item.icon || '',
      description: item.description || '',
      order: typeof item.order === 'number' ? item.order : 0,
      enabled: item.enabled !== false,
    }
    setEditing(item)
    form.reset(values, { keepDefaultValues: false })
    setShowDialog(true)
    // Dialog 挂载后再次 reset，避免 RHF 空白默认值覆盖
    queueMicrotask(() => {
      form.reset(values, { keepDefaultValues: false })
    })
  }

  const handleDelete = (item: FriendLinkItem) => {
    setEditing(item)
    setDeleteTarget('single')
    setShowDeleteDialog(true)
  }

  const handleBatchDelete = () => {
    if (selectedIds.length === 0) {
      toast.error(t('Please select items to delete'))
      return
    }
    setDeleteTarget('batch')
    setShowDeleteDialog(true)
  }

  const confirmDelete = () => {
    if (deleteTarget === 'single' && editing) {
      setList((prev) => prev.filter((item) => item.id !== editing.id))
      setHasChanges(true)
      toast.success(t('Deleted. Click "Save Settings" to apply.'))
    } else if (deleteTarget === 'batch') {
      setList((prev) => prev.filter((item) => !selectedIds.includes(item.id)))
      setSelectedIds([])
      setHasChanges(true)
      toast.success(t('Deleted. Click "Save Settings" to apply.'))
    }
    setShowDeleteDialog(false)
    setEditing(null)
  }

  const handleSubmitForm = (values: FriendLinkFormValues) => {
    if (editing) {
      setList((prev) =>
        prev.map((item) =>
          item.id === editing.id
            ? {
                ...item,
                name: values.name,
                url: values.url,
                icon: values.icon || '',
                description: values.description || '',
                order: values.order,
                enabled: values.enabled,
              }
            : item
        )
      )
    } else {
      if (list.length >= 30) {
        toast.error(t('At most 30 friend links'))
        return
      }
      const newId = Math.max(...list.map((item) => item.id), 0) + 1
      setList((prev) => [
        ...prev,
        {
          id: newId,
          name: values.name,
          url: values.url,
          icon: values.icon || '',
          description: values.description || '',
          order: values.order,
          enabled: values.enabled,
        },
      ])
    }
    setHasChanges(true)
    setShowDialog(false)
    toast.success(t('Updated. Click "Save Settings" to apply.'))
  }

  const handleSaveAll = async () => {
    try {
      // 保留 id，便于编辑回填与稳定排序
      const payload = list
        .slice()
        .sort((a, b) => a.order - b.order)
        .map(({ id, ...rest }) => ({ id, ...rest }))
      const result = await updateOption.mutateAsync({
        key: 'console_setting.friend_links',
        value: JSON.stringify(payload),
      })
      if (result.success) setHasChanges(false)
    } catch {
      toast.error(t('Failed to save friend links'))
    }
  }

  const move = (id: number, dir: -1 | 1) => {
    setList((prev) => {
      const sorted = [...prev].sort((a, b) => a.order - b.order)
      const idx = sorted.findIndex((x) => x.id === id)
      const j = idx + dir
      if (idx < 0 || j < 0 || j >= sorted.length) return prev
      const a = sorted[idx]
      const b = sorted[j]
      const tmp = a.order
      a.order = b.order
      b.order = tmp
      return sorted
    })
    setHasChanges(true)
  }

  return (
    <SettingsSection
      title={t('Friend Links')}
      description={t('Configure floating ball friend links')}
    >
      <div className='space-y-4'>
        <div className='flex flex-wrap items-center justify-between gap-2'>
          <div className='flex items-center gap-2'>
            <Switch checked={isEnabled} onCheckedChange={handleToggleEnabled} />
            <span className='text-sm'>{t('Enable floating friend links')}</span>
          </div>
          <div className='flex flex-wrap gap-2'>
            <Button type='button' size='sm' onClick={handleAdd}>
              <Plus className='mr-1 size-4' />
              {t('Add')}
            </Button>
            <Button
              type='button'
              size='sm'
              variant='outline'
              onClick={handleBatchDelete}
            >
              <Trash2 className='mr-1 size-4' />
              {t('Delete')}
            </Button>
            <Button
              type='button'
              size='sm'
              variant='secondary'
              disabled={!hasChanges}
              onClick={handleSaveAll}
            >
              <Save className='mr-1 size-4' />
              {t('Save Settings')}
            </Button>
          </div>
        </div>

        <div className='rounded-md border'>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className='w-10'>
                  <Checkbox
                    checked={
                      list.length > 0 && selectedIds.length === list.length
                    }
                    onCheckedChange={(c) =>
                      setSelectedIds(c ? list.map((i) => i.id) : [])
                    }
                  />
                </TableHead>
                <TableHead>{t('Icon')}</TableHead>
                <TableHead>{t('Name')}</TableHead>
                <TableHead>{t('Description')}</TableHead>
                <TableHead>{t('URL')}</TableHead>
                <TableHead>{t('Order')}</TableHead>
                <TableHead>{t('Enabled')}</TableHead>
                <TableHead className='text-right'>{t('Actions')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {[...list]
                .sort((a, b) => a.order - b.order)
                .map((item) => (
                  <TableRow key={item.id}>
                    <TableCell>
                      <Checkbox
                        checked={selectedIds.includes(item.id)}
                        onCheckedChange={(c) =>
                          setSelectedIds((prev) =>
                            c
                              ? [...prev, item.id]
                              : prev.filter((id) => id !== item.id)
                          )
                        }
                      />
                    </TableCell>
                    <TableCell>
                      <FriendLinkIconPreview
                        icon={item.icon}
                        name={item.name}
                      />
                    </TableCell>
                    <TableCell className='font-medium'>{item.name}</TableCell>
                    <TableCell className='text-muted-foreground max-w-[160px] truncate text-sm'>
                      {item.description}
                    </TableCell>
                    <TableCell className='max-w-[180px] truncate text-xs'>
                      {item.url}
                    </TableCell>
                    <TableCell>{item.order}</TableCell>
                    <TableCell>{item.enabled ? t('Yes') : t('No')}</TableCell>
                    <TableCell className='space-x-1 text-right'>
                      <Button
                        type='button'
                        size='icon'
                        variant='ghost'
                        onClick={() => move(item.id, -1)}
                      >
                        <ArrowUp className='size-4' />
                      </Button>
                      <Button
                        type='button'
                        size='icon'
                        variant='ghost'
                        onClick={() => move(item.id, 1)}
                      >
                        <ArrowDown className='size-4' />
                      </Button>
                      <Button
                        type='button'
                        size='icon'
                        variant='ghost'
                        onClick={() => handleEdit(item)}
                      >
                        <Edit className='size-4' />
                      </Button>
                      <Button
                        type='button'
                        size='icon'
                        variant='ghost'
                        onClick={() => handleDelete(item)}
                      >
                        <Trash2 className='size-4' />
                      </Button>
                    </TableCell>
                  </TableRow>
                ))}
            </TableBody>
          </Table>
        </div>
      </div>

      <Dialog
        open={showDialog}
        onOpenChange={(open) => {
          setShowDialog(open)
          if (!open) {
            setEditing(null)
            form.reset({
              name: '',
              url: '',
              icon: '',
              description: '',
              order: list.length,
              enabled: true,
            })
          }
        }}
      >
        <DialogContent key={editing ? `edit-${editing.id}` : 'add'}>
          <DialogHeader>
            <DialogTitle>
              {editing ? t('Edit Friend Link') : t('Add Friend Link')}
            </DialogTitle>
            <DialogDescription>
              {t('name, url required; icon can be image URL or emoji')}
            </DialogDescription>
          </DialogHeader>
          <Form {...form}>
            <form
              className='space-y-3'
              onSubmit={form.handleSubmit(handleSubmitForm)}
            >
              <FormField
                control={form.control}
                name='name'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Name')}</FormLabel>
                    <FormControl>
                      <Input {...field} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='url'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>URL</FormLabel>
                    <FormControl>
                      <Input {...field} placeholder='https://' />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='icon'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Icon (URL or emoji)')}</FormLabel>
                    <FormControl>
                      <Input
                        {...field}
                        placeholder='https://.../icon.png  or  🤖'
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='description'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Description')}</FormLabel>
                    <FormControl>
                      <Input {...field} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='order'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Order')}</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        value={field.value}
                        onChange={(e) =>
                          field.onChange(Number(e.target.value || 0))
                        }
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='enabled'
                render={({ field }) => (
                  <FormItem className='flex items-center justify-between rounded-lg border p-3'>
                    <FormLabel>{t('Enabled')}</FormLabel>
                    <FormControl>
                      <Switch
                        checked={field.value}
                        onCheckedChange={field.onChange}
                      />
                    </FormControl>
                  </FormItem>
                )}
              />
              <DialogFooter>
                <Button type='submit'>{t('Apply')}</Button>
              </DialogFooter>
            </form>
          </Form>
        </DialogContent>
      </Dialog>

      <AlertDialog open={showDeleteDialog} onOpenChange={setShowDeleteDialog}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('Confirm delete')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('This will mark items deleted until you save settings.')}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('Cancel')}</AlertDialogCancel>
            <AlertDialogAction onClick={confirmDelete}>
              {t('Delete')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </SettingsSection>
  )
}
