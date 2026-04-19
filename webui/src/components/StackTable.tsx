import { useState, useMemo } from 'react'
import { Menu, Collapse, Box, Skeleton, Alert, Stack } from '@mantine/core'
import {
  IconRefresh, IconTerminal2, IconHistory,
  IconChevronDown, IconChevronRight, IconAlertCircle,
  IconFile, IconSearch, IconDeviceFloppy, IconCamera,
  IconCheck, IconX, IconDots
} from '@tabler/icons-react'
import { triggerStackSnapshot } from '../api/containers'
import { getRollbackIncludeEnv } from '../settings'
import type { ComposeStack } from '../types/api'
import { UpdateBadge } from './UpdateBadge'
import { RollbackModeBadge } from './RollbackModeBadge'
import { ServicesTable } from './ServicesTable'
import { GlassCheck, StatusDot, SortHeader, type SortOrder } from './GlassUI'

interface Props {
  stacks: ComposeStack[]
  loading: boolean
  error: string | null
  onUpdate: (name: string) => void
  onLogs: (name: string) => void
  onRollbacks: (name: string) => void
  onSave: (name: string) => void
  onSaveAndUpdate: (name: string) => void
  onBulkUpdate: (names: string[]) => void
}

type SnapState = 'idle' | 'loading' | 'ok' | 'error'

function StackRow({
  stack, selected, onToggle,
  onUpdate, onLogs, onRollbacks, onSave, onSaveAndUpdate,
}: {
  stack: ComposeStack
  selected: boolean
  onToggle: () => void
  onUpdate: (name: string) => void
  onLogs: (name: string) => void
  onRollbacks: (name: string) => void
  onSave: (name: string) => void
  onSaveAndUpdate: (name: string) => void
}) {
  const [expanded, setExpanded] = useState(false)
  const [snap, setSnap] = useState<SnapState>('idle')

  const handleSnapshot = () => {
    setSnap('loading')
    triggerStackSnapshot(stack.name, getRollbackIncludeEnv())
      .then(() => { setSnap('ok'); setTimeout(() => setSnap('idle'), 2500) })
      .catch(() => { setSnap('error'); setTimeout(() => setSnap('idle'), 2500) })
  }

  const saveIcon = snap === 'ok' ? <IconCheck size={13} /> : snap === 'error' ? <IconX size={13} /> : <IconDeviceFloppy size={13} />
  const saveLabel = snap === 'ok' ? 'Saved!' : snap === 'error' ? 'Error' : 'Save'
  const saveCls = `btn tiny ghost${snap === 'ok' ? ' success' : snap === 'error' ? ' danger' : ''}`

  return (
    <>
      <tr className={selected ? 'selected' : ''} onClick={onToggle}>
        <td className="col-check" onClick={e => e.stopPropagation()}>
          <GlassCheck checked={selected} onChange={onToggle} />
        </td>
        <td className="col-expand">
          <button className="btn tiny ghost" onClick={e => { e.stopPropagation(); setExpanded(v => !v) }}>
            {expanded ? <IconChevronDown size={14} /> : <IconChevronRight size={14} />}
          </button>
        </td>
        <td>
          <span className="stack-name">
            <StatusDot status={stack.status} />
            {stack.name}
            <RollbackModeBadge mode={stack.rollback_mode} size="xs" />
          </span>
        </td>
        <td><UpdateBadge updateAvailable={stack.update_available} /></td>
        <td className="muted">{stack.services.length} services</td>
        <td>
          <span className="mono" title={stack.config_files?.[0]}>{stack.config_files?.[0] ?? '—'}</span>
        </td>

        <td className="col-actions" onClick={e => e.stopPropagation()}>
          <div className="row justify-end gap-xs">

            <div className="split-btn">
              <button className="btn tiny primary" onClick={() => onUpdate(stack.name)}>
                <IconRefresh size={13} /> Update
              </button>
              <Menu position="bottom-end" withinPortal>
                <Menu.Target>
                  <button className="btn tiny primary"><IconChevronDown size={11} /></button>
                </Menu.Target>
                <Menu.Dropdown>
                  <Menu.Item leftSection={<IconRefresh size={14} />} onClick={() => onUpdate(stack.name)}>
                    Update
                  </Menu.Item>
                  <Menu.Divider />
                  <Menu.Item
                    leftSection={<><IconDeviceFloppy size={12} className="icon-mr-xs" /><IconRefresh size={12} /></>}
                    onClick={() => onSaveAndUpdate(stack.name)}
                  >
                    Save + Update
                  </Menu.Item>
                </Menu.Dropdown>
              </Menu>
            </div>

            <div className="split-btn">
              <button className={saveCls} onClick={() => onSave(stack.name)} disabled={snap === 'loading'}>
                {saveIcon} {saveLabel}
              </button>
              <Menu position="bottom-end" withinPortal>
                <Menu.Target>
                  <button className="btn tiny ghost"><IconChevronDown size={11} /></button>
                </Menu.Target>
                <Menu.Dropdown>
                  <Menu.Item leftSection={<IconDeviceFloppy size={14} />} onClick={() => onSave(stack.name)}>
                    Save images
                  </Menu.Item>
                  <Menu.Item leftSection={<IconCamera size={14} />} onClick={handleSnapshot} disabled={snap === 'loading'}>
                    Snapshot
                  </Menu.Item>
                </Menu.Dropdown>
              </Menu>
            </div>

            <button className="btn tiny ghost" onClick={() => onLogs(stack.name)}>
              <IconTerminal2 size={13} /> Logs
            </button>

            <Menu position="bottom-end" withinPortal>
              <Menu.Target>
                <button className="btn tiny ghost"><IconDots size={14} /></button>
              </Menu.Target>
              <Menu.Dropdown>
                <Menu.Item leftSection={<IconHistory size={14} />} onClick={() => onRollbacks(stack.name)}>
                  View rollbacks
                </Menu.Item>
              </Menu.Dropdown>
            </Menu>

          </div>
        </td>
      </tr>

      {expanded && (
        <tr>
          <td colSpan={7} className="expand-cell">
            <Collapse expanded={expanded}>
              <Box p="md" className="expand-content">
                <Stack gap="xs">
                  <ServicesTable services={stack.services} />
                  {stack.config_files?.length > 1 && (
                    <div className="row">
                      <IconFile size={12} className="muted" />
                      {stack.config_files.slice(1).map(f => (
                        <span key={f} className="mono tiny muted">{f}</span>
                      ))}
                    </div>
                  )}
                </Stack>
              </Box>
            </Collapse>
          </td>
        </tr>
      )}
    </>
  )
}

export function StackTable({ stacks, loading, error, onUpdate, onLogs, onRollbacks, onSave, onSaveAndUpdate, onBulkUpdate }: Props) {
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [filter, setFilter] = useState('')
  const [sort, setSort] = useState<SortOrder>({ field: 'name', dir: 'asc' })

  const toggle = (name: string) =>
    setSelected(prev => {
      const next = new Set(prev)
      next.has(name) ? next.delete(name) : next.add(name)
      return next
    })

  const filteredStacks = useMemo(() => {
    const result = stacks.filter(s =>
      s.name.toLowerCase().includes(filter.toLowerCase()) ||
      s.services.some(svc => svc.name.toLowerCase().includes(filter.toLowerCase()))
    )
    result.sort((a, b) => {
      const av = a[sort.field as keyof ComposeStack]
      const bv = b[sort.field as keyof ComposeStack]
      if (av === bv) return 0
      const res = (av ?? '') < (bv ?? '') ? -1 : 1
      return sort.dir === 'asc' ? res : -res
    })
    return result
  }, [stacks, filter, sort])

  const allSelected = filteredStacks.length > 0 && filteredStacks.every(s => selected.has(s.name))
  const someSelected = filteredStacks.some(s => selected.has(s.name))
  const toggleAll = () => setSelected(allSelected ? new Set() : new Set(filteredStacks.map(s => s.name)))
  const selectedNames = filteredStacks.filter(s => selected.has(s.name)).map(s => s.name)
  const onSort = (field: string) =>
    setSort(prev => ({ field, dir: prev.field === field && prev.dir === 'asc' ? 'desc' : 'asc' }))

  if (error) {
    return (
      <Alert icon={<IconAlertCircle size={16} />} color="red" title="Error loading stacks" variant="filled">
        {error}
      </Alert>
    )
  }

  return (
    <div className="col gap-lg">
      <div className="row justify-between">
        <div className="row search-wrap">
          <IconSearch size={14} className="muted search-icon" />
          <input
            className="search search-with-icon"
            placeholder="Search stacks or services..."
            value={filter}
            onChange={e => setFilter(e.target.value)}
          />
        </div>
        {someSelected && (
          <div className="bulkbar">
            <span className="count">{selected.size} selected</span>
            <button className="btn small primary" onClick={() => { onBulkUpdate(selectedNames); setSelected(new Set()) }}>
              <IconRefresh size={14} /> Update selected
            </button>
            <button className="btn small ghost" onClick={() => setSelected(new Set())}>Clear</button>
          </div>
        )}
      </div>

      <div className="tbl-shell">
        <table className="wtable">
          <thead>
            <tr>
              <th className="col-check">
                <GlassCheck checked={allSelected} indeterminate={someSelected && !allSelected} onChange={toggleAll} />
              </th>
              <th className="col-expand" />
              <SortHeader label="Stack" field="name" sort={sort} onSort={onSort} />
              <SortHeader label="Update" field="update_available" sort={sort} onSort={onSort} />
              <th>Services</th>
              <th>Compose file</th>
              <th className="col-actions">Actions</th>
            </tr>
          </thead>
          <tbody>
            {loading ? (
              Array.from({ length: 3 }).map((_, i) => (
                <tr key={i}>{Array.from({ length: 7 }).map((_, j) => <td key={j}><Skeleton height={20} radius="sm" /></td>)}</tr>
              ))
            ) : filteredStacks.length === 0 ? (
              <tr>
                <td colSpan={7} className="muted empty-cell">
                  {filter ? `No stacks match "${filter}"` : 'No compose stacks found'}
                </td>
              </tr>
            ) : (
              filteredStacks.map(stack => (
                <StackRow
                  key={stack.name}
                  stack={stack}
                  selected={selected.has(stack.name)}
                  onToggle={() => toggle(stack.name)}
                  onUpdate={onUpdate}
                  onLogs={onLogs}
                  onRollbacks={onRollbacks}
                  onSave={onSave}
                  onSaveAndUpdate={onSaveAndUpdate}
                />
              ))
            )}
          </tbody>
        </table>
      </div>
    </div>
  )
}
