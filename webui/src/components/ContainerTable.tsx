import { useState } from 'react'
import { Skeleton, Alert } from '@mantine/core'
import { 
  IconAlertCircle, IconRefresh, IconSearch, 
  IconTerminal2, IconHistory, IconDeviceFloppy 
} from '@tabler/icons-react'
import type { Container } from '../types/api'
import { GlassCheck, StatusDot, SortHeader, type SortOrder } from './GlassUI'
import { UpdateBadge } from './UpdateBadge'

interface Props {
  containers: Container[]
  loading: boolean
  error: string | null
  onUpdate: (id: string, name: string) => void
  onLogs: (id: string, name: string) => void
  onRollbacks: (id: string, name: string) => void
  onSave: (id: string, name: string) => void
  onBulkUpdate: (containers: { id: string, name: string }[]) => void
}

export function ContainerTable({ containers, loading, error, onUpdate, onLogs, onRollbacks, onSave, onBulkUpdate }: Props) {
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [filter, setFilter] = useState('')
  const [sort, setSort] = useState<SortOrder>({ field: 'name', dir: 'asc' })

  const toggle = (id: string) =>
    setSelected(prev => {
      const next = new Set(prev)
      next.has(id) ? next.delete(id) : next.add(id)
      return next
    })

  const filtered = containers.filter(c => 
    c.name.toLowerCase().includes(filter.toLowerCase()) ||
    c.image.toLowerCase().includes(filter.toLowerCase())
  )

  filtered.sort((a, b) => {
    const av = a[sort.field as keyof Container]
    const bv = b[sort.field as keyof Container]
    if (av === bv) return 0
    const res = (av ?? '') < (bv ?? '') ? -1 : 1
    return sort.dir === 'asc' ? res : -res
  })

  const allSelected = filtered.length > 0 && filtered.every(c => selected.has(c.id))
  const someSelected = filtered.some(c => selected.has(c.id))

  const toggleAll = () =>
    setSelected(allSelected ? new Set() : new Set(filtered.map(c => c.id)))

  const selectedRefs = filtered.filter(c => selected.has(c.id)).map(c => ({ id: c.id, name: c.name }))

  const onSort = (field: string) => {
    setSort(prev => ({
      field,
      dir: prev.field === field && prev.dir === 'asc' ? 'desc' : 'asc'
    }))
  }

  if (error) {
    return (
      <Alert icon={<IconAlertCircle size={16} />} color="red" title="Error loading containers" variant="filled">
        {error}
      </Alert>
    )
  }

  return (
    <div className="col" style={{ gap: 16 }}>
      <div className="row" style={{ justifyContent: 'space-between' }}>
        <div className="row" style={{ position: 'relative' }}>
          <IconSearch size={14} className="muted" style={{ position: 'absolute', left: 10 }} />
          <input 
            className="search" 
            placeholder="Search standalone containers..." 
            value={filter} 
            onChange={e => setFilter(e.target.value)}
            style={{ paddingLeft: 32 }}
          />
        </div>
        
        {someSelected && (
          <div className="bulkbar">
            <span className="count">{selected.size} selected</span>
            <button className="btn small primary" onClick={() => { onBulkUpdate(selectedRefs); setSelected(new Set()) }}>
              <IconRefresh size={14} />
              Update selected
            </button>
            <button className="btn small ghost" onClick={() => setSelected(new Set())}>
              Clear
            </button>
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
              <SortHeader label="Container" field="name" sort={sort} onSort={onSort} />
              <SortHeader label="Image" field="image" sort={sort} onSort={onSort} />
              <SortHeader label="Status" field="state" sort={sort} onSort={onSort} />
              <th>Ports</th>
              <th className="col-actions">Actions</th>
            </tr>
          </thead>
          <tbody>
            {loading ? (
              Array.from({ length: 3 }).map((_, i) => (
                <tr key={i}>
                  {Array.from({ length: 6 }).map((_, j) => (
                    <td key={j}><Skeleton height={20} radius="sm" /></td>
                  ))}
                </tr>
              ))
            ) : filtered.length === 0 ? (
              <tr>
                <td colSpan={6} style={{ textAlign: 'center', padding: 40 }} className="muted">
                  {filter ? `No containers match "${filter}"` : 'No standalone containers found'}
                </td>
              </tr>
            ) : (
              filtered.map(c => {
                const ports = c.ports
                  .filter(p => p.host_port !== '0')
                  .map(p => `${p.host_port}:${p.container_port}`)
                  .join(', ')

                return (
                  <tr key={c.id} className={selected.has(c.id) ? 'selected' : ''} onClick={() => toggle(c.id)}>
                    <td className="col-check" onClick={e => e.stopPropagation()}>
                      <GlassCheck checked={selected.has(c.id)} onChange={() => toggle(c.id)} />
                    </td>
                    <td>
                      <span className="stack-name">
                        <StatusDot status={c.state === 'running' ? 'running' : 'stopped'} />
                        {c.name}
                      </span>
                    </td>
                    <td><span className="mono">{c.image}</span></td>
                    <td>
                      <div className="row">
                        <UpdateBadge updateAvailable={c.update_available} />
                      </div>
                    </td>
                    <td className="muted tiny">{ports || '—'}</td>
                    <td className="col-actions" onClick={e => e.stopPropagation()}>
                      <div className="row" style={{ justifyContent: 'flex-end' }}>
                        <button className="btn tiny primary" onClick={() => onUpdate(c.id, c.name)} disabled={c.state !== 'running'}>
                          <IconRefresh size={14} />
                          Update
                        </button>
                        <button className="btn tiny ghost" onClick={() => onLogs(c.id, c.name)}>
                          <IconTerminal2 size={14} />
                          Logs
                        </button>
                        <button className="btn tiny ghost" onClick={() => onRollbacks(c.id, c.name)}>
                          <IconHistory size={14} />
                          Rollback
                        </button>
                        <button className="btn tiny ghost" onClick={() => onSave(c.id, c.name)}>
                          <IconDeviceFloppy size={14} />
                          Save
                        </button>
                      </div>
                    </td>
                  </tr>
                )
              })
            )}
          </tbody>
        </table>
      </div>
    </div>
  )
}
