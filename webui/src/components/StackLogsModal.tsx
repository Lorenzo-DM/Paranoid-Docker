import { useEffect, useRef } from 'react'
import { Modal } from '@mantine/core'
import { createStackLogsEventSource } from '../api/containers'
import { TerminalView } from './TerminalView'

interface Props {
  stackName: string | null
  onClose: () => void
}

export function StackLogsModal({ stackName, onClose }: Props) {
  const writeRef = useRef<((line: string) => void) | null>(null)

  useEffect(() => {
    if (!stackName) return

    const es = createStackLogsEventSource(stackName)
    es.addEventListener('log', (e) => {
      const { line } = JSON.parse(e.data)
      writeRef.current?.(line)
    })

    return () => es.close()
  }, [stackName])

  return (
    <Modal
      opened={!!stackName}
      onClose={onClose}
      title={`Logs: ${stackName}`}
      size="xl"
      classNames={{ body: 'modal-logs-body' }}
    >
      <TerminalView onReady={(write) => { writeRef.current = write }} />
    </Modal>
  )
}
