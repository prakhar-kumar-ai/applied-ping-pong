import { useState } from 'react'
import { useStatus } from '../App'

function SendDM() {
  const status = useStatus()
  const [userId, setUserId] = useState('')
  const [text, setText] = useState('')
  const [loading, setLoading] = useState(false)
  const [result, setResult] = useState<{ success: boolean; message: string } | null>(null)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setLoading(true)
    setResult(null)

    try {
      const response = await fetch('/api/send-dm', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ user_id: userId, text }),
      })

      const data = await response.json()

      if (data.success) {
        setResult({ success: true, message: `DM sent successfully! Channel: ${data.channel}` })
        setText('')
      } else {
        setResult({ success: false, message: data.error || 'Failed to send DM' })
      }
    } catch (err) {
      setResult({ success: false, message: 'Network error' })
    } finally {
      setLoading(false)
    }
  }

  return (
    <div>
      <div className="mb-6">
        <h2 className="text-2xl font-bold text-gray-800">Send Direct Message</h2>
        <p className="text-gray-600 mt-1">Send a DM to any Slack user</p>
      </div>

      <div className="bg-white rounded-lg shadow p-6 max-w-xl">
        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">
              User ID
            </label>
            <input
              type="text"
              value={userId}
              onChange={(e) => setUserId(e.target.value)}
              placeholder="U01234567"
              className="w-full px-3 py-2 border rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
              required
            />
            <p className="mt-1 text-xs text-gray-500">
              Slack user ID (starts with U). You can find this in user profiles.
            </p>
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">
              Message
            </label>
            <textarea
              value={text}
              onChange={(e) => setText(e.target.value)}
              placeholder="Enter your message..."
              rows={4}
              className="w-full px-3 py-2 border rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
              required
            />
          </div>

          <button
            type="submit"
            disabled={loading || !status.ready}
            className="w-full bg-purple text-white py-2 px-4 rounded-lg hover:opacity-90 disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {loading ? 'Sending...' : !status.ready ? 'Bot Not Connected' : 'Send DM'}
          </button>
        </form>

        {result && (
          <div className={`mt-4 p-3 rounded-lg ${result.success ? 'bg-green-100 text-green-800' : 'bg-red-100 text-red-800'}`}>
            {result.message}
          </div>
        )}

        <div className="mt-6 p-4 bg-blue-50 rounded-lg">
          <h4 className="font-medium text-blue-800 mb-2">API Usage</h4>
          <p className="text-sm text-blue-700 mb-2">
            Other apps can send DMs through this bot using the API:
          </p>
          <pre className="text-xs bg-blue-100 p-2 rounded overflow-x-auto">
{`POST /api/send-dm
Content-Type: application/json

{
  "user_id": "U01234567",
  "text": "Hello from another app!"
}`}
          </pre>
        </div>
      </div>
    </div>
  )
}

export default SendDM
