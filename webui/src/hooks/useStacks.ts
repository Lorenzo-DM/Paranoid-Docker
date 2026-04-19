import { useState, useEffect, useCallback } from 'react'
import type { ComposeStack } from '../types/api'
import { fetchStacks } from '../api/containers'

export function useStacks(intervalMs = 10_000) {
  const [stacks, setStacks] = useState<ComposeStack[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const refresh = useCallback(async () => {
    try {
      const data = await fetchStacks()
      setStacks(data)
      setError(null)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Unknown error')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    refresh()
    const id = setInterval(refresh, intervalMs)
    return () => clearInterval(id)
  }, [refresh, intervalMs])

  return { stacks, loading, error, refresh }
}
