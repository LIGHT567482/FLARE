import { useCallback, useEffect, useRef, useState } from 'react'
import { db } from './db'
import { ipc, type SyncConflict } from './services/ipc'
import flareLogo from '../../../white.svg'

interface QueueItem {
  queue_id: string
  entity_type: string
  payload: unknown
  created_at: string
  status: 'pending' | 'failed'
  attempts: number
}

interface QueueResponse {
  items?: QueueItem[]
  error?: string
}

interface DaemonStatus {
  engine?: string
  socket?: string
  storage?: string
  error?: string
}

export default function App() {
  const [status, setStatus] = useState<DaemonStatus | null>(null)
  const [queue, setQueue] = useState<QueueItem[]>([])
  const [count, setCount] = useState(0)
  const [error, setError] = useState<string | null>(null)
  const [conflictError, setConflictError] = useState<string | null>(null)
  const [conflicts, setConflicts] = useState<SyncConflict[]>([])
  const [busy, setBusy] = useState(false)
  const [resolving, setResolving] = useState<string | null>(null)
  const refreshing = useRef(false)

  const refresh = useCallback(async () => {
    if (refreshing.current) return
    refreshing.current = true
    try {
      try {
        const [daemon, pending] = await Promise.all([
          ipc.status(),
          ipc.pull_queue(100) as Promise<QueueResponse>,
        ])
        if (typeof daemon.error === 'string') throw new Error(daemon.error)
        if (pending.error) throw new Error(pending.error)
        setStatus({
          engine: typeof daemon.engine === 'string' ? daemon.engine : undefined,
          socket: typeof daemon.socket === 'string' ? daemon.socket : undefined,
          storage: typeof daemon.storage === 'string' ? daemon.storage : undefined,
        })
        setQueue(Array.isArray(pending.items) ? pending.items : [])
        setError(null)
        try {
          const remote = await ipc.conflicts()
          setConflicts(remote.conflicts.filter(conflict => !conflict.resolved))
          setConflictError(null)
        } catch (reason) {
          setConflicts([])
          setConflictError(reason instanceof Error ? reason.message : 'Remote conflict list unavailable')
        }
      } catch (reason) {
        setStatus(null)
        setConflicts([])
        setError(reason instanceof Error ? reason.message : 'Unable to reach the FLARE bridge')
      }

      try {
        setCount(await db.counter.count())
      } catch (reason) {
        setError(reason instanceof Error ? reason.message : 'Unable to read local data')
      }
    } finally {
      refreshing.current = false
    }
  }, [])

  useEffect(() => {
    void refresh()
    const timer = window.setInterval(() => void refresh(), 5000)
    return () => window.clearInterval(timer)
  }, [refresh])

  async function add() {
    setBusy(true)
    try {
      const value = Date.now()
      await db.counter.add({ value })
      setCount(await db.counter.count())
      await ipc.enqueue('demo.counter', {
        entity_id: 'demo-counter',
        value,
        updated_at: new Date(value).toISOString(),
      })
      await refresh()
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : 'Unable to save the event')
    } finally {
      setBusy(false)
    }
  }

  async function resolveConflict(conflict: SyncConflict, eventId: string) {
    setResolving(conflict.conflict_id)
    try {
      await ipc.resolveConflict(conflict.conflict_id, eventId)
      await refresh()
    } catch (reason) {
      setConflictError(reason instanceof Error ? reason.message : 'Unable to resolve conflict')
    } finally {
      setResolving(null)
    }
  }

  return (
    <main className="mx-auto max-w-4xl space-y-6 p-6">
      <header className="flex flex-wrap items-center justify-between gap-4">
        <div className="flex items-center gap-3">
          <img src={flareLogo} alt="FLARE logo" className="h-12 w-12" />
          <div>
            <h1 className="text-3xl font-bold">FLARE</h1>
            <p className="mt-1 text-sm text-gray-400">Local-first operations dashboard</p>
          </div>
        </div>
        <button
          onClick={() => void refresh()}
          className="rounded border border-gray-700 px-3 py-2 text-sm hover:bg-gray-800"
        >
          Refresh status
        </button>
      </header>

      {error && (
        <div role="alert" className="rounded border border-red-800 bg-red-950 p-3 text-sm text-red-200">
          {error}
        </div>
      )}

      <section aria-label="System status" className="grid gap-4 sm:grid-cols-3">
        <div className="rounded border border-gray-800 bg-gray-900 p-4">
          <h2 className="text-sm text-gray-400">Daemon</h2>
          <p className={status ? 'mt-2 font-semibold text-green-400' : 'mt-2 font-semibold text-red-400'}>
            {status ? 'Connected' : 'Unavailable'}
          </p>
          <p className="mt-1 break-all text-xs text-gray-500">{status?.socket ?? 'Check bridge configuration'}</p>
        </div>
        <div className="rounded border border-gray-800 bg-gray-900 p-4">
          <h2 className="text-sm text-gray-400">Local items</h2>
          <p className="mt-2 text-2xl font-semibold">{count}</p>
          <p className="mt-1 text-xs text-gray-500">Stored in browser IndexedDB</p>
        </div>
        <div className="rounded border border-gray-800 bg-gray-900 p-4">
          <h2 className="text-sm text-gray-400">Events needing sync (up to 100 shown)</h2>
          <p className="mt-2 text-2xl font-semibold">{queue.length}</p>
          <p className="mt-1 text-xs text-gray-500">Remote delivery requires sync configuration</p>
        </div>
      </section>

      <section className="rounded border border-gray-800 bg-gray-900 p-4">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div>
            <h2 className="font-semibold">Offline queue</h2>
            <p className="text-sm text-gray-400">Failed deliveries remain visible and retry with backoff.</p>
          </div>
          <button
            onClick={() => void add()}
            disabled={busy}
            className="rounded bg-blue-600 px-3 py-2 text-sm font-medium hover:bg-blue-500 disabled:opacity-50"
          >
            Add demo event
          </button>
        </div>
        {queue.length === 0 ? (
          <p className="mt-4 text-sm text-gray-500">No pending events, or the daemon is unavailable.</p>
        ) : (
          <ul className="mt-4 divide-y divide-gray-800">
            {queue.map(item => (
              <li key={item.queue_id} className="grid gap-1 py-3 text-sm sm:grid-cols-3">
                <span className="font-medium">
                  {item.entity_type} · <span className={item.status === 'failed' ? 'text-amber-400' : 'text-blue-300'}>
                    {item.status}
                  </span>
                </span>
                <code className="break-all text-gray-400">{item.queue_id}</code>
                <time className="text-gray-500">{item.created_at}</time>
              </li>
            ))}
          </ul>
        )}
      </section>

      <section className="rounded border border-gray-800 bg-gray-900 p-4">
        <h2 className="font-semibold">Conflicts needing review ({conflicts.length})</h2>
        <p className="mt-1 text-sm text-gray-400">
          Both versions are preserved remotely. Choose which version should become the resolution.
        </p>
        {conflictError && <p role="status" className="mt-3 text-sm text-amber-300">{conflictError}</p>}
        {!conflictError && conflicts.length === 0 && (
          <p className="mt-3 text-sm text-gray-500">No unresolved conflicts.</p>
        )}
        <ul className="mt-3 space-y-4">
          {conflicts.map(conflict => (
            <li key={conflict.conflict_id} className="rounded border border-amber-900/70 p-3">
              <p className="text-sm font-medium">
                {conflict.entity_type} / <code>{conflict.entity_id}</code>
              </p>
              <div className="mt-3 grid gap-3 md:grid-cols-2">
                {conflict.event_ids.map((eventId, index) => (
                  <div key={eventId} className="min-w-0 rounded bg-gray-950 p-3">
                    <p className="break-all text-xs text-gray-400">Event {eventId}</p>
                    <pre className="mt-2 max-h-48 overflow-auto whitespace-pre-wrap break-words text-xs">
                      {JSON.stringify(conflict.payloads[index], null, 2)}
                    </pre>
                    <button
                      onClick={() => void resolveConflict(conflict, eventId)}
                      disabled={resolving !== null}
                      className="mt-3 rounded bg-amber-700 px-3 py-2 text-sm hover:bg-amber-600 disabled:opacity-50"
                    >
                      {resolving === conflict.conflict_id ? 'Resolving…' : 'Keep this version'}
                    </button>
                  </div>
                ))}
              </div>
            </li>
          ))}
        </ul>
      </section>

      <p className="text-xs text-gray-500">
        Daemon: {status?.engine ?? 'not connected'} · Storage: {status?.storage ?? 'unknown'}
      </p>
    </main>
  )
}
