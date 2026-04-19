import { useEffect, useState } from 'react'
import { Modal, Table, Text, Anchor, Stack, Loader, Alert, Code } from '@mantine/core'
import { IconAlertCircle } from '@tabler/icons-react'
import type { RollbackFile } from '../types/api'
import { fetchStackRollbacks, stackRollbackDownloadUrl } from '../api/containers'

interface Props {
  stackName: string | null
  onClose: () => void
}

export function StackRollbacksModal({ stackName, onClose }: Props) {
  const [files, setFiles] = useState<RollbackFile[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!stackName) return
    setLoading(true)
    setError(null)
    fetchStackRollbacks(stackName)
      .then(setFiles)
      .catch(e => setError(e.message))
      .finally(() => setLoading(false))
  }, [stackName])

  return (
    <Modal
      opened={!!stackName}
      onClose={onClose}
      title={`Rollbacks: ${stackName}`}
      size="lg"
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
            Restore with: <Code>docker compose -f &lt;file&gt; up -d</Code>
          </Text>
          <Table withTableBorder>
            <Table.Thead>
              <Table.Tr>
                <Table.Th>File</Table.Th>
                <Table.Th>Saved</Table.Th>
              </Table.Tr>
            </Table.Thead>
            <Table.Tbody>
              {files.map(f => (
                <Table.Tr key={f.filename}>
                  <Table.Td>
                    <Anchor href={stackRollbackDownloadUrl(stackName!, f.filename)} download>
                      {f.filename}
                    </Anchor>
                  </Table.Td>
                  <Table.Td>
                    <Text size="xs">{new Date(f.created_at).toLocaleString()}</Text>
                  </Table.Td>
                </Table.Tr>
              ))}
            </Table.Tbody>
          </Table>
        </Stack>
      )}
    </Modal>
  )
}
