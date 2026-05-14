import { useState, useEffect, useCallback } from 'react'
import { MessageSquare, Paperclip, Upload, History, Trash2, GitBranch } from 'lucide-react'
import GitIntegration from '../components/GitIntegration'

interface Issue {
  id: string
  key: string
  title: string
  description_json: string
  priority: string
  assignee_id: string | null
  status_id: string | null
  project_id: string
  created_at: string
  updated_at: string
}

interface Comment {
  id: string
  issue_id: string
  author_id: string | null
  body_json: string
  created_at: string
}

interface HistoryEntry {
  id: string
  issue_id: string
  actor_id: string | null
  actor_name: string
  field_name: string
  old_value: string
  new_value: string
  created_at: string
}

interface Attachment {
  id: string
  issue_id: string
  filename: string
  file_size: number
  uploader_id: string | null
  created_at: string
}

const API = 'http://localhost:8080/api/v1'

function getAuthHeaders(): Record<string, string> {
  const token = localStorage.getItem('token') || ''
  return {
    'Content-Type': 'application/json',
    Authorization: `Bearer ${token}`,
  }
}

function formatDate(ts: string) {
  return new Date(ts).toLocaleString()
}

function formatSize(bytes: number) {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1048576) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / 1048576).toFixed(1)} MB`
}

const PRIORITY_COLORS: Record<string, string> = {
  highest: 'bg-red-600',
  high: 'bg-orange-500',
  medium: 'bg-yellow-500',
  low: 'bg-green-500',
  lowest: 'bg-gray-400',
}

export default function IssueDetail({ issueKey, onBack }: { issueKey: string; onBack: () => void }) {
  const [issue, setIssue] = useState<Issue | null>(null)
  const [comments, setComments] = useState<Comment[]>([])
  const [history, setHistory] = useState<HistoryEntry[]>([])
  const [attachments, setAttachments] = useState<Attachment[]>([])
  const [newComment, setNewComment] = useState('')
  const [loading, setLoading] = useState(true)
  const [tab, setTab] = useState<'comments' | 'history' | 'attachments' | 'git'>('comments')
  const projectKey = issueKey.split('-')[0]

  const fetchIssue = useCallback(async () => {
    const res = await fetch(`${API}/issues/${issueKey}`, { headers: getAuthHeaders() })
    if (res.ok) setIssue(await res.json())
  }, [issueKey])

  const fetchComments = useCallback(async () => {
    const res = await fetch(`${API}/issues/${issueKey}/comments`, { headers: getAuthHeaders() })
    if (res.ok) setComments(await res.json())
  }, [issueKey])

  const fetchHistory = useCallback(async () => {
    const res = await fetch(`${API}/issues/${issueKey}/history`, { headers: getAuthHeaders() })
    if (res.ok) setHistory(await res.json())
  }, [issueKey])

  const fetchAttachments = useCallback(async () => {
    const res = await fetch(`${API}/issues/${issueKey}/attachments`, { headers: getAuthHeaders() })
    if (res.ok) setAttachments(await res.json())
  }, [issueKey])

  useEffect(() => {
    setLoading(true)
    Promise.all([fetchIssue(), fetchComments(), fetchHistory(), fetchAttachments()]).finally(() =>
      setLoading(false)
    )
  }, [fetchIssue, fetchComments, fetchHistory, fetchAttachments])

  const handleAddComment = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!newComment.trim()) return
    const body = JSON.stringify({ body_json: `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":${JSON.stringify(newComment)}}]}]}` })
    await fetch(`${API}/issues/${issueKey}/comments`, {
      method: 'POST',
      headers: getAuthHeaders(),
      body,
    })
    setNewComment('')
    fetchComments()
  }

  const handleDeleteComment = async (commentId: string) => {
    await fetch(`${API}/issues/${issueKey}/comments/${commentId}`, {
      method: 'DELETE',
      headers: getAuthHeaders(),
    })
    fetchComments()
  }

  const handleUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return
    const form = new FormData()
    form.append('file', file)
    const token = localStorage.getItem('token') || ''
    await fetch(`${API}/issues/${issueKey}/attachments`, {
      method: 'POST',
      headers: { Authorization: `Bearer ${token}` },
      body: form,
    })
    fetchAttachments()
  }

  const handleDeleteAttachment = async (id: string) => {
    await fetch(`${API}/issues/${issueKey}/attachments/${id}`, {
      method: 'DELETE',
      headers: getAuthHeaders(),
    })
    fetchAttachments()
  }

  if (loading) return <div className="p-6 text-gray-400">Loading...</div>
  if (!issue) return <div className="p-6 text-red-400">Issue not found</div>

  const desc = (() => {
    try {
      const parsed = JSON.parse(issue.description_json)
      return parsed.content || issue.title
    } catch {
      return issue.description_json || ''
    }
  })()

  return (
    <div className="max-w-4xl mx-auto p-6">
      <button onClick={onBack} className="text-blue-400 hover:text-blue-300 mb-4 inline-block">
        &larr; Back
      </button>
      <div className="bg-gray-800 rounded-lg p-6 mb-6">
        <div className="flex items-start justify-between mb-2">
          <div>
            <span className="text-sm text-gray-400">{issue.key}</span>
            <h1 className="text-2xl font-bold text-white">{issue.title}</h1>
          </div>
          <span className={`px-3 py-1 rounded text-xs font-semibold text-white ${PRIORITY_COLORS[issue.priority] || 'bg-gray-500'}`}>
            {issue.priority}
          </span>
        </div>
        <div className="text-gray-300 mt-4 whitespace-pre-wrap">{desc}</div>
        <div className="flex gap-6 mt-4 text-sm text-gray-400">
          <span>Status: <span className="text-white">{issue.status_id || 'None'}</span></span>
          <span>Assignee: <span className="text-white">{issue.assignee_id || 'Unassigned'}</span></span>
          <span>Created: {formatDate(issue.created_at)}</span>
        </div>
      </div>

      <div className="flex gap-2 mb-4 border-b border-gray-700">
        {(['comments', 'history', 'attachments', 'git'] as const).map((t) => (
          <button
            key={t}
            onClick={() => setTab(t)}
            className={`px-4 py-2 text-sm font-medium border-b-2 transition-colors ${
              tab === t ? 'border-blue-500 text-blue-400' : 'border-transparent text-gray-400 hover:text-gray-200'
            }`}
          >
            {t === 'comments' && <MessageSquare className="inline w-4 h-4 mr-1" />}
            {t === 'history' && <History className="inline w-4 h-4 mr-1" />}
            {t === 'attachments' && <Paperclip className="inline w-4 h-4 mr-1" />}
            {t === 'git' && <GitBranch className="inline w-4 h-4 mr-1" />}
            {t.charAt(0).toUpperCase() + t.slice(1)}
          </button>
        ))}
      </div>

      {tab === 'comments' && (
        <div>
          <form onSubmit={handleAddComment} className="mb-6">
            <textarea
              value={newComment}
              onChange={(e) => setNewComment(e.target.value)}
              placeholder="Add a comment..."
              className="w-full bg-gray-700 text-white rounded-lg p-3 border border-gray-600 focus:border-blue-500 focus:outline-none resize-none"
              rows={3}
            />
            <button
              type="submit"
              disabled={!newComment.trim()}
              className="mt-2 px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed"
            >
              Comment
            </button>
          </form>
          {comments.length === 0 ? (
            <p className="text-gray-500">No comments yet.</p>
          ) : (
            <div className="space-y-4">
              {comments.map((c) => (
                <div key={c.id} className="bg-gray-750 bg-gray-800 rounded-lg p-4">
                  <div className="flex items-center justify-between mb-2">
                    <span className="text-sm text-gray-400">
                      {c.author_id || 'Unknown'} &middot; {formatDate(c.created_at)}
                    </span>
                    <button
                      onClick={() => handleDeleteComment(c.id)}
                      className="text-gray-500 hover:text-red-400"
                      title="Delete"
                    >
                      <Trash2 className="w-4 h-4" />
                    </button>
                  </div>
                  <div className="text-gray-200 whitespace-pre-wrap">
                    {(() => {
                      try {
                        const parsed = JSON.parse(c.body_json)
                        const text = parsed.content?.[0]?.content?.[0]?.text
                        return text || c.body_json
                      } catch {
                        return c.body_json
                      }
                    })()}
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      )}

      {tab === 'history' && (
        <div>
          {history.length === 0 ? (
            <p className="text-gray-500">No history yet.</p>
          ) : (
            <div className="space-y-3">
              {history.map((h) => (
                <div key={h.id} className="flex gap-3 text-sm">
                  <div className="w-2 h-2 mt-2 rounded-full bg-blue-500 shrink-0" />
                  <div>
                    <span className="text-gray-400">
                      <span className="text-white">{h.actor_name || h.actor_id || 'System'}</span>
                      {' '}changed <span className="text-white">{h.field_name}</span>
                    </span>
                    {h.old_value && (
                      <span className="text-gray-500"> from "{h.old_value}"</span>
                    )}
                    {h.new_value && (
                      <span className="text-gray-500"> to "{h.new_value}"</span>
                    )}
                    <br />
                    <span className="text-xs text-gray-600">{formatDate(h.created_at)}</span>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      )}

      {tab === 'attachments' && (
        <div>
          <label className="inline-flex items-center gap-2 px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 cursor-pointer mb-4">
            <Upload className="w-4 h-4" />
            Upload File
            <input type="file" className="hidden" onChange={handleUpload} />
          </label>
          {attachments.length === 0 ? (
            <p className="text-gray-500">No attachments yet.</p>
          ) : (
            <div className="space-y-2">
              {attachments.map((a) => (
                <div key={a.id} className="bg-gray-800 rounded-lg p-3 flex items-center justify-between">
                  <div className="flex items-center gap-3">
                    <Paperclip className="w-4 h-4 text-gray-400" />
                    <div>
                      <a
                        href={`${API}/attachments/${a.id}`}
                        target="_blank"
                        rel="noreferrer"
                        className="text-blue-400 hover:text-blue-300"
                      >
                        {a.filename}
                      </a>
                      <span className="text-xs text-gray-500 ml-2">
                        {formatSize(a.file_size)} &middot; {formatDate(a.created_at)}
                      </span>
                    </div>
                  </div>
                  <button
                    onClick={() => handleDeleteAttachment(a.id)}
                    className="text-gray-500 hover:text-red-400"
                    title="Delete"
                  >
                    <Trash2 className="w-4 h-4" />
                  </button>
                </div>
              ))}
            </div>
          )}
        </div>
      )}

      {tab === 'git' && (
        <GitIntegration projectKey={projectKey} issueKey={issueKey} />
      )}
    </div>
  )
}
