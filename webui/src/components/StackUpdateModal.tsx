import { useEffect, useRef, useState } from 'react'
import { Modal, Progress, Stack, Button, Alert, Group, Badge, Text } from '@mantine/core'
import { useMediaQuery } from '@mantine/hooks'
import { IconCheck, IconAlertCircle } from '@tabler/icons-react'
import type { StackEvent } from '../types/api'
import { triggerStackUpdate, createStackUpdateEventSource } from '../api/containers'
import { getRollbackIncludeEnv } from '../settings'
import { useSSEJob } from '../hooks/useSSEJob'
import { createLayerProgress, type LayerProgress } from '../lib/pullProgress'
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
  const job = useSSEJob<StackEvent>()
  const layersRef = useRef<LayerProgress | null>(null)
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
    layersRef.current = createLayerProgress()
    job.start({
      trigger: () => triggerStackUpdate(stackName, getRollbackIncludeEnv()),
      createEventSource: () => createStackUpdateEventSource(stackName),
      onProgress: (evt) => {
        if (evt.step) setCurrentStep(evt.step)
        if (evt.line) setLines(prev => [...prev, evt.line!])
        const pct = layersRef.current?.update(evt.layer_id, evt.current, evt.total) ?? null
        setProgress(p => {
          if (pct !== null) return Math.max(p, pct)
          // compose-file mode has no byte counts; keep the line heuristic
          if (evt.step === 'rollback') return Math.max(p, 5)
          if (evt.step === 'pull') return Math.max(p, Math.min(p + 1, 70))
          if (evt.step === 'up') return Math.max(p, Math.min(p + 3, 95))
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
          <Text fw={600}>Update stack:</Text>
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
            <Alert icon={<IconAlertCircle size={16} />} color="blue" title="Confirm stack update">
              <Text size="sm">
                Pull the latest images and recreate every container of{' '}
                <Text span ff="monospace">{stackName}</Text>. A rollback snapshot is saved first.
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
                Stack updated and containers recreated successfully.
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
