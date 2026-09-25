import { useState, useEffect } from 'react'

interface FeedbackSubmission {
  id: string
  user_id: string
  category: string
  urgency: string
  description: string
  submitted_at: string
}

function FeedbackList() {
  const [submissions, setSubmissions] = useState<FeedbackSubmission[]>([])
  const [loading, setLoading] = useState(true)

  const fetchFeedback = async () => {
    try {
      const response = await fetch('/api/feedback')
      const data = await response.json()
      if (data.success) {
        setSubmissions(data.submissions || [])
      }
    } catch (err) {
      console.error('Failed to fetch feedback:', err)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchFeedback()
    const interval = setInterval(fetchFeedback, 10000)
    return () => clearInterval(interval)
  }, [])

  const getCategoryColor = (category: string) => {
    switch (category) {
      case 'bug':
        return 'bg-red-100 text-red-800'
      case 'feature':
        return 'bg-blue-100 text-blue-800'
      case 'improvement':
        return 'bg-green-100 text-green-800'
      default:
        return 'bg-gray-100 text-gray-800'
    }
  }

  const getUrgencyColor = (urgency: string) => {
    switch (urgency) {
      case 'high':
        return 'bg-red-100 text-red-800'
      case 'medium':
        return 'bg-yellow-100 text-yellow-800'
      case 'low':
        return 'bg-green-100 text-green-800'
      default:
        return 'bg-gray-100 text-gray-800'
    }
  }

  const formatTime = (timestamp: string) => {
    return new Date(timestamp).toLocaleString()
  }

  return (
    <div>
      <div className="mb-6">
        <h2 className="text-2xl font-bold text-gray-800">Feedback Submissions</h2>
        <p className="text-gray-600 mt-1">View all feedback submitted through the Slack form</p>
      </div>

      <div className="bg-white rounded-lg shadow mb-6 p-4">
        <h3 className="font-medium text-gray-800 mb-2">How to Submit Feedback</h3>
        <ul className="text-sm text-gray-600 space-y-1">
          <li>1. Use the <code className="bg-gray-100 px-1 rounded">/feedback</code> slash command in Slack</li>
          <li>2. Click a "Submit Feedback" button sent via the Send Message page</li>
          <li>3. Fill out the modal form with category, urgency, and description</li>
        </ul>
      </div>

      <div className="bg-white rounded-lg shadow">
        <div className="px-4 py-3 border-b flex justify-between items-center">
          <h3 className="font-medium text-gray-800">
            Submissions ({submissions.length})
          </h3>
          <button
            onClick={fetchFeedback}
            className="text-sm text-blue-600 hover:text-blue-800"
          >
            Refresh
          </button>
        </div>

        {loading ? (
          <div className="p-8 text-center text-gray-500">Loading...</div>
        ) : submissions.length === 0 ? (
          <div className="p-8 text-center text-gray-500">
            No feedback submissions yet. Use /feedback in Slack to submit one!
          </div>
        ) : (
          <div className="divide-y">
            {submissions.map((submission) => (
              <div key={submission.id} className="p-4">
                <div className="flex items-start gap-3">
                  <div className="flex-1">
                    <div className="flex items-center gap-2 mb-2">
                      <span className={`px-2 py-1 rounded text-xs font-medium ${getCategoryColor(submission.category)}`}>
                        {submission.category}
                      </span>
                      <span className={`px-2 py-1 rounded text-xs font-medium ${getUrgencyColor(submission.urgency)}`}>
                        {submission.urgency}
                      </span>
                      <span className="text-xs text-gray-500">
                        {formatTime(submission.submitted_at)}
                      </span>
                    </div>
                    <p className="text-gray-800">{submission.description}</p>
                    <p className="text-xs text-gray-500 mt-1">
                      From: {submission.user_id}
                    </p>
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

export default FeedbackList
