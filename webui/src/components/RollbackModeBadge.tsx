import { Tooltip } from '@mantine/core'
import { IconFileCode, IconMicroscope } from '@tabler/icons-react'

interface Props {
  mode: 'compose' | 'inspect'
  size?: 'sm' | 'xs'
}

export function RollbackModeBadge({ mode, size = 'sm' }: Props) {
  const isCompose = mode === 'compose'
  const iconSize = size === 'xs' ? 11 : 13

  return (
    <Tooltip
      label={isCompose ? 'Rollback from compose file (docker compose config)' : 'Rollback from docker inspect'}
      position="top"
      withArrow
    >
      <span className={`pill rollback-mode-badge rollback-mode-badge--${mode} rollback-mode-badge--${size}`}>
        {isCompose
          ? <><IconFileCode size={iconSize} /> compose</>
          : <><IconMicroscope size={iconSize} /> inspect</>
        }
      </span>
    </Tooltip>
  )
}
