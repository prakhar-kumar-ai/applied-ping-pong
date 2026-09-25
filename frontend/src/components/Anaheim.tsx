import { useState, useEffect } from 'react'
import { isBuildingResponse } from '../lib/api'

interface Employee {
  firstName: string
  lastName: string
  email: string
  teamName: string
  title: string
  profileImageUrl: string
  managerEmail: string
  office: string
  timezone: string
  githubName: string
  slackId: string
  linkedinUrl: string
  joinDate: string
  isActive: boolean
  memberType: string
}

type FetchResult<T> =
  | { status: 'ok'; data: T[] }
  | { status: 'not_configured' }
  | { status: 'building' }
  | { status: 'error' }

async function fetchUser(email: string): Promise<FetchResult<Employee>> {
  try {
    const res = await fetch(`/api/anaheim/user/${email}`)
    if (res.status === 404 || res.status === 400) return { status: 'not_configured' }
    if (await isBuildingResponse(res)) return { status: 'building' }
    if (!res.ok) return { status: 'error' }
    const json = await res.json()
    return { status: 'ok', data: [json.user] }
  } catch {
    return { status: 'error' }
  }
}

async function fetchSampleProfiles(): Promise<FetchResult<Employee>> {
  const [qy, pl] = await Promise.all([
    fetchUser('qy@applied.co'),
    fetchUser('pl@applied.co'),
  ])

  if (qy.status === 'not_configured' || pl.status === 'not_configured') {
    return { status: 'not_configured' }
  }
  if (qy.status === 'building' || pl.status === 'building') {
    return { status: 'building' }
  }
  if (qy.status === 'error' || pl.status === 'error') {
    return { status: 'error' }
  }

  return {
    status: 'ok',
    data: [...qy.data, ...pl.data],
  }
}

async function searchUsers(query: string): Promise<FetchResult<Employee>> {
  try {
    const res = await fetch('/api/anaheim/users', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ query }),
    })
    if (res.status === 404 || res.status === 400) return { status: 'not_configured' }
    if (await isBuildingResponse(res)) return { status: 'building' }
    if (!res.ok) return { status: 'error' }
    const json = await res.json()
    return { status: 'ok', data: json.users || [] }
  } catch {
    return { status: 'error' }
  }
}

function Anaheim() {
  const [profiles, setProfiles] = useState<FetchResult<Employee> | null>(null)
  const [searchQuery, setSearchQuery] = useState('')
  const [isSearching, setIsSearching] = useState(false)

  useEffect(() => {
    fetchSampleProfiles().then(setProfiles)
  }, [])

  const handleSearch = async () => {
    if (!searchQuery.trim()) return
    setIsSearching(true)
    const result = await searchUsers(searchQuery)
    setProfiles(result)
    setIsSearching(false)
  }

  const handleKeyPress = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter') {
      handleSearch()
    }
  }

  const getInitials = (firstName: string, lastName: string) => {
    return `${firstName[0] || ''}${lastName[0] || ''}`.toUpperCase()
  }

  const formatDate = (dateStr: string) => {
    if (!dateStr) return 'N/A'
    try {
      return new Date(dateStr).toLocaleDateString('en-US', {
        year: 'numeric',
        month: 'long',
        day: 'numeric',
      })
    } catch {
      return dateStr
    }
  }

  const loading = profiles === null

  if (loading) {
    return (
      <div>
        <div className="mb-6">
          <h2 className="text-2xl font-bold text-gray-800">Anaheim</h2>
          <p className="text-gray-600 mt-1">Employee directory</p>
        </div>
        <div className="p-8 text-center text-gray-500">Loading...</div>
      </div>
    )
  }

  if (profiles.status === 'building') {
    return (
      <div>
        <div className="mb-6">
          <h2 className="text-2xl font-bold text-gray-800">Anaheim</h2>
          <p className="text-gray-600 mt-1">Employee directory</p>
        </div>
        <div className="bg-white rounded-lg shadow p-8 text-center">
          <p className="text-gray-500">Backend is compiling, please wait...</p>
        </div>
      </div>
    )
  }

  if (profiles.status === 'error') {
    return (
      <div>
        <div className="mb-6">
          <h2 className="text-2xl font-bold text-gray-800">Anaheim</h2>
          <p className="text-gray-600 mt-1">Employee directory</p>
        </div>
        <div className="bg-white rounded-lg shadow p-8 text-center">
          <p className="text-red-600 font-medium">Something went wrong. Please try again.</p>
        </div>
      </div>
    )
  }

  if (profiles.status === 'not_configured') {
    return (
      <div>
        <div className="mb-6">
          <h2 className="text-2xl font-bold text-gray-800">Anaheim</h2>
          <p className="text-gray-600 mt-1">Employee directory</p>
        </div>
        <div className="bg-white rounded-lg shadow p-8 text-center">
          <p className="text-gray-600">
            Anaheim is not configured. Follow the setup instructions in README.md to configure your Anaheim credentials.
          </p>
        </div>
      </div>
    )
  }

  return (
    <div>
      <div className="mb-6">
        <h2 className="text-2xl font-bold text-gray-800">Anaheim</h2>
        <p className="text-gray-600 mt-1">Employee directory</p>
      </div>

      <div className="bg-green-50 border border-green-200 rounded-lg p-4 mb-6">
        <p className="text-green-800 font-medium">Anaheim is connected</p>
      </div>

      <div className="mb-6 flex gap-2">
        <input
          type="text"
          value={searchQuery}
          onChange={(e) => setSearchQuery(e.target.value)}
          onKeyPress={handleKeyPress}
          placeholder="Search by email or name..."
          className="flex-1 px-4 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
        />
        <button
          onClick={handleSearch}
          disabled={isSearching || !searchQuery.trim()}
          className="px-6 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:bg-gray-400 disabled:cursor-not-allowed"
        >
          {isSearching ? 'Searching...' : 'Search'}
        </button>
      </div>

      {profiles.data.length === 0 ? (
        <div className="bg-white rounded-lg shadow p-8 text-center">
          <p className="text-gray-600">No employees found matching your search</p>
        </div>
      ) : (
        <div className="grid gap-4 md:grid-cols-2">
          {profiles.data.map((employee) => (
            <div key={employee.email} className="bg-white rounded-lg shadow p-4">
              <div className="flex items-start gap-4">
                <div className="flex-shrink-0">
                  {employee.profileImageUrl ? (
                    <img
                      src={employee.profileImageUrl}
                      alt={`${employee.firstName} ${employee.lastName}`}
                      className="w-16 h-16 rounded-full object-cover"
                    />
                  ) : (
                    <div className="w-16 h-16 rounded-full bg-blue-500 flex items-center justify-center text-white font-semibold text-xl">
                      {getInitials(employee.firstName, employee.lastName)}
                    </div>
                  )}
                </div>
                <div className="flex-1 min-w-0">
                  <h4 className="font-bold text-gray-800 text-lg">
                    {employee.firstName} {employee.lastName}
                  </h4>
                  <p className="text-gray-600 text-sm">{employee.title || 'N/A'}</p>
                </div>
              </div>

              <div className="mt-4 space-y-2 text-sm text-gray-600">
                <div className="flex">
                  <span className="font-medium w-24">Email:</span>
                  <a href={`mailto:${employee.email}`} className="text-blue-600 hover:underline truncate">
                    {employee.email}
                  </a>
                </div>
                <div className="flex">
                  <span className="font-medium w-24">Team:</span>
                  <span className="truncate">{employee.teamName || 'N/A'}</span>
                </div>
                <div className="flex">
                  <span className="font-medium w-24">Manager:</span>
                  <span className="truncate">{employee.managerEmail || 'N/A'}</span>
                </div>
                <div className="flex">
                  <span className="font-medium w-24">Office:</span>
                  <span className="truncate">{employee.office || 'N/A'}</span>
                </div>
                <div className="flex">
                  <span className="font-medium w-24">Join Date:</span>
                  <span>{formatDate(employee.joinDate)}</span>
                </div>
                {employee.linkedinUrl && (
                  <div className="flex items-center">
                    <span className="font-medium w-24">LinkedIn:</span>
                    <a
                      href={employee.linkedinUrl}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="text-blue-600 hover:underline"
                    >
                      View Profile
                    </a>
                  </div>
                )}
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}

export default Anaheim
