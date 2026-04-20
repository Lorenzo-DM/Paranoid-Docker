import type { ComposeService } from '../types/api'
import type { ImageCheckResult, ReleaseNote } from '../types/versionCheck'

// ---------------------------------------------------------------------------
// Static fallback: known Docker Hub official image names → GitHub owner/repo
// ---------------------------------------------------------------------------
const KNOWN_GITHUB_REPOS: Record<string, string> = {
  nginx: 'nginx/nginx',
  traefik: 'traefik/traefik',
  redis: 'redis/redis',
  grafana: 'grafana/grafana',
  prometheus: 'prometheus/prometheus',
  alertmanager: 'prometheus/alertmanager',
  'node-exporter': 'prometheus/node_exporter',
  cadvisor: 'google/cadvisor',
  portainer: 'portainer/portainer-ce',
  gitea: 'go-gitea/gitea',
  nextcloud: 'nextcloud/nextcloud',
  vaultwarden: 'dani-garcia/vaultwarden',
  filebrowser: 'filebrowser/filebrowser',
  'uptime-kuma': 'louislam/uptime-kuma',
  watchtower: 'containrrr/watchtower',
  dozzle: 'amir20/dozzle',
  authelia: 'authelia/authelia',
  minio: 'minio/minio',
  keycloak: 'keycloak/keycloak',
  lldap: 'lldap/lldap',
  'immich-server': 'immich-app/immich',
  jellyfin: 'jellyfin/jellyfin',
  audiobookshelf: 'advplyr/audiobookshelf',
  miniflux: 'miniflux/v2',
  'paperless-ngx': 'paperless-ngx/paperless-ngx',
  'stirling-pdf': 'Stirling-Tools/Stirling-PDF',
  homepage: 'gethomepage/homepage',
  syncthing: 'syncthing/syncthing',
  caddy: 'caddyserver/caddy',
  loki: 'grafana/loki',
  tempo: 'grafana/tempo',
  influxdb: 'influxdata/influxdb',
  mosquitto: 'eclipse/mosquitto',
  outline: 'outline/outline',
  actual: 'actualbudget/actual',
}

// ---------------------------------------------------------------------------
// Tag-version regex: matches "1.27.3", "v1.27.3", "1.27.3-alpine", etc.
// ---------------------------------------------------------------------------
const TAG_VERSION_RE = /^v?([\d]+(?:\.[\d]+)*)(-[a-z][a-z0-9.-]*)?$/i

// Tags that carry no version semantics
const NON_VERSION_TAGS = new Set(['latest', 'stable', 'edge', 'main', 'master', 'dev', 'nightly', 'beta', 'alpha'])

// ---------------------------------------------------------------------------
// Parsed image reference
// ---------------------------------------------------------------------------
interface ParsedImage {
  registry: string   // '' for Docker Hub
  namespace: string  // 'library' for official images
  name: string
  tag: string
  suffix: string     // e.g. '-alpine', '' for plain semver
}

export function parseImageRef(imageStr: string): ParsedImage {
  let remaining = imageStr

  // Strip digest (@sha256:...)
  const atIdx = remaining.indexOf('@')
  if (atIdx >= 0) remaining = remaining.slice(0, atIdx)

  // Registry prefix: contains a dot/colon in the part before the first slash
  let registry = ''
  const firstSlash = remaining.indexOf('/')
  if (firstSlash > 0) {
    const prefix = remaining.slice(0, firstSlash)
    if (prefix.includes('.') || prefix.includes(':') || prefix === 'localhost') {
      registry = prefix
      remaining = remaining.slice(firstSlash + 1)
    }
  }

  // Tag
  const colonIdx = remaining.lastIndexOf(':')
  let tag = 'latest'
  if (colonIdx > 0) {
    tag = remaining.slice(colonIdx + 1)
    remaining = remaining.slice(0, colonIdx)
  }

  // Namespace / name
  const slashIdx = remaining.indexOf('/')
  let namespace = 'library'
  let name = remaining
  if (slashIdx >= 0) {
    namespace = remaining.slice(0, slashIdx)
    name = remaining.slice(slashIdx + 1)
  }

  // Suffix from tag (e.g. "-alpine")
  const tagMatch = tag.match(TAG_VERSION_RE)
  const suffix = tagMatch ? (tagMatch[2] ?? '') : ''

  return { registry, namespace, name, tag, suffix }
}

// ---------------------------------------------------------------------------
// Resolve the "current version" of a service
// Priority: semver from tag > OCI label org.opencontainers.image.version
// ---------------------------------------------------------------------------
export function resolveCurrentVersion(svc: ComposeService): {
  version: string | null
  viaLabel: boolean
  revision?: string
} {
  const labels = svc.image_labels ?? {}
  const parsed = parseImageRef(svc.image)
  const revision = labels['org.opencontainers.image.revision']?.slice(0, 7)

  // Tag carries a real version
  if (!NON_VERSION_TAGS.has(parsed.tag.toLowerCase())) {
    const m = parsed.tag.match(TAG_VERSION_RE)
    if (m) return { version: m[1], viaLabel: false, revision }
  }

  // Fall back to OCI label
  const labelVersion = labels['org.opencontainers.image.version']
  if (labelVersion) {
    const m = labelVersion.match(/^v?([\d]+(?:\.[\d]+)*)/)
    if (m) return { version: m[1], viaLabel: true, revision }
  }

  return { version: null, viaLabel: false, revision }
}

// ---------------------------------------------------------------------------
// Resolve GitHub repo from OCI label or static mapping
// ---------------------------------------------------------------------------
export function resolveGitHubRepo(svc: ComposeService): string | null {
  const labels = svc.image_labels ?? {}

  // 1. OCI label source / url
  for (const key of ['org.opencontainers.image.source', 'org.opencontainers.image.url']) {
    const src = labels[key] ?? ''
    const m = src.match(/github\.com\/([^/\s]+\/[^/\s.]+)/)
    if (m) return m[1].replace(/\.git$/, '')
  }

  // 2. Static mapping
  const parsed = parseImageRef(svc.image)
  const key = parsed.name.toLowerCase()
  if (KNOWN_GITHUB_REPOS[key]) return KNOWN_GITHUB_REPOS[key]

  // 3. Non-library DockerHub image: try namespace/name as GitHub repo
  if (parsed.namespace !== 'library' && parsed.registry === '') {
    return `${parsed.namespace}/${parsed.name}`
  }

  return null
}

// ---------------------------------------------------------------------------
// Semver utilities
// ---------------------------------------------------------------------------
function parseSemver(v: string): number[] | null {
  const m = v.match(/^v?([\d]+(?:\.[\d]+)*)/)
  if (!m) return null
  return m[1].split('.').map(Number)
}

function compareSemver(a: number[], b: number[]): number {
  const len = Math.max(a.length, b.length)
  for (let i = 0; i < len; i++) {
    const diff = (a[i] ?? 0) - (b[i] ?? 0)
    if (diff !== 0) return diff
  }
  return 0
}

// ---------------------------------------------------------------------------
// Docker Hub: fetch tags
// ---------------------------------------------------------------------------
async function fetchDockerHubTags(namespace: string, name: string): Promise<string[]> {
  try {
    const url = `https://hub.docker.com/v2/repositories/${namespace}/${name}/tags?page_size=100&ordering=last_updated`
    const res = await fetch(url)
    if (!res.ok) return []
    const data = await res.json()
    return (data.results ?? []).map((t: { name: string }) => t.name)
  } catch {
    return []
  }
}

// Finds the highest semver tag that is newer than currentVersion and shares the same suffix
async function findLatestTag(parsed: ParsedImage, currentVersion: string): Promise<string | null> {
  if (parsed.registry !== '') return null  // Not a Docker Hub image — skip

  const tags = await fetchDockerHubTags(parsed.namespace, parsed.name)
  if (tags.length === 0) return null

  const currentSv = parseSemver(currentVersion)
  if (!currentSv) return null

  const candidates: Array<{ tag: string; sv: number[] }> = []
  for (const tag of tags) {
    const m = tag.match(TAG_VERSION_RE)
    if (!m) continue
    const tagSuffix = m[2] ?? ''
    if (tagSuffix !== parsed.suffix) continue  // Must share same variant (e.g. -alpine)
    const sv = parseSemver(tag)
    if (!sv) continue
    if (compareSemver(sv, currentSv) > 0) {
      candidates.push({ tag, sv })
    }
  }

  if (candidates.length === 0) return null
  candidates.sort((a, b) => compareSemver(b.sv, a.sv))
  return candidates[0].tag
}

// ---------------------------------------------------------------------------
// Breaking change detection
// ---------------------------------------------------------------------------
const BREAKING_KEYWORDS = [
  'breaking change', 'breaking:', '⚠️', '🚨',
  'migration required', 'requires migration',
  'incompatible', 'upgrade guide', 'action required',
  'manual step', 'upgrade path', 'removed in',
  'deprecated and removed', 'you must', 'before upgrading',
]

function detectBreakingChanges(body: string): boolean {
  const lower = body.toLowerCase()
  return BREAKING_KEYWORDS.some(kw => lower.includes(kw.toLowerCase()))
}

// ---------------------------------------------------------------------------
// GitHub Releases API
// ---------------------------------------------------------------------------
async function fetchGitHubReleases(
  githubRepo: string,
  currentVersion: string,
  latestTag: string | null,
): Promise<ReleaseNote[]> {
  try {
    const res = await fetch(
      `https://api.github.com/repos/${githubRepo}/releases?per_page=50`,
      { headers: { Accept: 'application/vnd.github+json' } },
    )
    if (!res.ok) return []

    const releases = await res.json() as Array<{
      tag_name: string
      html_url: string
      body: string | null
      published_at: string
    }>

    const currentSv = parseSemver(currentVersion)
    if (!currentSv) return []
    const latestSv = latestTag ? parseSemver(latestTag) : null

    const result: ReleaseNote[] = []
    for (const r of releases) {
      const sv = parseSemver(r.tag_name)
      if (!sv) continue
      if (compareSemver(sv, currentSv) <= 0) continue  // Not newer than current
      if (latestSv && compareSemver(sv, latestSv) > 0) continue  // Beyond latest
      const body = r.body ?? ''
      result.push({
        version: r.tag_name,
        url: r.html_url,
        body,
        isBreaking: detectBreakingChanges(body),
        publishedAt: r.published_at,
      })
    }
    // Newest first
    result.sort((a, b) => {
      const as = parseSemver(a.version)
      const bs = parseSemver(b.version)
      if (!as || !bs) return 0
      return compareSemver(bs, as)
    })
    return result
  } catch {
    return []
  }
}

// ---------------------------------------------------------------------------
// Check a single service
// ---------------------------------------------------------------------------
async function checkService(svc: ComposeService): Promise<ImageCheckResult> {
  const parsed = parseImageRef(svc.image)
  const { version: currentVersion, viaLabel, revision } = resolveCurrentVersion(svc)
  const githubRepo = resolveGitHubRepo(svc)

  if (!currentVersion) {
    return {
      service: svc.name,
      image: svc.image,
      currentVersion: parsed.tag,
      resolvedViaLabel: false,
      gitRevision: revision,
      latestTag: null,
      hasUpdate: svc.update_available,
      githubRepo,
      releases: [],
      hasBreakingChanges: false,
      error: parsed.tag === 'latest' || NON_VERSION_TAGS.has(parsed.tag)
        ? 'Image uses a non-versioned tag and has no OCI version label'
        : 'Cannot determine current version',
    }
  }

  const latestTag = await findLatestTag(parsed, currentVersion)
  const releases = githubRepo
    ? await fetchGitHubReleases(githubRepo, currentVersion, latestTag)
    : []

  return {
    service: svc.name,
    image: svc.image,
    currentVersion,
    resolvedViaLabel: viaLabel,
    gitRevision: revision,
    latestTag,
    hasUpdate: !!latestTag || svc.update_available,
    githubRepo,
    releases,
    hasBreakingChanges: releases.some(r => r.isBreaking),
  }
}

// ---------------------------------------------------------------------------
// Public API: check all services of a stack
// ---------------------------------------------------------------------------
export async function checkStackUpdates(services: ComposeService[]): Promise<ImageCheckResult[]> {
  return Promise.all(services.map(checkService))
}
