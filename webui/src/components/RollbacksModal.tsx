import { useEffect, useState } from 'react'
import { Modal, Table, Text, Anchor, Stack, Loader, Alert } from '@mantine/core'
import { IconAlertCircle } from '@tabler/icons-react'
import type { RollbackFile } from '../types/api'
import { fetchRollbacks, rollbackDownloadUrl } from '../api/containers'

interface Props {
  containerId: string | null
  containerName: string
  onClose: () => void
}

export function RollbacksModal({ containerId, containerName, onClose }: Props) {
  const [files, setFiles] = useState<RollbackFile[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!containerId) return
    setLoading(true)
    setError(null)
    fetchRollbacks(containerId)
      .then(setFiles)
      .catch(e => setError(e.message))
      .finally(() => setLoading(false))
  }, [containerId])

  return (
    <Modal
      opened={!!containerId}
      onClose={onClose}
      title={`Rollbacks: ${containerName}`}
      size="lg"
    >
      {loading && <Loader size="sm" />}
      {error && (
        <Alert icon={<IconAlertCircle size={16} />} color="red">
          {error}
        </Alert>
      )}
      {!loading && !error && files.length === 0 && (
        <Text c="dimmed" size="sm">No rollback files yet. Rollbacks are generated automatically on update.</Text>
      )}
      {files.length > 0 && (
        <Stack>
          <Text size="sm" c="dimmed">
            Use: <code>docker compose -f &lt;file&gt; up -d</code> to restore
          </Text>
          <Table>
            <Table.Thead>
              <Table.Tr>
                <Table.Th>File</Table.Th>
                <Table.Th>Previous image</Table.Th>
                <Table.Th>Created</Table.Th>
              </Table.Tr>
            </Table.Thead>
            <Table.Tbody>
              {files.map(f => (
                <Table.Tr key={f.filename}>
                  <Table.Td>
                    <Anchor href={rollbackDownloadUrl(containerId!, f.filename)} download>
                      {f.filename}
                    </Anchor>
                  </Table.Td>
                  <Table.Td>
                    <Text size="xs" style={{ fontFamily: 'monospace', wordBreak: 'break-all' }}>
                      {f.previous_image}
                    </Text>
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
