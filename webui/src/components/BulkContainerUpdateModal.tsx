import { useEffect, useState } from 'react'
import { Modal, Stack, Text, Progress, ScrollArea, Group, ThemeIcon, Button, Box, Loader } from '@mantine/core'
import { IconCheck, IconX, IconMinus } from '@tabler/icons-react'
import { triggerUpdate, createPullStatusEventSource } from '../api/containers'
import { getRollbackIncludeEnv } from '../settings'

type ItemStatus = 'pending' | 'running' | 'done' | 'error'

interface ContainerItem {
  id: string
  name: string
  status: ItemStatus
  error?: string
}

export interface ContainerRef {
  id: string
  name: string
}

interface Props {
  containers: ContainerRef[]
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

export function BulkContainerUpdateModal({ containers, onClose }: Props) {
  const [active, setActive] = useState<ContainerRef[]>([])
  const [items, setItems] = useState<ContainerItem[]>([])
  const [currentLogs, setCurrentLogs] = useState<string[]>([])
  const [currentIndex, setCurrentIndex] = useState(-1)
  const [finished, setFinished] = useState(false)

  const key = containers.map(c => c.id).join(',')

  useEffect(() => {
    if (containers.length === 0) return
    const refs = [...containers]
    setActive(refs)
    setItems(refs.map(c => ({ ...c, status: 'pending' })))
    setCurrentLogs([])
    setCurrentIndex(0)
    setFinished(false)
  }, [key]) // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    if (active.length === 0 || currentIndex < 0) return
    if (currentIndex >= active.length) {
      setFinished(true)
      return
    }

    const { id } = active[currentIndex]
    setCurrentLogs([])
    setItems(prev => prev.map((s, i) => i === currentIndex ? { ...s, status: 'running' } : s))

    let es: EventSource | null = null

    triggerUpdate(id, getRollbackIncludeEnv())
      .then(() => {
        es = createPullStatusEventSource(id)

        es.addEventListener('progress', (e) => {
          const evt = JSON.parse(e.data)
          const text = [evt.id, evt.status, evt.progress].filter(Boolean).join(' ')
          if (text) setCurrentLogs(prev => [...prev, text])
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
  }, [currentIndex, active]) // eslint-disable-line react-hooks/exhaustive-deps

  const completedCount = items.filter(s => s.status === 'done' || s.status === 'error').length
  const progress = active.length > 0 ? Math.round((completedCount / active.length) * 100) : 0
  const hasErrors = items.some(s => s.status === 'error')

  return (
    <Modal
      opened={containers.length > 0}
      onClose={onClose}
      title={`Bulk update — ${active.length} container${active.length !== 1 ? 's' : ''}`}
      size="lg"
      closeOnClickOutside={finished}
      closeOnEscape={finished}
    >
      <Stack>
        <Progress
          value={progress}
          animated={!finished}
          color={finished ? (hasErrors ? 'orange' : 'green') : 'blue'}
        />

        <Box>
          {items.map(item => (
            <Group key={item.id} gap="xs" py={4} wrap="nowrap">
              <StatusIcon status={item.status} />
              <Text size="sm" ff="monospace" flex={1}>{item.name}</Text>
              {item.error && <Text size="xs" c="red" lineClamp={1}>{item.error}</Text>}
            </Group>
          ))}
        </Box>

        {currentLogs.length > 0 && (
          <ScrollArea h={100} type="auto" style={{ background: 'var(--mantine-color-dark-8)', borderRadius: 4 }}>
            <Box p="xs">
              {currentLogs.map((line, i) => (
                <Text key={i} size="xs" ff="monospace" c="dimmed">{line}</Text>
              ))}
            </Box>
          </ScrollArea>
        )}

        <Group justify="flex-end">
          <Button variant="default" onClick={onClose} disabled={!finished}>
            {finished ? 'Close' : 'Running…'}
          </Button>
        </Group>
      </Stack>
    </Modal>
  )
}
