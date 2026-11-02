import { useEffect, useState } from 'react'
import { db } from './db'
import { ipc } from './services/ipc'

export default function App() {
  const [status, setStatus] = useState<any>(null)
  const [count, setCount] = useState(0)

  useEffect(() => {
    ipc.status().then(setStatus).catch(() => setStatus({error: 'no daemon'}))
    db.counter.count().then(c => setCount(c))
  }, [])

  async function add() {
    await db.counter.add({ value: Date.now() })
    setCount(await db.counter.count())
    await ipc.enqueue('counter', { ts: Date.now() })
  }

  return (
    <div className="p-6 space-y-4">
      <h1 className="text-2xl font-bold">FLARE UI</h1>
      <div className="text-sm text-gray-400">Offline-first • Dexie.js + IPC to flared</div>
      <div className="bg-gray-900 p-4 rounded border border-gray-800">
        <div>Daemon status: <pre className="text-xs">{JSON.stringify(status, null, 2)}</pre></div>
        <div>Local items: {count}</div>
        <button onClick={add} className="mt-2 px-3 py-1 bg-blue-600 rounded">Queue offline event</button>
      </div>
    </div>
  )
}
