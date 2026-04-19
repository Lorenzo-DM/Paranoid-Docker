import { useEffect, useRef } from 'react'
import { Terminal } from 'xterm'
import { FitAddon } from '@xterm/addon-fit'
import 'xterm/css/xterm.css'

interface Props {
  onReady: (write: (line: string) => void) => void
}

export function TerminalView({ onReady }: Props) {
  const containerRef = useRef<HTMLDivElement>(null)
  const termRef = useRef<Terminal | null>(null)

  useEffect(() => {
    if (!containerRef.current) return

    const term = new Terminal({
      convertEol: true,
      disableStdin: true,
      scrollback: 5000,
      theme: {
        background: '#1a1b1e',
        foreground: '#c1c2c5',
      },
      fontSize: 13,
      fontFamily: 'Menlo, Monaco, "Courier New", monospace',
    })

    const fitAddon = new FitAddon()
    term.loadAddon(fitAddon)
    term.open(containerRef.current)
    fitAddon.fit()
    termRef.current = term

    onReady((line: string) => {
      termRef.current?.writeln(line)
    })

    const observer = new ResizeObserver(() => fitAddon.fit())
    observer.observe(containerRef.current)

    return () => {
      observer.disconnect()
      term.dispose()
      termRef.current = null
    }
  }, [])  // eslint-disable-line react-hooks/exhaustive-deps

  return <div ref={containerRef} className="terminal-container" />
}
