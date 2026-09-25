import { useState, useEffect } from 'react'
import { isBuildingResponse } from '../lib/api'

interface EventLog {
  id: string
  type: string
  user: string
  channel: string
  text: string
  timestamp: string
}

function Dashboard() {
  const [events, setEvents] = useState<EventLog[]>([])
  const [loading, setLoading] = useState(true)
  const [building, setBuilding] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const fetchEvents = async () => {
    try {
      const response = await fetch('/api/events')
      if (await isBuildingResponse(response)) {
        setBuilding(true)
        return
      }
      setBuilding(false)
      const data = await response.json()
      if (data.success) {
        setEvents(data.events || [])
      }
      setError(null)
    } catch (err) {
      setError('Failed to fetch events')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchEvents()
    const interval = setInterval(fetchEvents, 5000) // Poll every 5 seconds
    return () => clearInterval(interval)
  }, [])

  const getEventTypeColor = (type: string) => {
    switch (type) {
      case 'app_mention':
        return 'bg-blue-100 text-blue-800'
      case 'dm_received':
        return 'bg-green-100 text-green-800'
      case 'dm_sent':
        return 'bg-purple-100 text-purple-800'
      case 'message_sent':
        return 'bg-yellow-100 text-yellow-800'
      case 'feedback_submitted':
        return 'bg-pink-100 text-pink-800'
      case 'slash_command':
        return 'bg-orange-100 text-orange-800'
      case 'form_opened':
        return 'bg-indigo-100 text-indigo-800'
      default:
        return 'bg-gray-100 text-gray-800'
    }
  }

  const formatTime = (timestamp: string) => {
    return new Date(timestamp).toLocaleTimeString()
  }

  return (
    <div>
      <div className="mb-6">
        <h2 className="text-2xl font-bold text-gray-800">Event Dashboard</h2>
        <p className="text-gray-600 mt-1">Real-time view of Slack events and API calls</p>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-3 gap-4 mb-6">
        <div className="bg-white rounded-lg shadow p-4">
          <h3 className="text-sm font-medium text-gray-500">Total Events</h3>
          <p className="text-3xl font-bold text-gray-800">{events.length}</p>
        </div>
        <div className="bg-white rounded-lg shadow p-4">
          <h3 className="text-sm font-medium text-gray-500">Mentions</h3>
          <p className="text-3xl font-bold text-blue-600">
            {events.filter(e => e.type === 'app_mention').length}
          </p>
        </div>
        <div className="bg-white rounded-lg shadow p-4">
          <h3 className="text-sm font-medium text-gray-500">DMs</h3>
          <p className="text-3xl font-bold text-green-600">
            {events.filter(e => e.type === 'dm_received' || e.type === 'dm_sent').length}
          </p>
        </div>
      </div>

      <div className="bg-white rounded-lg shadow">
        <div className="px-4 py-3 border-b flex justify-between items-center">
          <h3 className="font-medium text-gray-800">Recent Events</h3>
          <button
            onClick={fetchEvents}
            className="text-sm text-blue-600 hover:text-blue-800"
          >
            Refresh
          </button>
        </div>

        {loading || building ? (
          <div className="p-8 text-center text-gray-500">
            {building ? 'Backend is compiling, please wait...' : 'Loading...'}
          </div>
        ) : error ? (
          <div className="p-8 text-center text-red-500">{error}</div>
        ) : events.length === 0 ? (
          <div className="p-8 text-center text-gray-500">
            No events yet. Try mentioning the bot or sending a DM!
          </div>
        ) : (
          <div className="divide-y">
            {events.map((event) => (
              <div key={event.id} className="p-4 hover:bg-gray-50">
                <div className="flex items-start justify-between">
                  <div className="flex-1">
                    <div className="flex items-center gap-2">
                      <span className={`px-2 py-1 rounded text-xs font-medium ${getEventTypeColor(event.type)}`}>
                        {event.type}
                      </span>
                      <span className="text-xs text-gray-500">
                        {formatTime(event.timestamp)}
                      </span>
                    </div>
                    <p className="mt-2 text-gray-800">{event.text || '(no text)'}</p>
                    <div className="mt-1 text-xs text-gray-500">
                      {event.user && <span>User: {event.user}</span>}
                      {event.channel && <span className="ml-3">Channel: {event.channel}</span>}
                    </div>
                  </div>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}

export default Dashboard
