import { useEffect, useRef, useState } from 'react'
import { Modal, Stack, Text, ScrollArea, Group, ThemeIcon, Box, Loader, Progress, Alert, Button, Badge } from '@mantine/core'
import { useMediaQuery } from '@mantine/hooks'
import { IconCheck, IconX, IconMinus, IconAlertCircle } from '@tabler/icons-react'
import type { StackEvent } from '../types/api'
import { triggerStackUpdate, createStackUpdateEventSource } from '../api/containers'
import { getRollbackIncludeEnv } from '../settings'
import { runSSEJob } from '../hooks/useSSEJob'

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

type Phase = 'confirm' | 'running' | 'done'

export function BulkStackUpdateModal({ stackNames, onClose }: Props) {
  const [phase, setPhase] = useState<Phase>('confirm')
  const [items, setItems] = useState<StackItem[]>([])
  const [currentLogs, setCurrentLogs] = useState<string[]>([])
  const [reconnecting, setReconnecting] = useState(false)
  const abortRef = useRef<(() => void) | null>(null)
  const fullScreen = useMediaQuery('(max-width: 48em)')

  const key = stackNames.join(',')

  useEffect(() => {
    if (stackNames.length === 0) return
    setPhase('confirm')
    setItems(stackNames.map(name => ({ name, status: 'pending' })))
    setCurrentLogs([])
    setReconnecting(false)
    return () => abortRef.current?.()
  }, [key]) // eslint-disable-line react-hooks/exhaustive-deps

  const start = async () => {
    setPhase('running')
    const names = stackNames
    for (let i = 0; i < names.length; i++) {
      setCurrentLogs([])
      setItems(prev => prev.map((s, idx) => idx === i ? { ...s, status: 'running' } : s))
      try {
        const job = runSSEJob<StackEvent>({
          trigger: () => triggerStackUpdate(names[i], getRollbackIncludeEnv()),
          createEventSource: () => createStackUpdateEventSource(names[i]),
          onProgress: (evt) => { if (evt.line) setCurrentLogs(prev => [...prev, evt.line!]) },
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
  const finished = phase === 'done'

  return (
    <Modal
      opened={stackNames.length > 0}
      onClose={onClose}
      fullScreen={fullScreen}
      title={
        <Group gap="xs">
          <Text fw={600}>Bulk update</Text>
          <span className="pill mono">{stackNames.length} stacks</span>
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
                Update {stackNames.length} stack{stackNames.length !== 1 ? 's' : ''} sequentially.
                Each stack gets a rollback snapshot before its containers are recreated.
              </Text>
              <Text size="sm" mt={4}>
                Environment in rollbacks: <Text span fw={600}>{getRollbackIncludeEnv() ? 'included' : 'excluded'}</Text>
              </Text>
            </Alert>
            <Box className="glass-inset" p="xs">
              {items.map(item => (
                <Text key={item.name} size="sm" ff="monospace" py={2} c="var(--ink-2)">{item.name}</Text>
              ))}
            </Box>
            <Group justify="flex-end">
              <Button variant="default" onClick={onClose}>Cancel</Button>
              <Button onClick={start}>Start updates</Button>
            </Group>
          </>
        ) : (
          <>
            <Progress value={progress} size="xs" />

            <Box className="glass-inset" p="xs">
              {items.map(item => (
                <Group key={item.name} gap="xs" py={4} wrap="nowrap" className="bulk-item">
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
          </>
        )}
      </Stack>
    </Modal>
  )
}
