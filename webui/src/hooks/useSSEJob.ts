import { useCallback, useEffect, useRef, useState } from 'react'

export type JobPhase = 'idle' | 'running' | 'reconnecting' | 'done' | 'error'

const MAX_RECONNECT_ATTEMPTS = 10

export interface RunSSEJobOptions<E> {
  /** POST that starts the job */
  trigger: () => Promise<unknown>
  /** opens the matching -status EventSource */
  createEventSource: () => EventSource
  onProgress: (evt: E) => void
  onReconnecting?: (attempt: number) => void
  onReconnected?: () => void
}

export interface RunningJob<E> {
  promise: Promise<E>
  abort: () => void
}

/**
 * Runs one backend job over SSE. Transport drops are NOT terminal: the
 * EventSource auto-reconnects and the server replays missed events via
 * Last-Event-ID. Only a server-sent `error` event (which carries data)
 * or exhausting reconnect attempts rejects the promise.
 */
export function runSSEJob<E>(opts: RunSSEJobOptions<E>): RunningJob<E> {
  let es: EventSource | null = null
  let aborted = false
  let attempts = 0

  const promise = new Promise<E>((resolve, reject) => {
    opts.trigger()
      .then(() => {
        if (aborted) return
        es = opts.createEventSource()

        es.addEventListener('progress', (e) => {
          attempts = 0
          opts.onReconnected?.()
          opts.onProgress(JSON.parse((e as MessageEvent).data))
        })

        es.addEventListener('done', (e) => {
          es?.close()
          resolve(JSON.parse((e as MessageEvent).data))
        })

        es.addEventListener('error', (e) => {
          const data = (e as MessageEvent).data
          if (data !== undefined) {
            // server-sent terminal error event
            es?.close()
            const evt = JSON.parse(data || '{}')
            reject(new Error(evt.error ?? evt.message ?? 'Operation failed'))
            return
          }
          // transport error: the browser reconnects with Last-Event-ID
          attempts++
          if (attempts > MAX_RECONNECT_ATTEMPTS) {
            es?.close()
            reject(new Error('Connection lost — the operation may still be running on the server'))
            return
          }
          opts.onReconnecting?.(attempts)
        })
      })
      .catch((err) => {
        if (!aborted) reject(err instanceof Error ? err : new Error(String(err)))
      })
  })

  return {
    promise,
    abort: () => {
      aborted = true
      es?.close()
    },
  }
}

export interface UseSSEJobResult<E> {
  phase: JobPhase
  error: string | null
  start: (opts: RunSSEJobOptions<E> & { onDone?: (evt: E) => void }) => void
  reset: () => void
}

/**
 * Modal-friendly wrapper around runSSEJob: exposes the phase machine
 * confirm/idle → running → reconnecting → done|error.
 */
export function useSSEJob<E>(): UseSSEJobResult<E> {
  const [phase, setPhase] = useState<JobPhase>('idle')
  const [error, setError] = useState<string | null>(null)
  const abortRef = useRef<(() => void) | null>(null)

  useEffect(() => () => abortRef.current?.(), [])

  const start = useCallback((opts: RunSSEJobOptions<E> & { onDone?: (evt: E) => void }) => {
    setPhase('running')
    setError(null)
    const job = runSSEJob<E>({
      ...opts,
      onReconnecting: (attempt) => {
        setPhase('reconnecting')
        opts.onReconnecting?.(attempt)
      },
      onReconnected: () => {
        setPhase((p) => (p === 'reconnecting' ? 'running' : p))
        opts.onReconnected?.()
      },
    })
    abortRef.current = job.abort
    job.promise
      .then((evt) => {
        setPhase('done')
        opts.onDone?.(evt)
      })
      .catch((err: Error) => {
        setPhase('error')
        setError(err.message)
      })
  }, [])

  const reset = useCallback(() => {
    abortRef.current?.()
    abortRef.current = null
    setPhase('idle')
    setError(null)
  }, [])

  return { phase, error, start, reset }
}
