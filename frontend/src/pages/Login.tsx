import { useState } from 'react'
import { KeyRound, Mail, User } from 'lucide-react'

interface Props {
  onLogin: (token: string) => void
}

export default function Login({ onLogin }: Props) {
  const [email, setEmail] = useState('')
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [isRegister, setIsRegister] = useState(false)
  const [error, setError] = useState('')

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setError('')

    const endpoint = isRegister ? '/rest/api/3/auth/register' : '/rest/api/3/auth/login'
    const body = isRegister
      ? { email, username, password }
      : { email, password }

    try {
      const res = await fetch(endpoint, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body)
      })
      const data = await res.json()
      if (!res.ok) { setError(data.error || 'Failed'); return }

      if (isRegister) {
        const loginRes = await fetch('/rest/api/3/auth/login', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ email, password })
        })
        const loginData = await loginRes.json()
        localStorage.setItem('token', loginData.token)
        onLogin(loginData.token)
      } else {
        localStorage.setItem('token', data.token)
        onLogin(data.token)
      }
    } catch {
      setError('Connection error')
    }
  }

  return (
    <div className="min-h-screen bg-surface flex items-center justify-center">
      <div className="w-full max-w-sm">
        <div className="text-center mb-8">
          <h1 className="text-2xl font-bold text-primary">Open Jira</h1>
          <p className="text-sm text-secondary mt-1">
            {isRegister ? 'Create your account' : 'Log in to continue'}
          </p>
        </div>

        <form onSubmit={handleSubmit} className="bg-card rounded-sm border border-default p-6 space-y-4">
          {error && <div className="text-sm text-accent-red bg-red-50 border border-red-200 rounded-sm px-3 py-2">{error}</div>}

          <div>
            <label className="block text-xs text-secondary font-medium mb-1">Email</label>
            <div className="flex items-center gap-2 bg-input border border-default rounded-sm px-3 h-9">
              <Mail className="w-4 h-4 text-subtlest" />
              <input className="bg-transparent text-sm text-primary outline-none flex-1 placeholder:text-subtlest"
                type="email" value={email} onChange={e => setEmail(e.target.value)} placeholder="you@example.com" required />
            </div>
          </div>

          {isRegister && (
            <div>
              <label className="block text-xs text-secondary font-medium mb-1">Username</label>
              <div className="flex items-center gap-2 bg-input border border-default rounded-sm px-3 h-9">
                <User className="w-4 h-4 text-subtlest" />
                <input className="bg-transparent text-sm text-primary outline-none flex-1 placeholder:text-subtlest"
                  value={username} onChange={e => setUsername(e.target.value)} placeholder="username" required />
              </div>
            </div>
          )}

          <div>
            <label className="block text-xs text-secondary font-medium mb-1">Password</label>
            <div className="flex items-center gap-2 bg-input border border-default rounded-sm px-3 h-9">
              <KeyRound className="w-4 h-4 text-subtlest" />
              <input className="bg-transparent text-sm text-primary outline-none flex-1 placeholder:text-subtlest"
                type="password" value={password} onChange={e => setPassword(e.target.value)} placeholder="••••••" required />
            </div>
          </div>

          <button type="submit" className="w-full bg-accent-blue text-white text-sm font-medium rounded-sm h-9 hover:brightness-110 transition-all">
            {isRegister ? 'Sign up' : 'Log in'}
          </button>

          <p className="text-center text-xs text-secondary">
            {isRegister ? (
              <>Already have an account? <button type="button" onClick={() => setIsRegister(false)} className="text-link hover:underline">Log in</button></>
            ) : (
              <>New here? <button type="button" onClick={() => setIsRegister(true)} className="text-link hover:underline">Create account</button></>
            )}
          </p>
        </form>
      </div>
    </div>
  )
}
