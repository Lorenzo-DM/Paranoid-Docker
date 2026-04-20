export interface ReleaseNote {
  version: string
  url: string
  body: string
  isBreaking: boolean
  publishedAt: string
}

export interface ImageCheckResult {
  service: string
  image: string
  currentVersion: string
  resolvedViaLabel: boolean
  gitRevision?: string
  latestTag: string | null
  hasUpdate: boolean
  githubRepo: string | null
  releases: ReleaseNote[]
  hasBreakingChanges: boolean
  error?: string
}
