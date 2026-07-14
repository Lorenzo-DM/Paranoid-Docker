import { useEffect, useRef, useState } from 'react'
import { Modal, Stack, Text, Progress, ScrollArea, Group, ThemeIcon, Button, Box, Loader, Alert, Badge } from '@mantine/core'
import { useMediaQuery } from '@mantine/hooks'
import { IconCheck, IconX, IconMinus, IconAlertCircle } from '@tabler/icons-react'
import type { PullEvent } from '../types/api'
import { triggerUpdate, createPullStatusEventSource } from '../api/containers'
import { getRollbackIncludeEnv } from '../settings'
import { runSSEJob } from '../hooks/useSSEJob'

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

type Phase = 'confirm' | 'running' | 'done'

export function BulkContainerUpdateModal({ containers, onClose }: Props) {
  const [phase, setPhase] = useState<Phase>('confirm')
  const [items, setItems] = useState<ContainerItem[]>([])
  const [currentLogs, setCurrentLogs] = useState<string[]>([])
  const [reconnecting, setReconnecting] = useState(false)
  const abortRef = useRef<(() => void) | null>(null)
  const fullScreen = useMediaQuery('(max-width: 48em)')

  const key = containers.map(c => c.id).join(',')

  useEffect(() => {
    if (containers.length === 0) return
    setPhase('confirm')
    setItems(containers.map(c => ({ ...c, status: 'pending' })))
    setCurrentLogs([])
    setReconnecting(false)
    return () => abortRef.current?.()
  }, [key]) // eslint-disable-line react-hooks/exhaustive-deps

  const start = async () => {
    setPhase('running')
    const refs = containers
    for (let i = 0; i < refs.length; i++) {
      setCurrentLogs([])
      setItems(prev => prev.map((s, idx) => idx === i ? { ...s, status: 'running' } : s))
      try {
        const job = runSSEJob<PullEvent>({
          trigger: () => triggerUpdate(refs[i].id, getRollbackIncludeEnv()),
          createEventSource: () => createPullStatusEventSource(refs[i].id),
          onProgress: (evt) => {
            const text = [evt.id, evt.status, evt.progress].filter(Boolean).join(' ')
            if (text) setCurrentLogs(prev => [...prev, text])
          },
          onReconnecting: () => setReconnecting(true),
          onReconnected: () => setReconnecting(false),
        })
        abortRef.current = job.abort
        await job.promise
        setItems(prev => prev.map((s, idx) => idx === i ? { ...s, status: 'done' } : s))
      } catch (err) {
        setItems(prev => prev.map((s, idx) =>
          idx === i ? { ...s, status: 'error', error: err instanceof Error ? err.message : 'Update failed' } : s
        ))
      }
      setReconnecting(false)
    }
    setPhase('done')
  }

  const completedCount = items.filter(s => s.status === 'done' || s.status === 'error').length
  const progress = items.length > 0 ? Math.round((completedCount / items.length) * 100) : 0
  const hasErrors = items.some(s => s.status === 'error')
  const finished = phase === 'done'

  return (
    <Modal
      opened={containers.length > 0}
      onClose={onClose}
      fullScreen={fullScreen}
      title={
        <Group gap="xs">
          <Text fw={600}>Bulk update — {containers.length} container{containers.length !== 1 ? 's' : ''}</Text>
          {reconnecting && <Badge color="yellow" variant="light" size="sm">reconnecting…</Badge>}
        </Group>
      }
      size="lg"
      closeOnClickOutside={phase !== 'running'}
      closeOnEscape={phase !== 'running'}
    >
      <Stack>
        {phase === 'confirm' ? (
          <>
            <Alert icon={<IconAlertCircle size={16} />} color="blue" title="Confirm bulk update">
              <Text size="sm">
                Update {containers.length} container{containers.length !== 1 ? 's' : ''} sequentially.
                Each container gets a rollback compose file before recreation.
              </Text>
              <Text size="sm" mt={4}>
                Environment in rollbacks: <Text span fw={600}>{getRollbackIncludeEnv() ? 'included' : 'excluded'}</Text>
              </Text>
            </Alert>
            <Box>
              {items.map(item => (
                <Text key={item.id} size="sm" ff="monospace" py={2}>{item.name}</Text>
              ))}
            </Box>
            <Group justify="flex-end">
              <Button variant="default" onClick={onClose}>Cancel</Button>
              <Button onClick={start}>Start updates</Button>
            </Group>
          </>
        ) : (
          <>
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
              <ScrollArea h={100} type="auto" className="bulk-logs-scroll">
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
          </>
        )}
      </Stack>
    </Modal>
  )
}
