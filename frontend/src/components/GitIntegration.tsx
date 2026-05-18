import { useState, useEffect } from 'react'
import { GitBranch, GitCommit, GitPullRequest, Settings, Link2 } from 'lucide-react'

interface CommitInfo {
  id: string
  issue_id: string
  commit_sha: string
  message: string
  author: string
  committed_at?: string
}

interface BranchInfo {
  id: string
  issue_id: string
  branch_name: string
  repo_url: string
}

interface PRInfo {
  id: string
  issue_id: string
  pr_number: number
  title: string
  url: string
  state: string
  created_at: string
  merged_at?: string
}

interface ProviderConfig {
  id: string
  project_id: string
  provider_type: string
  base_url: string
  webhook_secret: string
  created_at: string
}

interface GitInfo {
  commits: CommitInfo[]
  branches: BranchInfo[]
  pull_requests: PRInfo[]
}

const API = 'http://localhost:8080/rest/api/3'

function getAuthHeaders(): Record<string, string> {
  const token = localStorage.getItem('token') || ''
  return {
    'Content-Type': 'application/json',
    Authorization: `Bearer ${token}`,
  }
}

function shortSHA(sha: string) {
  return sha.substring(0, 7)
}

function formatDate(ts: string) {
  return new Date(ts).toLocaleString()
}

const STATE_COLORS: Record<string, string> = {
  open: 'bg-green-600',
  merged: 'bg-purple-600',
  closed: 'bg-red-600',
}

interface GitIntegrationProps {
  projectKey: string
  issueKey: string
}

export default function GitIntegration({ projectKey, issueKey }: GitIntegrationProps) {
  const [gitInfo, setGitInfo] = useState<GitInfo | null>(null)
  const [provider, setProvider] = useState<ProviderConfig | null>(null)
  const [showConfig, setShowConfig] = useState(false)
  const [loading, setLoading] = useState(true)
  const [configForm, setConfigForm] = useState({
    provider_type: 'github',
    base_url: '',
    token: '',
    webhook_secret: '',
  })

  useEffect(() => {
    fetchGitInfo()
    fetchProvider()
  }, [issueKey])

  async function fetchGitInfo() {
    try {
      const res = await fetch(`${API}/issues/${issueKey}/git`, { headers: getAuthHeaders() })
      if (res.ok) setGitInfo(await res.json())
    } catch { /* ignore */ }
  }

  async function fetchProvider() {
    try {
      const res = await fetch(`${API}/projects/${projectKey}/git/providers`, { headers: getAuthHeaders() })
      if (res.ok) {
        const data = await res.json()
        setProvider(data)
      }
    } catch { /* ignore */ }
    setLoading(false)
  }

  async function handleConfigure(e: React.FormEvent) {
    e.preventDefault()
    const res = await fetch(`${API}/projects/${projectKey}/git/providers`, {
      method: 'POST',
      headers: getAuthHeaders(),
      body: JSON.stringify(configForm),
    })
    if (res.ok) {
      const data = await res.json()
      setProvider(data)
      setShowConfig(false)
    }
  }

  async function handleDeleteProvider() {
    await fetch(`${API}/projects/${projectKey}/git/providers`, {
      method: 'DELETE',
      headers: getAuthHeaders(),
    })
    setProvider(null)
  }

  if (loading) return null

  const hasContent = gitInfo && (
    gitInfo.commits.length > 0 ||
    gitInfo.branches.length > 0 ||
    gitInfo.pull_requests.length > 0
  )

  return (
    <div className="mt-6 border-t border-gray-700 pt-6">
      <div className="flex items-center justify-between mb-4">
        <h3 className="text-lg font-semibold text-white flex items-center gap-2">
          <GitBranch className="w-5 h-5" />
          Git Integration
        </h3>
        <button
          onClick={() => setShowConfig(!showConfig)}
          className="text-gray-400 hover:text-white"
          title="Configure Git Provider"
        >
          <Settings className="w-5 h-5" />
        </button>
      </div>

      {showConfig && (
        <div className="bg-gray-750 bg-gray-800 rounded-lg p-4 mb-4">
          {provider ? (
            <div>
              <div className="flex items-center justify-between mb-3">
                <div>
                  <span className="text-sm text-gray-400">Connected to </span>
                  <span className="text-sm font-medium text-white capitalize">{provider.provider_type}</span>
                  <span className="text-sm text-gray-400"> at </span>
                  <span className="text-sm text-blue-400">{provider.base_url}</span>
                </div>
                <button
                  onClick={handleDeleteProvider}
                  className="text-xs text-red-400 hover:text-red-300"
                >
                  Disconnect
                </button>
              </div>
              <div className="text-xs text-gray-500">
                Webhook URL: <code className="text-blue-400">{API}/webhooks/git/{provider.webhook_secret}</code>
              </div>
            </div>
          ) : (
            <form onSubmit={handleConfigure}>
              <div className="grid grid-cols-1 gap-3">
                <div>
                  <label className="block text-sm text-gray-400 mb-1">Provider</label>
                  <select
                    value={configForm.provider_type}
                    onChange={(e) => setConfigForm({ ...configForm, provider_type: e.target.value })}
                    className="w-full bg-gray-700 text-white rounded-lg px-3 py-2 border border-gray-600 focus:border-blue-500 focus:outline-none"
                  >
                    <option value="github">GitHub</option>
                    <option value="gitlab">GitLab</option>
                    <option value="forgejo">Forgejo</option>
                    <option value="gitea">Gitea</option>
                  </select>
                </div>
                <div>
                  <label className="block text-sm text-gray-400 mb-1">Base URL</label>
                  <input
                    type="text"
                    value={configForm.base_url}
                    onChange={(e) => setConfigForm({ ...configForm, base_url: e.target.value })}
                    placeholder="https://github.com/user/repo"
                    className="w-full bg-gray-700 text-white rounded-lg px-3 py-2 border border-gray-600 focus:border-blue-500 focus:outline-none"
                  />
                </div>
                <div>
                  <label className="block text-sm text-gray-400 mb-1">API Token</label>
                  <input
                    type="password"
                    value={configForm.token}
                    onChange={(e) => setConfigForm({ ...configForm, token: e.target.value })}
                    placeholder="ghp_xxx or personal access token"
                    className="w-full bg-gray-700 text-white rounded-lg px-3 py-2 border border-gray-600 focus:border-blue-500 focus:outline-none"
                  />
                </div>
                <div>
                  <label className="block text-sm text-gray-400 mb-1">Webhook Secret</label>
                  <input
                    type="text"
                    value={configForm.webhook_secret}
                    onChange={(e) => setConfigForm({ ...configForm, webhook_secret: e.target.value })}
                    placeholder="auto-generated if empty"
                    className="w-full bg-gray-700 text-white rounded-lg px-3 py-2 border border-gray-600 focus:border-blue-500 focus:outline-none"
                  />
                </div>
                <button
                  type="submit"
                  className="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700"
                >
                  Save Provider
                </button>
              </div>
            </form>
          )}
        </div>
      )}

      {!hasContent ? (
        <p className="text-sm text-gray-500">
          {provider
            ? 'No linked commits, branches, or pull requests yet.'
            : 'Configure a git provider to link commits and pull requests.'}
        </p>
      ) : (
        <div className="space-y-4">
          {gitInfo!.pull_requests.length > 0 && (
            <div>
              <h4 className="text-sm font-medium text-gray-400 mb-2 flex items-center gap-1">
                <GitPullRequest className="w-4 h-4" />
                Pull Requests
              </h4>
              <div className="space-y-2">
                {gitInfo!.pull_requests.map((pr) => (
                  <div key={pr.id} className="bg-gray-800 rounded-lg p-3">
                    <div className="flex items-center justify-between">
                      <a
                        href={pr.url}
                        target="_blank"
                        rel="noreferrer"
                        className="text-blue-400 hover:text-blue-300 text-sm font-medium"
                      >
                        #{pr.pr_number} {pr.title}
                      </a>
                      <span className={`px-2 py-0.5 rounded text-xs font-medium text-white ${STATE_COLORS[pr.state] || 'bg-gray-500'}`}>
                        {pr.state}
                      </span>
                    </div>
                    <div className="text-xs text-gray-500 mt-1">
                      Created {formatDate(pr.created_at)}
                      {pr.merged_at && ` · Merged ${formatDate(pr.merged_at)}`}
                    </div>
                  </div>
                ))}
              </div>
            </div>
          )}

          {gitInfo!.branches.length > 0 && (
            <div>
              <h4 className="text-sm font-medium text-gray-400 mb-2 flex items-center gap-1">
                <GitBranch className="w-4 h-4" />
                Branches
              </h4>
              <div className="space-y-1">
                {gitInfo!.branches.map((b) => (
                  <div key={b.id} className="bg-gray-800 rounded-lg p-2 flex items-center gap-2 text-sm">
                    <span className="text-blue-400 font-mono">{b.branch_name}</span>
                    {b.repo_url && (
                      <a
                        href={b.repo_url}
                        target="_blank"
                        rel="noreferrer"
                        className="text-gray-500 hover:text-gray-300"
                      >
                        <Link2 className="w-3 h-3" />
                      </a>
                    )}
                  </div>
                ))}
              </div>
            </div>
          )}

          {gitInfo!.commits.length > 0 && (
            <div>
              <h4 className="text-sm font-medium text-gray-400 mb-2 flex items-center gap-1">
                <GitCommit className="w-4 h-4" />
                Commits
              </h4>
              <div className="space-y-2">
                {gitInfo!.commits.map((c) => (
                  <div key={c.id} className="bg-gray-800 rounded-lg p-3">
                    <div className="flex items-center gap-2">
                      <code className="text-xs text-yellow-400 font-mono">{shortSHA(c.commit_sha)}</code>
                      <span className="text-sm text-gray-200">{c.message}</span>
                    </div>
                    <div className="text-xs text-gray-500 mt-1">
                      {c.author} · {c.committed_at ? formatDate(c.committed_at) : 'recently'}
                    </div>
                  </div>
                ))}
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  )
}
