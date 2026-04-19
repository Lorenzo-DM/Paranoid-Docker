import { useState } from 'react'
import { Group, Tooltip, Popover, useMantineColorScheme } from '@mantine/core'
import { IconBrandDocker, IconSun, IconMoon, IconRefresh, IconSettings } from '@tabler/icons-react'
import { getRollbackIncludeEnv, setRollbackIncludeEnv } from '../settings'

interface Props {
  onRefresh: () => void
}

export function AppHeader({ onRefresh }: Props) {
  const { colorScheme, toggleColorScheme } = useMantineColorScheme()
  const [includeEnv, setIncludeEnv] = useState(getRollbackIncludeEnv)

  const handleIncludeEnvChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setRollbackIncludeEnv(e.target.checked)
    setIncludeEnv(e.target.checked)
  }

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

        <Popover position="bottom-end" withArrow shadow="md">
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
