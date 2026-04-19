import { useEffect, useRef } from 'react'
import { Modal } from '@mantine/core'
import { createLogsEventSource } from '../api/containers'
import { TerminalView } from './TerminalView'

interface Props {
  containerId: string | null
  containerName: string
  onClose: () => void
}

export function LogsModal({ containerId, containerName, onClose }: Props) {
  const writeRef = useRef<((line: string) => void) | null>(null)

  useEffect(() => {
    if (!containerId) return

    const es = createLogsEventSource(containerId)

    es.addEventListener('log', (e) => {
      const { line } = JSON.parse(e.data)
      writeRef.current?.(line)
    })

    return () => {
      es.close()
    }
  }, [containerId])

  return (
    <Modal
      opened={!!containerId}
      onClose={onClose}
      title={`Logs: ${containerName}`}
      size="xl"
      styles={{ body: { height: '60vh', padding: 0 } }}
    >
      <TerminalView
        onReady={(write) => {
          writeRef.current = write
        }}
      />
    </Modal>
  )
}
