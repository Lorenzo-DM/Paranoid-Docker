import { useEffect, useState } from 'react'
import { Modal, Stack, Progress, Text, Alert, Group, Button, Anchor, Badge } from '@mantine/core'
import { useMediaQuery } from '@mantine/hooks'
import { IconCheck, IconAlertCircle } from '@tabler/icons-react'
import type { SaveProgress } from '../types/api'
import { triggerSaveImage, createSaveStatusEventSource, savedImageDownloadUrl } from '../api/containers'
import { useSSEJob } from '../hooks/useSSEJob'

interface Props {
  containerId: string | null
  containerName: string
  onClose: () => void
}

function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}

export function SaveProgressModal({ containerId, containerName, onClose }: Props) {
  const [written, setWritten] = useState(0)
  const [filename, setFilename] = useState<string | null>(null)
  const [totalSize, setTotalSize] = useState(0)
  const job = useSSEJob<SaveProgress>()
  const fullScreen = useMediaQuery('(max-width: 48em)')

  const { reset, start } = job
  useEffect(() => {
    if (!containerId) return
    setWritten(0)
    setFilename(null)
    setTotalSize(0)
    reset()
    // saving is non-destructive: start immediately on open
    start({
      trigger: () => triggerSaveImage(containerId),
      createEventSource: () => createSaveStatusEventSource(containerId),
      onProgress: (evt) => setWritten(evt.written_bytes ?? 0),
      onDone: (evt) => {
        setFilename(evt.filename ?? null)
        setTotalSize(evt.size_bytes ?? 0)
      },
    })
  }, [containerId, reset, start])

  const running = job.phase === 'running' || job.phase === 'reconnecting'
  const done = job.phase === 'done'

  return (
    <Modal
      opened={!!containerId}
      onClose={onClose}
      fullScreen={fullScreen}
      title={
        <Group gap="xs">
          <Text fw={600}>Save image: {containerName}</Text>
          {job.phase === 'reconnecting' && (
            <Badge color="yellow" variant="light" size="sm">reconnecting…</Badge>
          )}
        </Group>
      }
      closeOnClickOutside={!running}
      closeOnEscape={!running}
      size="md"
    >
      <Stack>
        <Progress value={done ? 100 : 0} animated={running} color={job.error ? 'red' : done ? 'green' : 'blue'} />
        {running && (
          <Text size="sm" c="dimmed">
            Written: {formatBytes(written)}
          </Text>
        )}
        {done && filename && (
          <Alert icon={<IconCheck size={16} />} color="green" title="Image saved">
            <Text size="sm">{filename} — {formatBytes(totalSize)}</Text>
            <Anchor href={savedImageDownloadUrl(filename)} download mt="xs" display="block">
              Download .tar
            </Anchor>
          </Alert>
        )}
        {job.error && (
          <Alert icon={<IconAlertCircle size={16} />} color="red" title="Save failed">
            {job.error}
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
