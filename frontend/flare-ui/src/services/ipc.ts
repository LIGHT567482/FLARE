const SOCKET_PATH = '/tmp/flared.sock'

class IpcClient {
  private async request(cmd: string, payload: any = {}): Promise<any> {
    try {
      const msg = JSON.stringify({ cmd, ...payload })
      // Note: browser cannot connect to Unix socket directly; this is a placeholder.
      // In Tauri/Electron, bridge via native layer; here we simulate via fetch if proxied.
      // For now return stub if no backend proxy.
      const res = await fetch('/api/flare-ipc', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: msg,
      })
      if (!res.ok) throw new Error('ipc http fail')
      return await res.json()
    } catch (e) {
      // Fallback to noop/offline
      return { offline: true }
    }
  }

  status() { return this.request('status') }
  ping() { return this.request('ping') }
  enqueue(entity_type: string, payload: any) { return this.request('enqueue', { entity_type, payload }) }
  pull_queue(limit=10) { return this.request('pull_queue', { limit }) }
  health() { return this.request('health') }
}

export const ipc = new IpcClient()
