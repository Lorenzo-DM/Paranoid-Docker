import { IconArrowUp } from '@tabler/icons-react'

interface Props {
  updateAvailable: boolean
}

export function UpdateBadge({ updateAvailable }: Props) {
  if (!updateAvailable) {
    return <span className="pill ok">up-to-date</span>
  }
  
  return (
    <span className="pill warn">
      <IconArrowUp size={12} style={{ marginRight: 4 }} />
      Update available
    </span>
  )
}
