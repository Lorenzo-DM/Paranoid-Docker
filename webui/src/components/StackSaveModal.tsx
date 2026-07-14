import { useEffect, useState } from 'react'
import { Modal, Progress, Stack, Button, Alert, Group, Badge, Text } from '@mantine/core'
import { useMediaQuery } from '@mantine/hooks'
import { IconCheck, IconAlertCircle } from '@tabler/icons-react'
import type { StackEvent } from '../types/api'
import {
  triggerSaveStackImages, createStackSaveEventSource,
  triggerSaveAndUpdateStack, createStackSaveUpdateEventSource,
} from '../api/containers'
import { getRollbackIncludeEnv } from '../settings'
import { useSSEJob } from '../hooks/useSSEJob'
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
  confirmText,
  onTrigger,
  onEventSource,
  doneMessage,
  onClose,
}: {
  stackName: string | null
  title: string
  confirmText: string
  onTrigger: TriggerFn
  onEventSource: EsFn
  doneMessage: string
  onClose: () => void
}) {
  const [lines, setLines] = useState<string[]>([])
  const [currentStep, setCurrentStep] = useState('')
  const [progress, setProgress] = useState(0)
  const job = useSSEJob<StackEvent>()
  const fullScreen = useMediaQuery('(max-width: 48em)')

  const { reset } = job
  useEffect(() => {
    if (stackName) {
      setLines([])
      setCurrentStep('')
      setProgress(0)
      reset()
    }
  }, [stackName, reset])

  const start = () => {
    if (!stackName) return
    job.start({
      trigger: () => onTrigger(stackName),
      createEventSource: () => onEventSource(stackName),
      onProgress: (evt) => {
        if (evt.step) setCurrentStep(evt.step)
        if (evt.line) setLines(prev => [...prev, evt.line!])
        setProgress(p => {
          if (evt.step === 'save')     return Math.max(p, Math.min(p + 2, 20))
          if (evt.step === 'rollback') return Math.max(p, 22)
          if (evt.step === 'pull')     return Math.max(p, Math.min(p + 1, 80))
          if (evt.step === 'up')       return Math.max(p, Math.min(p + 3, 97))
          return p
        })
      },
      onDone: (evt) => {
        setLines(prev => [...prev, evt.line ?? 'Done!'])
        setProgress(100)
      },
    })
  }

  const running = job.phase === 'running' || job.phase === 'reconnecting'
  const done = job.phase === 'done'
  const confirming = job.phase === 'idle'

  return (
    <Modal
      opened={!!stackName}
      onClose={onClose}
      fullScreen={fullScreen}
      title={
        <Group gap="xs">
          <Text fw={600}>{title}</Text>
          <Text ff="monospace">{stackName}</Text>
          {currentStep && !confirming && (
            <Badge color={stepColor[currentStep] ?? 'gray'} variant="light" size="sm">
              {currentStep}
            </Badge>
          )}
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
            <Alert icon={<IconAlertCircle size={16} />} color="blue" title="Confirm">
              <Text size="sm">{confirmText}</Text>
            </Alert>
            <Group justify="flex-end">
              <Button variant="default" onClick={onClose}>Cancel</Button>
              <Button onClick={start}>Start</Button>
            </Group>
          </>
        ) : (
          <>
            <Progress value={progress} animated={running} color={job.error ? 'red' : done ? 'green' : 'blue'} />
            <PullProgressLog lines={lines} />
            {done && (
              <Alert icon={<IconCheck size={16} />} color="green" title="Done">
                {doneMessage}
              </Alert>
            )}
            {job.error && (
              <Alert icon={<IconAlertCircle size={16} />} color="red" title="Failed">
                {job.error}
              </Alert>
            )}
            <Group justify="flex-end">
              <Button variant="default" onClick={onClose} disabled={running}>Close</Button>
            </Group>
          </>
        )}
      </Stack>
    </Modal>
  )
}

export function StackSaveModal({ stackName, onClose }: Props) {
  return (
    <StackOpModal
      stackName={stackName}
      title="Save images:"
      confirmText="Save every service image of this stack as a .tar.gz archive."
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
      confirmText="Save every service image, then pull the latest images and recreate the stack's containers. A rollback snapshot is saved before the update."
      onTrigger={name => triggerSaveAndUpdateStack(name, getRollbackIncludeEnv())}
      onEventSource={createStackSaveUpdateEventSource}
      doneMessage="Images saved and stack updated successfully."
      onClose={onClose}
    />
  )
}
