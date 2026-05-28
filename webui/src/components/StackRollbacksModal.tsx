import { useEffect, useState } from 'react'
import {
  Modal,
  Table,
  Text,
  Anchor,
  Stack,
  Loader,
  Alert,
  Group,
  SegmentedControl,
  Checkbox,
  Button,
  Badge,
  ScrollArea,
  Collapse,
  Box,
} from '@mantine/core'
import { IconAlertCircle, IconCheck, IconChevronDown, IconChevronRight } from '@tabler/icons-react'
import type { RollbackFile, RollbackPreview, RollbackRestoreMode } from '../types/api'
import { fetchStackRollbacks, stackRollbackDownloadUrl, fetchStackRollbackPreview, executeStackRollback } from '../api/containers'

interface Props {
  stackName: string | null
  onClose: () => void
}

export function StackRollbacksModal({ stackName, onClose }: Props) {
  const [files, setFiles] = useState<RollbackFile[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const [selected, setSelected] = useState<RollbackFile | null>(null)
  const [preview, setPreview] = useState<RollbackPreview | null>(null)
  const [previewLoading, setPreviewLoading] = useState(false)
  const [previewError, setPreviewError] = useState<string | null>(null)

  const [mode, setMode] = useState<RollbackRestoreMode>('standard')
  const [confirmed, setConfirmed] = useState(false)
  const [executing, setExecuting] = useState(false)
  const [execError, setExecError] = useState<string | null>(null)
  const [execSuccess, setExecSuccess] = useState(false)

  const [yamlOpen, setYamlOpen] = useState(false)

  useEffect(() => {
    if (!stackName) return
    setLoading(true)
    setError(null)
    setSelected(null)
    setPreview(null)
    fetchStackRollbacks(stackName)
      .then(setFiles)
      .catch(e => setError(e.message))
      .finally(() => setLoading(false))
  }, [stackName])

  useEffect(() => {
    if (!selected || !stackName) return
    setPreview(null)
    setPreviewError(null)
    setPreviewLoading(true)
    setConfirmed(false)
    setExecError(null)
    setExecSuccess(false)
    setYamlOpen(false)
    fetchStackRollbackPreview(stackName, selected.filename, mode)
      .then(setPreview)
      .catch(e => setPreviewError(e.message))
      .finally(() => setPreviewLoading(false))
  }, [selected, mode, stackName])

  function handleExecute() {
    if (!stackName || !selected) return
    setExecuting(true)
    setExecError(null)
    executeStackRollback(stackName, selected.filename, mode)
      .then(() => {
        setExecSuccess(true)
        setTimeout(() => onClose(), 1500)
      })
      .catch(e => setExecError(e.message))
      .finally(() => setExecuting(false))
  }

  return (
    <Modal
      opened={!!stackName}
      onClose={onClose}
      title={`Rollbacks: ${stackName}`}
      size={selected ? 'xl' : 'lg'}
    >
      {loading && <Loader size="sm" />}
      {error && (
        <Alert icon={<IconAlertCircle size={16} />} color="red">{error}</Alert>
      )}
      {!loading && !error && files.length === 0 && (
        <Text c="dimmed" size="sm">
          No rollbacks yet — they are saved automatically before each update.
        </Text>
      )}
      {files.length > 0 && (
        <Stack>
          <Text size="sm" c="dimmed">
            Click a snapshot to preview and restore, or download the compose file.
          </Text>
          <Table withTableBorder highlightOnHover>
            <Table.Thead>
              <Table.Tr>
                <Table.Th>File</Table.Th>
                <Table.Th>Saved</Table.Th>
              </Table.Tr>
            </Table.Thead>
            <Table.Tbody>
              {files.map(f => (
                <Table.Tr
                  key={f.filename}
                  onClick={() => setSelected(f)}
                  style={{ cursor: 'pointer', background: selected?.filename === f.filename ? 'var(--mantine-color-blue-light)' : undefined }}
                >
                  <Table.Td>
                    <Group gap="xs" wrap="nowrap">
                      {selected?.filename === f.filename
                        ? <IconChevronDown size={14} />
                        : <IconChevronRight size={14} />}
                      <Anchor
                        href={stackRollbackDownloadUrl(stackName!, f.filename)}
                        download
                        onClick={e => e.stopPropagation()}
                        size="sm"
                      >
                        {f.filename}
                      </Anchor>
                    </Group>
                  </Table.Td>
                  <Table.Td>
                    <Text size="xs">{new Date(f.created_at).toLocaleString()}</Text>
                  </Table.Td>
                </Table.Tr>
              ))}
            </Table.Tbody>
          </Table>

          {selected && (
            <Box
              p="md"
              style={{ border: '1px solid var(--mantine-color-blue-light)', borderRadius: 'var(--mantine-radius-sm)' }}
            >
              <Stack>
                <Text fw={600} size="sm">Preview: {selected.filename}</Text>

                {previewLoading && <Loader size="sm" />}
                {previewError && (
                  <Alert icon={<IconAlertCircle size={16} />} color="red">
                    {previewError}
                  </Alert>
                )}

                {preview && (
                  <Stack>
                    {/* Image changes per service */}
                    <Stack gap={4}>
                      <Text size="sm" fw={500}>Image changes</Text>
                      {preview.manifest.items.map(item => (
                        <Group key={item.name} gap="xs" wrap="nowrap">
                          <Text size="xs" ff="monospace" c="dimmed">{item.name}:</Text>
                          <Text size="xs" ff="monospace" c="red" style={{ wordBreak: 'break-all' }}>{item.current_image}</Text>
                          <Text size="xs" c="dimmed">→</Text>
                          <Text size="xs" ff="monospace" c="green" style={{ wordBreak: 'break-all' }}>{item.rollback_image}</Text>
                        </Group>
                      ))}
                    </Stack>

                    {/* Planned actions */}
                    {preview.actions.length > 0 && (
                      <Stack gap={4}>
                        <Text size="sm" fw={500}>Planned actions</Text>
                        {preview.actions.map((a, i) => (
                          <Group key={i} gap="xs" wrap="nowrap">
                            {a.destructive && <Badge color="red" size="xs" variant="light">destructive</Badge>}
                            <Text size="xs" c={a.destructive ? 'red' : undefined}>{a.description}</Text>
                          </Group>
                        ))}
                      </Stack>
                    )}

                    {/* Warnings */}
                    {preview.warnings.length > 0 && (
                      <Alert icon={<IconAlertCircle size={16} />} color="yellow" title="Warnings">
                        <Stack gap={4}>
                          {preview.warnings.map((w, i) => <Text key={i} size="xs">{w}</Text>)}
                        </Stack>
                      </Alert>
                    )}

                    {/* YAML preview (collapsible) */}
                    <Box>
                      <Anchor size="xs" onClick={() => setYamlOpen(o => !o)} style={{ cursor: 'pointer' }}>
                        {yamlOpen ? 'Hide' : 'Show'} compose YAML
                      </Anchor>
                      <Collapse expanded={yamlOpen}>
                        <ScrollArea h={200} mt="xs">
                          <Text size="xs" ff="monospace" style={{ whiteSpace: 'pre' }}>{preview.yaml}</Text>
                        </ScrollArea>
                      </Collapse>
                    </Box>

                    {/* Secrets note */}
                    {preview.secrets_note && (
                      <Text size="xs" c="dimmed">{preview.secrets_note}</Text>
                    )}

                    {/* Mode selector */}
                    <Group gap="xs" align="center">
                      <Text size="sm">Mode:</Text>
                      <SegmentedControl
                        size="xs"
                        value={mode}
                        onChange={v => setMode(v as RollbackRestoreMode)}
                        data={[
                          { label: 'Standard', value: 'standard' },
                          { label: 'Advanced', value: 'advanced' },
                        ]}
                      />
                    </Group>

                    {/* Confirmation */}
                    <Checkbox
                      label="I understand this will stop and recreate containers"
                      checked={confirmed}
                      onChange={e => setConfirmed(e.currentTarget.checked)}
                    />

                    {execError && (
                      <Alert icon={<IconAlertCircle size={16} />} color="red">
                        {execError}
                      </Alert>
                    )}
                    {execSuccess && (
                      <Alert icon={<IconCheck size={16} />} color="green">
                        Rollback started successfully. Closing…
                      </Alert>
                    )}

                    <Group justify="flex-end">
                      <Button
                        color="red"
                        disabled={!confirmed || executing || execSuccess}
                        loading={executing}
                        onClick={handleExecute}
                      >
                        Execute Rollback
                      </Button>
                    </Group>
                  </Stack>
                )}
              </Stack>
            </Box>
          )}
        </Stack>
      )}
    </Modal>
  )
}
