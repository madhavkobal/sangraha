import { useState } from 'react'
import { api, setBaseURL } from '../api/client'

interface Props {
  onLogin: (serverURL: string) => void
}

export default function Login({ onLogin }: Props) {
  const [serverURL, setServerURL] = useState('http://localhost:9001')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  async function handleConnect(e: React.FormEvent) {
    e.preventDefault()
    setError('')
    setLoading(true)
    try {
      setBaseURL(serverURL)
      await api.health()
      onLogin(serverURL)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Cannot connect to server')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="min-h-screen flex items-center justify-center bg-bg">
      <div className="bg-surface border border-border rounded-lg p-8 w-full max-w-sm">
        <h1 className="text-2xl font-bold mb-1">संग्रह</h1>
        <p className="text-muted text-sm mb-6">sangraha admin dashboard</p>

        {error && (
          <div className="bg-red-900/20 border border-danger text-danger rounded p-3 mb-4 text-sm">
            {error}
          </div>
        )}

        <form onSubmit={handleConnect} className="space-y-4">
          <div>
            <label className="block text-xs text-muted uppercase tracking-wide mb-1.5">
              Server URL
            </label>
            <input
              type="text"
              value={serverURL}
              onChange={e => setServerURL(e.target.value)}
              className="w-full bg-bg border border-border rounded px-3 py-2 text-sm text-white focus:outline-none focus:border-accent"
              required
            />
          </div>

          <button
            type="submit"
            disabled={loading}
            className="w-full bg-accent text-bg font-semibold py-2.5 rounded text-sm hover:opacity-90 disabled:opacity-50 transition"
          >
            {loading ? 'Connecting…' : 'Connect'}
          </button>
        </form>

        <p className="text-muted text-xs mt-4 text-center">
          Connects to the sangraha admin port (default :9001)
        </p>
      </div>
    </div>
  )
}
