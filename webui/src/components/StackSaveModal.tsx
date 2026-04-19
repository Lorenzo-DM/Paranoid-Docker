import { useEffect, useState } from 'react'
import { Modal, Progress, Stack, Button, Alert, Group, Badge, Text } from '@mantine/core'
import { IconCheck, IconAlertCircle } from '@tabler/icons-react'
import type { StackEvent } from '../types/api'
import {
  triggerSaveStackImages, createStackSaveEventSource,
  triggerSaveAndUpdateStack, createStackSaveUpdateEventSource,
} from '../api/containers'
import { getRollbackIncludeEnv } from '../settings'
import { PullProgressLog } from './PullProgressLog'

interface Props {
  stackName: string | null
  onClose: () => void
}

const stepColor: Record<string, string> = {
  save:     'violet',
  rollback: 'gray',
  pull:     'blue',
  up:       'teal',
}

type TriggerFn = (name: string) => Promise<{ job_id: string }>
type EsFn     = (name: string) => EventSource

function StackOpModal({
  stackName,
  title,
  onTrigger,
  onEventSource,
  doneMessage,
  onClose,
}: {
  stackName: string | null
  title: string
  onTrigger: TriggerFn
  onEventSource: EsFn
  doneMessage: string
  onClose: () => void
}) {
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

    onTrigger(stackName)
      .then(() => {
        es = onEventSource(stackName)

        es.addEventListener('progress', (e) => {
          const evt: StackEvent = JSON.parse(e.data)
          if (evt.step) setCurrentStep(evt.step)
          if (evt.line) setLines(prev => [...prev, evt.line!])
          setProgress(p => {
            if (evt.step === 'save')     return Math.max(p, Math.min(p + 2, 20))
            if (evt.step === 'rollback') return Math.max(p, 22)
            if (evt.step === 'pull')     return Math.max(p, Math.min(p + 1, 80))
            if (evt.step === 'up')       return Math.max(p, Math.min(p + 3, 97))
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
          setError(evt.error ?? 'Operation failed')
          setRunning(false)
          es?.close()
        })
      })
      .catch(e => {
        setError(e.message)
        setRunning(false)
      })

    return () => es?.close()
  }, [stackName]) // eslint-disable-line react-hooks/exhaustive-deps

  return (
    <Modal
      opened={!!stackName}
      onClose={onClose}
      title={
        <Group gap="xs">
          <Text fw={600}>{title}</Text>
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
          <Alert icon={<IconCheck size={16} />} color="green" title="Done">
            {doneMessage}
          </Alert>
        )}
        {error && (
          <Alert icon={<IconAlertCircle size={16} />} color="red" title="Failed">
            {error}
          </Alert>
        )}
        <Group justify="flex-end">
          <Button variant="default" onClick={onClose} disabled={running}>Close</Button>
        </Group>
      </Stack>
    </Modal>
  )
}

export function StackSaveModal({ stackName, onClose }: Props) {
  return (
    <StackOpModal
      stackName={stackName}
      title="Save images:"
      onTrigger={triggerSaveStackImages}
      onEventSource={createStackSaveEventSource}
      doneMessage="All service images saved to the images/ directory."
      onClose={onClose}
    />
  )
}

export function StackSaveUpdateModal({ stackName, onClose }: Props) {
  return (
    <StackOpModal
      stackName={stackName}
      title="Save + Update:"
      onTrigger={name => triggerSaveAndUpdateStack(name, getRollbackIncludeEnv())}
      onEventSource={createStackSaveUpdateEventSource}
      doneMessage="Images saved and stack updated successfully."
      onClose={onClose}
    />
  )
}
