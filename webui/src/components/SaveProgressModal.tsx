import { useEffect, useState } from 'react'
import { Modal, Stack, Progress, Text, Alert, Group, Button, Anchor } from '@mantine/core'
import { IconCheck, IconAlertCircle } from '@tabler/icons-react'
import type { SaveProgress } from '../types/api'
import { triggerSaveImage, createSaveStatusEventSource, savedImageDownloadUrl } from '../api/containers'

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
  const [done, setDone] = useState(false)
  const [filename, setFilename] = useState<string | null>(null)
  const [totalSize, setTotalSize] = useState(0)
  const [error, setError] = useState<string | null>(null)
  const [running, setRunning] = useState(false)

  useEffect(() => {
    if (!containerId) return

    setWritten(0)
    setDone(false)
    setFilename(null)
    setTotalSize(0)
    setError(null)
    setRunning(true)

    let es: EventSource | null = null

    triggerSaveImage(containerId)
      .then(() => {
        es = createSaveStatusEventSource(containerId)

        es.addEventListener('progress', (e) => {
          const evt: SaveProgress = JSON.parse(e.data)
          setWritten(evt.written_bytes ?? 0)
        })

        es.addEventListener('done', (e) => {
          const evt: SaveProgress = JSON.parse(e.data)
          setFilename(evt.filename ?? null)
          setTotalSize(evt.size_bytes ?? 0)
          setDone(true)
          setRunning(false)
          es?.close()
        })

        es.addEventListener('error', (e) => {
          const evt: SaveProgress = JSON.parse((e as MessageEvent).data ?? '{}')
          setError(evt.error ?? 'Save failed')
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
      title={`Save image: ${containerName}`}
      closeOnClickOutside={!running}
      closeOnEscape={!running}
      size="md"
    >
      <Stack>
        <Progress value={done ? 100 : 0} animated={running} color={error ? 'red' : done ? 'green' : 'blue'} />
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
        {error && (
          <Alert icon={<IconAlertCircle size={16} />} color="red" title="Save failed">
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
