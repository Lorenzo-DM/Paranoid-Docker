import { useState } from 'react'
import { Group, Tooltip, Popover, Divider, Text, SegmentedControl, useMantineColorScheme } from '@mantine/core'
import { IconBrandDocker, IconSun, IconMoon, IconRefresh, IconSettings } from '@tabler/icons-react'
import { getRollbackIncludeEnv, setRollbackIncludeEnv } from '../settings'
import type { Capabilities } from '../types/api'

interface Props {
  onRefresh: () => void
  capabilities: Capabilities | null
  onModeChange: (mode: 'auto' | 'compose' | 'inspect') => void
}

export function AppHeader({ onRefresh, capabilities, onModeChange }: Props) {
  const { colorScheme, toggleColorScheme } = useMantineColorScheme()
  const [includeEnv, setIncludeEnv] = useState(getRollbackIncludeEnv)

  const handleIncludeEnvChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setRollbackIncludeEnv(e.target.checked)
    setIncludeEnv(e.target.checked)
  }

  const canSwitch = capabilities?.compose_files_available ?? false
  const currentMode = capabilities?.rollback_mode ?? 'auto'

  return (
    <div className="topbar">
      <div className="title">
        <IconBrandDocker size={28} color="var(--accent)" />
        <h1>Paranoid Docker Update</h1>
      </div>
      <div className="spacer" />
      <Group gap="xs">
        <Tooltip label="Refresh">
          <button className="btn small ghost" onClick={onRefresh}>
            <IconRefresh size={18} className="icon-mr" />
            Refresh
          </button>
        </Tooltip>

        <Popover position="bottom-end" withArrow shadow="md" width={260}>
          <Popover.Target>
            <Tooltip label="Settings">
              <button className="btn small ghost"><IconSettings size={18} /></button>
            </Tooltip>
          </Popover.Target>
          <Popover.Dropdown>
            <label className="settings-label">
              <input
                type="checkbox"
                checked={includeEnv}
                onChange={handleIncludeEnvChange}
              />
              Include env vars in rollback files
            </label>

            <Divider my="sm" />

            <Text size="xs" fw={600} c="dimmed" tt="uppercase" mb={6}>Rollback mode</Text>
            {canSwitch ? (
              <SegmentedControl
                fullWidth
                size="xs"
                value={currentMode}
                onChange={v => onModeChange(v as 'auto' | 'compose' | 'inspect')}
                data={[
                  { label: 'Auto', value: 'auto' },
                  { label: 'Compose', value: 'compose' },
                  { label: 'Inspect', value: 'inspect' },
                ]}
              />
            ) : (
              <Text size="xs" c="dimmed">
                {capabilities?.compose_files_available === false
                  ? 'Inspect mode only — compose files not mounted'
                  : 'Loading…'}
              </Text>
            )}
            <Text size="xs" c="dimmed" mt={6}>
              {currentMode === 'compose' && 'Uses docker compose config — full file with interpolation'}
              {currentMode === 'inspect' && 'Reconstructs compose from docker inspect data'}
              {currentMode === 'auto' && 'Auto-selects per stack based on file availability'}
            </Text>
          </Popover.Dropdown>
        </Popover>

        <Tooltip label="Toggle color scheme">
          <button className="btn small ghost" onClick={() => toggleColorScheme()}>
            {colorScheme === 'dark' ? <IconSun size={18} /> : <IconMoon size={18} />}
          </button>
        </Tooltip>
      </Group>
    </div>
  )
}
