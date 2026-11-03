export interface SyncConflict {
  conflict_id: string
  entity_type: string
  entity_id: string
  event_ids: string[]
  payloads: unknown[]
  created_at: string
  resolved: boolean
}

class IpcClient {
  private async request<T>(cmd: string, payload: Record<string, unknown> = {}): Promise<T> {
    const res = await fetch('/api/flare-ipc', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ cmd, ...payload }),
    })
    if (!res.ok) {
      throw new Error(`FLARE bridge request failed (${res.status})`)
    }
    const data: unknown = await res.json()
    if (typeof data === 'object' && data !== null && 'error' in data && typeof data.error === 'string') {
      const detail = 'detail' in data && typeof data.detail === 'string' ? data.detail : data.error
      throw new Error(`FLARE daemon request failed: ${detail}`)
    }
    return data as T
  }

  status() { return this.request<Record<string, unknown>>('status') }
  ping() { return this.request<Record<string, unknown>>('ping') }
  enqueue(entity_type: string, payload: unknown) {
    return this.request<Record<string, unknown>>('enqueue', { entity_type, payload })
  }
  pull_queue(limit = 10) { return this.request<Record<string, unknown>>('pull_queue', { limit }) }
  health() { return this.request<Record<string, unknown>>('health') }
  conflicts() {
    return this.request<{ conflicts: SyncConflict[] }>('sync_conflicts')
  }
  resolveConflict(conflict_id: string, event_id: string) {
    return this.request<Record<string, unknown>>('resolve_conflict', { conflict_id, event_id })
  }
}

export const ipc = new IpcClient()
