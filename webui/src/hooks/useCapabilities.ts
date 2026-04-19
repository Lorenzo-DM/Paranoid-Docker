import { useState, useEffect, useCallback } from 'react'
import type { Capabilities } from '../types/api'
import { fetchCapabilities, setRollbackMode } from '../api/containers'

export function useCapabilities() {
  const [capabilities, setCapabilities] = useState<Capabilities | null>(null)

  const refresh = useCallback(async () => {
    try {
      const data = await fetchCapabilities()
      setCapabilities(data)
    } catch {
      // capabilities endpoint failure is non-fatal
    }
  }, [])

  const changeMode = useCallback(async (mode: 'auto' | 'compose' | 'inspect') => {
    await setRollbackMode(mode)
    await refresh()
  }, [refresh])

  useEffect(() => {
    refresh()
  }, [refresh])

  return { capabilities, changeMode, refresh }
}
