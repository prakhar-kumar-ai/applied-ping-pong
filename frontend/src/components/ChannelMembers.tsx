import { useState } from 'react'
import { useStatus } from '../App'

interface UserInfo {
  id: string
  name: string
  real_name: string
  email: string
  title: string
  image: string
}

function ChannelMembers() {
  const status = useStatus()
  const [channelId, setChannelId] = useState('')
  const [members, setMembers] = useState<string[]>([])
  const [selectedUser, setSelectedUser] = useState<UserInfo | null>(null)
  const [loading, setLoading] = useState(false)
  const [userLoading, setUserLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const fetchMembers = async (e: React.FormEvent) => {
    e.preventDefault()
    setLoading(true)
    setError(null)
    setMembers([])
    setSelectedUser(null)

    try {
      const response = await fetch(`/api/members?channel=${encodeURIComponent(channelId)}`)
      const data = await response.json()

      if (data.success) {
        setMembers(data.members || [])
      } else {
        setError(data.error || 'Failed to fetch members')
      }
    } catch (err) {
      setError('Network error')
    } finally {
      setLoading(false)
    }
  }

  const fetchUserInfo = async (userId: string) => {
    setUserLoading(true)
    setSelectedUser(null)

    try {
      const response = await fetch(`/api/user?user_id=${encodeURIComponent(userId)}`)
      const data = await response.json()

      if (data.success) {
        setSelectedUser(data.user)
      }
    } catch (err) {
      // Ignore errors for user info
    } finally {
      setUserLoading(false)
    }
  }

  return (
    <div>
      <div className="mb-6">
        <h2 className="text-2xl font-bold text-gray-800">Channel Members</h2>
        <p className="text-gray-600 mt-1">List all members in a Slack channel</p>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <div className="bg-white rounded-lg shadow p-6">
          <form onSubmit={fetchMembers} className="space-y-4">
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">
                Channel ID
              </label>
              <input
                type="text"
                value={channelId}
                onChange={(e) => setChannelId(e.target.value)}
                placeholder="C01234567"
                className="w-full px-3 py-2 border rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
                required
              />
              <p className="mt-1 text-xs text-gray-500">
                Channel ID starts with C. Find it in channel details.
              </p>
            </div>

            <button
              type="submit"
              disabled={loading || !status.ready}
              className="w-full bg-green-600 text-white py-2 px-4 rounded-lg hover:bg-green-700 disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {loading ? 'Loading...' : !status.ready ? 'Bot Not Connected' : 'List Members'}
            </button>
          </form>

          {error && (
            <div className="mt-4 p-3 rounded-lg bg-red-100 text-red-800">
              {error}
            </div>
          )}

          {members.length > 0 && (
            <div className="mt-4">
              <h3 className="font-medium text-gray-800 mb-2">
                Members ({members.length})
              </h3>
              <div className="max-h-80 overflow-y-auto border rounded-lg">
                {members.map((userId) => (
                  <button
                    key={userId}
                    onClick={() => fetchUserInfo(userId)}
                    className={`w-full text-left px-3 py-2 hover:bg-gray-100 border-b last:border-b-0 ${
                      selectedUser?.id === userId ? 'bg-blue-50' : ''
                    }`}
                  >
                    <code className="text-sm">{userId}</code>
                  </button>
                ))}
              </div>
            </div>
          )}
        </div>

        <div className="bg-white rounded-lg shadow p-6">
          <h3 className="font-medium text-gray-800 mb-4">User Details</h3>

          {userLoading ? (
            <div className="text-center text-gray-500 py-8">Loading...</div>
          ) : selectedUser ? (
            <div className="space-y-4">
              {selectedUser.image && (
                <img
                  src={selectedUser.image}
                  alt={selectedUser.name}
                  className="w-16 h-16 rounded-full"
                />
              )}
              <div>
                <label className="text-xs text-gray-500">ID</label>
                <p className="font-mono text-sm">{selectedUser.id}</p>
              </div>
              <div>
                <label className="text-xs text-gray-500">Username</label>
                <p className="font-medium">{selectedUser.name}</p>
              </div>
              <div>
                <label className="text-xs text-gray-500">Full Name</label>
                <p>{selectedUser.real_name || '-'}</p>
              </div>
              <div>
                <label className="text-xs text-gray-500">Email</label>
                <p>{selectedUser.email || '-'}</p>
              </div>
              <div>
                <label className="text-xs text-gray-500">Title</label>
                <p>{selectedUser.title || '-'}</p>
              </div>
            </div>
          ) : (
            <div className="text-center text-gray-500 py-8">
              Select a user from the list to view details
            </div>
          )}
        </div>
      </div>
    </div>
  )
}

export default ChannelMembers
