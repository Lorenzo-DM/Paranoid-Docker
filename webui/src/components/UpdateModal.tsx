import { useEffect, useState } from 'react'
import { Modal, Progress, Stack, Button, Alert, Group } from '@mantine/core'
import { IconCheck, IconAlertCircle } from '@tabler/icons-react'
import type { PullEvent } from '../types/api'
import { triggerUpdate, createPullStatusEventSource } from '../api/containers'
import { getRollbackIncludeEnv } from '../settings'
import { PullProgressLog } from './PullProgressLog'

interface Props {
  containerId: string | null
  containerName: string
  onClose: () => void
}

export function UpdateModal({ containerId, containerName, onClose }: Props) {
  const [lines, setLines] = useState<string[]>([])
  const [progress, setProgress] = useState(0)
  const [done, setDone] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [running, setRunning] = useState(false)

  useEffect(() => {
    if (!containerId) return

    setLines([])
    setProgress(0)
    setDone(false)
    setError(null)
    setRunning(true)

    let es: EventSource | null = null

    triggerUpdate(containerId, getRollbackIncludeEnv())
      .then(() => {
        es = createPullStatusEventSource(containerId)

        es.addEventListener('progress', (e) => {
          const evt: PullEvent = JSON.parse(e.data)
          const text = [evt.status, evt.progress].filter(Boolean).join(' ')
          if (text) setLines(prev => [...prev, text])
          setProgress(p => Math.min(p + 2, 90))
        })

        es.addEventListener('done', (e) => {
          const evt: PullEvent = JSON.parse(e.data)
          setLines(prev => [...prev, evt.status ?? 'Done!'])
          setProgress(100)
          setDone(true)
          setRunning(false)
          es?.close()
        })

        es.addEventListener('error', (e) => {
          const evt: PullEvent = JSON.parse((e as MessageEvent).data ?? '{}')
          setError(evt.error ?? 'Update failed')
          setRunning(false)
          es?.close()
        })
      })
      .catch(e => {
        setError(e.message)
        setRunning(false)
      })

    return () => {
      es?.close()
    }
  }, [containerId])

  return (
    <Modal
      opened={!!containerId}
      onClose={onClose}
      title={`Update: ${containerName}`}
      closeOnClickOutside={!running}
      closeOnEscape={!running}
      size="lg"
    >
      <Stack>
        <Progress value={progress} animated={running} color={error ? 'red' : done ? 'green' : 'blue'} />
        <PullProgressLog lines={lines} />
        {done && (
          <Alert icon={<IconCheck size={16} />} color="green" title="Update complete">
            Container updated and restarted successfully.
          </Alert>
        )}
        {error && (
          <Alert icon={<IconAlertCircle size={16} />} color="red" title="Update failed">
            {error}
          </Alert>
        )}
        <Group justify="flex-end">
          <Button variant="default" onClick={onClose} disabled={running}>
            Close
          </Button>
        </Group>
      </Stack>
    </Modal>
  )
}
