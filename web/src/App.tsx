import { useState } from 'react'
import { setBaseURL } from './api/client'
import Login from './pages/Login'
import Shell from './pages/Shell'

export default function App() {
  const [authed, setAuthed] = useState(false)

  function handleLogin(serverURL: string) {
    setBaseURL(serverURL)
    setAuthed(true)
  }

  function handleLogout() {
    setAuthed(false)
  }

  if (!authed) {
    return <Login onLogin={handleLogin} />
  }

  return <Shell onLogout={handleLogout} />
}
