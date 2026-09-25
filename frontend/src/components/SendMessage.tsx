import { useState } from 'react'
import { useStatus } from '../App'

function SendMessage() {
  const status = useStatus()
  const [channel, setChannel] = useState('')
  const [text, setText] = useState('')
  const [threadTs, setThreadTs] = useState('')
  const [includeButton, setIncludeButton] = useState(false)
  const [loading, setLoading] = useState(false)
  const [result, setResult] = useState<{ success: boolean; message: string } | null>(null)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setLoading(true)
    setResult(null)

    try {
      const endpoint = includeButton ? '/api/send-message-with-button' : '/api/send-message'
      const body: Record<string, string> = { channel, text }
      if (threadTs && !includeButton) {
        body.thread_ts = threadTs
      }
      if (includeButton) {
        body.button_text = 'Submit Feedback'
        body.action_id = 'open_feedback_form'
      }

      const response = await fetch(endpoint, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      })

      const data = await response.json()

      if (data.success) {
        setResult({ success: true, message: `Message sent! Timestamp: ${data.timestamp}` })
        setText('')
        setThreadTs('')
      } else {
        setResult({ success: false, message: data.error || 'Failed to send message' })
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
        <h2 className="text-2xl font-bold text-gray-800">Send Channel Message</h2>
        <p className="text-gray-600 mt-1">Send a message to any Slack channel</p>
      </div>

      <div className="bg-white rounded-lg shadow p-6 max-w-xl">
        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">
              Channel ID
            </label>
            <input
              type="text"
              value={channel}
              onChange={(e) => setChannel(e.target.value)}
              placeholder="C01234567 or #channel-name"
              className="w-full px-3 py-2 border rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
              required
            />
            <p className="mt-1 text-xs text-gray-500">
              Use channel ID (starts with C) or channel name
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

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">
              Thread Timestamp (optional)
            </label>
            <input
              type="text"
              value={threadTs}
              onChange={(e) => setThreadTs(e.target.value)}
              placeholder="1234567890.123456"
              className="w-full px-3 py-2 border rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
              disabled={includeButton}
            />
            <p className="mt-1 text-xs text-gray-500">
              Leave empty for a new message, or provide thread_ts to reply in thread
            </p>
          </div>

          <div className="flex items-center gap-2">
            <input
              type="checkbox"
              id="includeButton"
              checked={includeButton}
              onChange={(e) => setIncludeButton(e.target.checked)}
              className="rounded text-blue-600"
            />
            <label htmlFor="includeButton" className="text-sm text-gray-700">
              Include "Submit Feedback" button (opens feedback form modal)
            </label>
          </div>

          <button
            type="submit"
            disabled={loading || !status.ready}
            className="w-full bg-blue-600 text-white py-2 px-4 rounded-lg hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {loading ? 'Sending...' : !status.ready ? 'Bot Not Connected' : 'Send Message'}
          </button>
        </form>

        {result && (
          <div className={`mt-4 p-3 rounded-lg ${result.success ? 'bg-green-100 text-green-800' : 'bg-red-100 text-red-800'}`}>
            {result.message}
          </div>
        )}
      </div>
    </div>
  )
}

export default SendMessage
