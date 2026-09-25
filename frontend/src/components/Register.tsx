import { useState, useEffect } from 'react'

type PageState = 'loading' | 'form' | 'success' | 'already_registered' | 'closed' | 'error'

export default function Register() {
  const [pageState, setPageState] = useState<PageState>('loading')
  const [name, setName] = useState('')
  const [email, setEmail] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [errorMsg, setErrorMsg] = useState('')

  useEffect(() => {
    fetch('/api/tournament/status')
      .then(r => r.json())
      .then(data => {
        if (!data.registration_open) {
          setPageState('closed')
        } else {
          setPageState('form')
        }
      })
      .catch(() => setPageState('form'))
  }, [])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!name.trim() || !email.trim()) return
    setSubmitting(true)
    setErrorMsg('')

    try {
      const res = await fetch('/api/register', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name: name.trim(), email: email.trim() }),
      })
      const data = await res.json()

      if (res.status === 201) {
        setPageState('success')
      } else if (data.error === 'already_registered') {
        setPageState('already_registered')
      } else if (data.error === 'registration_closed') {
        setPageState('closed')
      } else {
        setErrorMsg(data.error || 'Something went wrong. Please try again.')
      }
    } catch {
      setErrorMsg('Network error. Please try again.')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="min-h-screen bg-gradient-to-br from-green-50 to-emerald-100 flex items-center justify-center p-4">
      <div className="bg-white rounded-2xl shadow-lg p-8 w-full max-w-md">
        <div className="text-center mb-6">
          <div className="text-5xl mb-3">🏓</div>
          <h1 className="text-2xl font-bold text-gray-900">Applied Ping Pong Tournament</h1>
          <p className="text-gray-500 mt-1 text-sm">Doubles tournament — register to join!</p>
        </div>

        {pageState === 'loading' && (
          <div className="text-center text-gray-400 py-8">Loading...</div>
        )}

        {pageState === 'form' && (
          <form onSubmit={handleSubmit} className="space-y-4">
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Full Name</label>
              <input
                type="text"
                value={name}
                onChange={e => setName(e.target.value)}
                placeholder="Jane Smith"
                required
                className="w-full border border-gray-300 rounded-lg px-3 py-2 focus:outline-none focus:ring-2 focus:ring-green-500"
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Applied Email</label>
              <input
                type="email"
                value={email}
                onChange={e => setEmail(e.target.value)}
                placeholder="jane@applied.dev"
                required
                className="w-full border border-gray-300 rounded-lg px-3 py-2 focus:outline-none focus:ring-2 focus:ring-green-500"
              />
            </div>
            {errorMsg && (
              <p className="text-red-500 text-sm">{errorMsg}</p>
            )}
            <button
              type="submit"
              disabled={submitting}
              className="w-full bg-green-600 hover:bg-green-700 disabled:bg-green-300 text-white font-semibold py-2 px-4 rounded-lg transition-colors"
            >
              {submitting ? 'Registering...' : 'Register Me! 🎾'}
            </button>
          </form>
        )}

        {pageState === 'success' && (
          <div className="text-center py-6">
            <div className="text-4xl mb-3">🎉</div>
            <h2 className="text-xl font-bold text-gray-900">You're in!</h2>
            <p className="text-gray-500 mt-2">We'll announce teams and match schedules soon. Stay tuned!</p>
          </div>
        )}

        {pageState === 'already_registered' && (
          <div className="text-center py-6">
            <div className="text-4xl mb-3">✅</div>
            <h2 className="text-xl font-bold text-gray-900">Already registered!</h2>
            <p className="text-gray-500 mt-2">You're already signed up. See you on the court!</p>
          </div>
        )}

        {pageState === 'closed' && (
          <div className="text-center py-6">
            <div className="text-4xl mb-3">🔒</div>
            <h2 className="text-xl font-bold text-gray-900">Registration is closed</h2>
            <p className="text-gray-500 mt-2">
              Sign-ups have ended. Check the{' '}
              <a href="/bracket" className="text-green-600 underline">live bracket</a> for match progress!
            </p>
          </div>
        )}
      </div>
    </div>
  )
}
