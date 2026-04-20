import { useState, useEffect, useCallback } from 'react'
import { fetchUpdateLog } from '../api/containers'

export function useUpdateLog() {
  const [log, setLog] = useState<Record<string, string>>({})

  const refresh = useCallback(async () => {
    try {
      const data = await fetchUpdateLog()
      setLog(data)
    } catch {
      // non-fatal
    }
  }, [])

  useEffect(() => { refresh() }, [refresh])

  return { log, refresh }
}
