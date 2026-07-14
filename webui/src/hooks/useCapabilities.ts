import { useState, useEffect, useCallback } from 'react'
import type { Capabilities } from '../types/api'
import { fetchCapabilities, setRollbackMode } from '../api/containers'

export function useCapabilities() {
  const [capabilities, setCapabilities] = useState<Capabilities | null>(null)
  const [error, setError] = useState<string | null>(null)

  const refresh = useCallback(async () => {
    try {
      const data = await fetchCapabilities()
      setCapabilities(data)
      setError(null)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to fetch capabilities')
    }
  }, [])

  const changeMode = useCallback(async (mode: 'auto' | 'compose' | 'inspect') => {
    await setRollbackMode(mode)
    await refresh()
  }, [refresh])

  useEffect(() => {
    refresh()
  }, [refresh])

  return { capabilities, error, changeMode, refresh }
}
