import { useEffect, useRef } from 'react'
import { ScrollArea } from '@mantine/core'

interface Props {
  lines: string[]
}

export function PullProgressLog({ lines }: Props) {
  const bottomRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [lines])

  return (
    <ScrollArea h={260} type="auto" className="glass-inset" p="xs">
      <div style={{ 
        fontFamily: 'var(--mono)', 
        fontSize: '12px', 
        whiteSpace: 'pre-wrap', 
        color: 'var(--ink-2)',
        minHeight: '240px' 
      }}>
        {lines.join('\n') || 'Waiting for output...'}
      </div>
      <div ref={bottomRef} />
    </ScrollArea>
  )
}
