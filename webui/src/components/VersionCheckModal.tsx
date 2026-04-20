import { useState } from 'react'
import { IconExternalLink, IconAlertTriangle, IconChevronDown, IconChevronRight, IconRefresh, IconCloudSearch } from '@tabler/icons-react'
import type { ImageCheckResult } from '../types/versionCheck'

interface Props {
  stackName: string
  loading: boolean
  error: string | null
  results: ImageCheckResult[]
  onClose: () => void
  onUpdate: () => void
  onRecheck: () => void
}

function stripMarkdown(text: string): string {
  return text
    .replace(/#{1,6}\s+/g, '')
    .replace(/\*\*([^*]+)\*\*/g, '$1')
    .replace(/\*([^*]+)\*/g, '$1')
    .replace(/`([^`]+)`/g, '$1')
    .replace(/\[([^\]]+)\]\([^)]+\)/g, '$1')
    .replace(/^\s*[-*+]\s/gm, '• ')
    .replace(/\n{3,}/g, '\n\n')
    .trim()
}

function formatDate(iso: string): string {
  try {
    return new Date(iso).toLocaleDateString('en-GB', { year: 'numeric', month: 'short', day: 'numeric' })
  } catch {
    return iso
  }
}

function ReleaseItem({ release }: { release: ImageCheckResult['releases'][number] }) {
  const [open, setOpen] = useState(release.isBreaking)
  const preview = stripMarkdown(release.body)
  const truncated = preview.length > 600 ? preview.slice(0, 600) + '…' : preview

  return (
    <div className={`release-item${release.isBreaking ? ' release-item-breaking' : ''}`}>
      <button className="release-toggle" onClick={() => setOpen(v => !v)}>
        <span className="release-toggle-icon">
          {open ? <IconChevronDown size={12} /> : <IconChevronRight size={12} />}
        </span>
        <span className="release-version">{release.version}</span>
        {release.isBreaking && (
          <span className="release-breaking-pill">
            <IconAlertTriangle size={11} /> Breaking
          </span>
        )}
        <span className="release-date muted">{formatDate(release.publishedAt)}</span>
        <a
          className="release-link"
          href={release.url}
          target="_blank"
          rel="noopener noreferrer"
          onClick={e => e.stopPropagation()}
          title="Open on GitHub"
        >
          <IconExternalLink size={11} />
        </a>
      </button>

      {open && release.body && (
        <div className="release-body">
          <pre className="release-body-text">{truncated}</pre>
        </div>
      )}
    </div>
  )
}

function ServiceResult({ result }: { result: ImageCheckResult }) {
  const status = result.error
    ? 'unknown'
    : result.hasBreakingChanges
      ? 'breaking'
      : result.hasUpdate
        ? 'update'
        : 'ok'

  return (
    <div className="service-check-row">
      <div className="service-check-header">
        <div className="service-check-meta">
          <span className="service-check-name">{result.service}</span>
          <span className="mono muted tiny">{result.image}</span>
        </div>

        <div className="version-flow">
          <span className="version-chip">
            {result.currentVersion}
            {result.resolvedViaLabel && <span className="via-label-hint">via label</span>}
            {result.gitRevision && !result.resolvedViaLabel && (
              <span className="via-label-hint">{result.gitRevision}</span>
            )}
          </span>

          {result.latestTag && (
            <>
              <span className="version-arrow">→</span>
              <span className="version-chip version-chip-new">{result.latestTag}</span>
            </>
          )}
        </div>

        <div className={`status-badge status-badge-${status}`}>
          {status === 'ok' && 'Up to date'}
          {status === 'update' && 'Update available'}
          {status === 'breaking' && '⚠ Breaking changes'}
          {status === 'unknown' && 'Unknown'}
        </div>
      </div>

      {result.error && (
        <p className="service-check-error muted tiny">{result.error}</p>
      )}

      {result.githubRepo && !result.error && (
        <p className="service-check-repo muted tiny">
          GitHub: <a href={`https://github.com/${result.githubRepo}`} target="_blank" rel="noopener noreferrer">{result.githubRepo}</a>
        </p>
      )}

      {result.releases.length > 0 && (
        <div className="release-list">
          {result.releases.map(r => (
            <ReleaseItem key={r.version} release={r} />
          ))}
        </div>
      )}

      {result.hasUpdate && result.releases.length === 0 && !result.error && (
        <p className="service-check-noreleases muted tiny">
          {result.githubRepo
            ? 'Update available — no release notes found on GitHub.'
            : 'Update available — no GitHub repository linked.'}
        </p>
      )}
    </div>
  )
}

export function VersionCheckModal({ stackName, loading, error, results, onClose, onUpdate, onRecheck }: Props) {
  const anyBreaking = results.some(r => r.hasBreakingChanges)
  const anyUpdate = results.some(r => r.hasUpdate)
  const allOk = results.length > 0 && !anyBreaking && !anyUpdate

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div
        className="modal modal-wide"
        onClick={e => e.stopPropagation()}
        role="dialog"
        aria-label={`Version check for ${stackName}`}
      >
        {/* Header */}
        <div className="modal-header">
          <div className="row" style={{ gap: 10 }}>
            <IconCloudSearch size={18} style={{ opacity: 0.7 }} />
            <div>
              <div style={{ fontWeight: 600, fontSize: 15 }}>Version Check</div>
              <div className="muted tiny">{stackName}</div>
            </div>
          </div>
        </div>

        {/* Body */}
        <div className="modal-body version-check-body">
          {loading && (
            <div className="version-check-loading">
              <span className="check-spinner" />
              <span className="muted">Checking images and release notes…</span>
            </div>
          )}

          {!loading && error && (
            <div className="version-check-error-banner">
              <IconAlertTriangle size={15} />
              <span>{error}</span>
            </div>
          )}

          {!loading && !error && anyBreaking && (
            <div className="breaking-banner">
              <IconAlertTriangle size={15} />
              <strong>Breaking changes detected</strong>
              <span>— review release notes carefully before updating.</span>
            </div>
          )}

          {!loading && !error && allOk && (
            <div className="uptodate-banner">
              All images are up to date.
            </div>
          )}

          {!loading && results.map(r => (
            <ServiceResult key={r.service} result={r} />
          ))}
        </div>

        {/* Footer */}
        <div className="modal-footer">
          <button className="btn small ghost" onClick={onRecheck} disabled={loading}>
            <IconRefresh size={13} /> Re-check
          </button>
          <div style={{ flex: 1 }} />
          <button className="btn small ghost" onClick={onClose}>Close</button>
          {anyUpdate && (
            <button className={`btn small${anyBreaking ? ' danger-btn' : ' primary'}`} onClick={onUpdate}>
              <IconRefresh size={13} />
              {anyBreaking ? '⚠ Update anyway' : 'Update now'}
            </button>
          )}
        </div>
      </div>
    </div>
  )
}
