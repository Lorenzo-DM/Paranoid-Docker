import { useEffect, useRef, useState } from 'react'
import { Modal, Progress, Stack, Button, Alert, Group, Badge, Text } from '@mantine/core'
import { useMediaQuery } from '@mantine/hooks'
import { IconCheck, IconAlertCircle } from '@tabler/icons-react'
import type { PullEvent } from '../types/api'
import { triggerUpdate, createPullStatusEventSource } from '../api/containers'
import { getRollbackIncludeEnv } from '../settings'
import { useSSEJob } from '../hooks/useSSEJob'
import { createLayerProgress, type LayerProgress } from '../lib/pullProgress'
import { PullProgressLog } from './PullProgressLog'

interface Props {
  containerId: string | null
  containerName: string
  onClose: () => void
}

// recreate steps reported after the pull, mapped into the 70–100 band
const recreateProgress: Record<string, number> = {
  'Stopping...': 75,
  'Removing...': 80,
  'Creating...': 85,
  'Starting...': 95,
}

export function UpdateModal({ containerId, containerName, onClose }: Props) {
  const [lines, setLines] = useState<string[]>([])
  const [progress, setProgress] = useState(0)
  const job = useSSEJob<PullEvent>()
  const layersRef = useRef<LayerProgress | null>(null)
  const fullScreen = useMediaQuery('(max-width: 48em)')

  const { reset } = job
  useEffect(() => {
    if (containerId) {
      setLines([])
      setProgress(0)
      reset()
    }
  }, [containerId, reset])

  const start = () => {
    if (!containerId) return
    layersRef.current = createLayerProgress()
    job.start({
      trigger: () => triggerUpdate(containerId, getRollbackIncludeEnv()),
      createEventSource: () => createPullStatusEventSource(containerId),
      onProgress: (evt) => {
        const text = [evt.status, evt.progress].filter(Boolean).join(' ')
        if (text) setLines(prev => [...prev, text])
        const pct = layersRef.current?.update(evt.id, evt.current, evt.total) ?? null
        if (pct !== null) {
          setProgress(p => Math.max(p, pct))
        } else if (evt.status && recreateProgress[evt.status] !== undefined) {
          setProgress(p => Math.max(p, recreateProgress[evt.status!]))
        }
      },
      onDone: (evt) => {
        setLines(prev => [...prev, evt.status ?? 'Done!'])
        setProgress(100)
      },
    })
  }

  const running = job.phase === 'running' || job.phase === 'reconnecting'
  const done = job.phase === 'done'
  const confirming = job.phase === 'idle'

  return (
    <Modal
      opened={!!containerId}
      onClose={onClose}
      fullScreen={fullScreen}
      title={
        <Group gap="xs">
          <Text fw={600}>Update:</Text>
          <Text ff="monospace">{containerName}</Text>
          {job.phase === 'reconnecting' && (
            <Badge color="yellow" variant="light" size="sm">reconnecting…</Badge>
          )}
        </Group>
      }
      closeOnClickOutside={!running}
      closeOnEscape={!running}
      size="lg"
    >
      <Stack>
        {confirming ? (
          <>
            <Alert icon={<IconAlertCircle size={16} />} color="blue" title="Confirm update">
              <Text size="sm">
                Pull the latest image and recreate <Text span ff="monospace">{containerName}</Text>.
                A rollback compose file is saved first.
              </Text>
              <Text size="sm" mt={4}>
                Environment in rollback: <Text span fw={600}>{getRollbackIncludeEnv() ? 'included' : 'excluded'}</Text>
              </Text>
            </Alert>
            <Group justify="flex-end">
              <Button variant="default" onClick={onClose}>Cancel</Button>
              <Button onClick={start}>Start update</Button>
            </Group>
          </>
        ) : (
          <>
            <Progress value={progress} animated={running} color={job.error ? 'red' : done ? 'green' : 'blue'} />
            <PullProgressLog lines={lines} />
            {done && (
              <Alert icon={<IconCheck size={16} />} color="green" title="Update complete">
                Container updated and restarted successfully.
              </Alert>
            )}
            {job.error && (
              <Alert icon={<IconAlertCircle size={16} />} color="red" title="Update failed">
                {job.error}
              </Alert>
            )}
            <Group justify="flex-end">
              <Button variant="default" onClick={onClose} disabled={running}>
                Close
              </Button>
            </Group>
          </>
        )}
      </Stack>
    </Modal>
  )
}
