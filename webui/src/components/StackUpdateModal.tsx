import { useEffect, useState } from 'react'
import { Modal, Progress, Stack, Button, Alert, Group, Badge, Text } from '@mantine/core'
import { IconCheck, IconAlertCircle } from '@tabler/icons-react'
import type { StackEvent } from '../types/api'
import { triggerStackUpdate, createStackUpdateEventSource } from '../api/containers'
import { getRollbackIncludeEnv } from '../settings'
import { PullProgressLog } from './PullProgressLog'

interface Props {
  stackName: string | null
  onClose: () => void
}

const stepColor: Record<string, string> = {
  rollback: 'gray',
  pull: 'blue',
  up: 'teal',
}

export function StackUpdateModal({ stackName, onClose }: Props) {
  const [lines, setLines] = useState<string[]>([])
  const [currentStep, setCurrentStep] = useState('')
  const [progress, setProgress] = useState(0)
  const [done, setDone] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [running, setRunning] = useState(false)

  useEffect(() => {
    if (!stackName) return

    setLines([])
    setCurrentStep('')
    setProgress(0)
    setDone(false)
    setError(null)
    setRunning(true)

    let es: EventSource | null = null

    triggerStackUpdate(stackName, getRollbackIncludeEnv())
      .then(() => {
        es = createStackUpdateEventSource(stackName)

        es.addEventListener('progress', (e) => {
          const evt: StackEvent = JSON.parse(e.data)
          if (evt.step) setCurrentStep(evt.step)
          const text = evt.line ?? ''
          if (text) setLines(prev => [...prev, text])
          setProgress(p => {
            if (evt.step === 'rollback') return Math.max(p, 5)
            if (evt.step === 'pull') return Math.max(p, Math.min(p + 1, 70))
            if (evt.step === 'up') return Math.max(p, Math.min(p + 3, 95))
            return p
          })
        })

        es.addEventListener('done', (e) => {
          const evt: StackEvent = JSON.parse(e.data)
          setLines(prev => [...prev, evt.line ?? 'Done!'])
          setProgress(100)
          setDone(true)
          setRunning(false)
          es?.close()
        })

        es.addEventListener('error', (e) => {
          const evt: StackEvent = JSON.parse((e as MessageEvent).data ?? '{}')
          setError(evt.error ?? 'Update failed')
          setRunning(false)
          es?.close()
        })
      })
      .catch(e => {
        setError(e.message)
        setRunning(false)
      })

    return () => es?.close()
  }, [stackName])

  return (
    <Modal
      opened={!!stackName}
      onClose={onClose}
      title={
        <Group gap="xs">
          <Text fw={600}>Update stack:</Text>
          <Text ff="monospace">{stackName}</Text>
          {currentStep && (
            <Badge color={stepColor[currentStep] ?? 'gray'} variant="light" size="sm">
              {currentStep}
            </Badge>
          )}
        </Group>
      }
      closeOnClickOutside={!running}
      closeOnEscape={!running}
      size="lg"
    >
      <Stack>
        <Progress value={progress} animated={running} color={error ? 'red' : done ? 'green' : 'blue'} />
        <PullProgressLog lines={lines} />
        {done && (
          <Alert icon={<IconCheck size={16} />} color="green" title="Update complete">
            Stack updated and containers recreated successfully.
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
