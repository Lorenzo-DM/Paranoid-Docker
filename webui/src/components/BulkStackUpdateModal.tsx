import { useEffect, useState } from 'react'
import { Modal, Stack, Text, ScrollArea, Group, ThemeIcon, Box, Loader } from '@mantine/core'
import { IconCheck, IconX, IconMinus } from '@tabler/icons-react'
import { triggerStackUpdate, createStackUpdateEventSource } from '../api/containers'
import { getRollbackIncludeEnv } from '../settings'

type ItemStatus = 'pending' | 'running' | 'done' | 'error'

interface StackItem {
  name: string
  status: ItemStatus
  error?: string
}

interface Props {
  stackNames: string[]
  onClose: () => void
}

function StatusIcon({ status }: { status: ItemStatus }) {
  switch (status) {
    case 'done':    return <ThemeIcon size="xs" color="green" variant="light"  radius="xl"><IconCheck size={10} /></ThemeIcon>
    case 'error':   return <ThemeIcon size="xs" color="red"   variant="light"  radius="xl"><IconX     size={10} /></ThemeIcon>
    case 'running': return <Loader size="xs" />
    default:        return <ThemeIcon size="xs" color="gray"  variant="subtle" radius="xl"><IconMinus size={10} /></ThemeIcon>
  }
}

export function BulkStackUpdateModal({ stackNames, onClose }: Props) {
  const [activeNames, setActiveNames] = useState<string[]>([])
  const [items, setItems] = useState<StackItem[]>([])
  const [currentLogs, setCurrentLogs] = useState<string[]>([])
  const [currentIndex, setCurrentIndex] = useState(-1)
  const [finished, setFinished] = useState(false)

  const key = stackNames.join(',')

  useEffect(() => {
    if (stackNames.length === 0) return
    const names = [...stackNames]
    setActiveNames(names)
    setItems(names.map(name => ({ name, status: 'pending' })))
    setCurrentLogs([])
    setCurrentIndex(0)
    setFinished(false)
  }, [key])

  useEffect(() => {
    if (activeNames.length === 0 || currentIndex < 0) return
    if (currentIndex >= activeNames.length) {
      setFinished(true)
      return
    }

    const name = activeNames[currentIndex]
    setCurrentLogs([])
    setItems(prev => prev.map((s, i) => i === currentIndex ? { ...s, status: 'running' } : s))

    let es: EventSource | null = null

    triggerStackUpdate(name, getRollbackIncludeEnv())
      .then(() => {
        es = createStackUpdateEventSource(name)

        es.addEventListener('progress', (e) => {
          const evt = JSON.parse(e.data)
          if (evt.line) setCurrentLogs(prev => [...prev, evt.line])
        })

        es.addEventListener('done', () => {
          setItems(prev => prev.map((s, i) => i === currentIndex ? { ...s, status: 'done' } : s))
          es?.close()
          setCurrentIndex(i => i + 1)
        })

        es.addEventListener('error', (e) => {
          const evt = JSON.parse((e as MessageEvent).data ?? '{}')
          setItems(prev => prev.map((s, i) =>
            i === currentIndex ? { ...s, status: 'error', error: evt.error ?? 'Update failed' } : s
          ))
          es?.close()
          setCurrentIndex(i => i + 1)
        })
      })
      .catch(err => {
        setItems(prev => prev.map((s, i) =>
          i === currentIndex ? { ...s, status: 'error', error: err.message } : s
        ))
        setCurrentIndex(i => i + 1)
      })

    return () => es?.close()
  }, [currentIndex, activeNames])

  const completedCount = items.filter(s => s.status === 'done' || s.status === 'error').length
  const progress = activeNames.length > 0 ? Math.round((completedCount / activeNames.length) * 100) : 0

  return (
    <Modal
      opened={stackNames.length > 0}
      onClose={onClose}
      title={
        <Group gap="xs">
          <Text fw={600}>Bulk update</Text>
          <span className="pill mono">{activeNames.length} stacks</span>
        </Group>
      }
      size="lg"
      closeOnClickOutside={finished}
      closeOnEscape={finished}
    >
      <Stack>
        <div className="progress">
          <div style={{ width: `${progress}%`, transition: 'width 0.3s' }} />
        </div>

        <Box className="glass-inset" p="xs">
          {items.map(item => (
            <Group key={item.name} gap="xs" py={4} wrap="nowrap" style={{ borderBottom: '1px solid var(--line)', lastChild: { borderBottom: 'none' } }}>
              <StatusIcon status={item.status} />
              <Text size="sm" ff="monospace" flex={1} c="var(--ink-2)">{item.name}</Text>
              {item.error && <Text size="xs" c="var(--danger)" lineClamp={1}>{item.error}</Text>}
            </Group>
          ))}
        </Box>

        {currentLogs.length > 0 && (
          <ScrollArea h={100} type="auto" className="glass-inset" p="xs">
            {currentLogs.map((line, i) => (
              <Text key={i} size="xs" ff="monospace" c="var(--ink-3)">{line}</Text>
            ))}
          </ScrollArea>
        )}

        <Group justify="flex-end">
          <button className="btn" onClick={onClose} disabled={!finished}>
            {finished ? 'Close' : 'Running…'}
          </button>
        </Group>
      </Stack>
    </Modal>
  )
}
