import { useState, useEffect, useCallback } from 'react'
import { fetchUpdateLog } from '../api/containers'

export function useUpdateLog() {
  const [log, setLog] = useState<Record<string, string>>({})
  const [error, setError] = useState<string | null>(null)

  const refresh = useCallback(async () => {
    try {
      const data = await fetchUpdateLog()
      setLog(data)
      setError(null)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to fetch update log')
    }
  }, [])

  useEffect(() => { refresh() }, [refresh])

  return { log, error, refresh }
}
