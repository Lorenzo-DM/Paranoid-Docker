import type { ComposeStack, Container, RollbackFile, SavedImage } from '../types/api'

const BASE = import.meta.env.VITE_API_BASE_URL || '/api/v1'

export async function fetchStacks(): Promise<ComposeStack[]> {
  const res = await fetch(`${BASE}/stacks`)
  if (!res.ok) throw new Error(`Failed to fetch stacks: ${res.statusText}`)
  return res.json()
}

export async function triggerStackUpdate(name: string, includeEnv = false): Promise<{ job_id: string }> {
  const res = await fetch(`${BASE}/stacks/${name}/update`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ include_env: includeEnv }),
  })
  if (!res.ok) throw new Error(`Failed to trigger update: ${res.statusText}`)
  return res.json()
}

export function createStackUpdateEventSource(name: string): EventSource {
  return new EventSource(`${BASE}/stacks/${name}/update-status`)
}

export function createStackLogsEventSource(name: string): EventSource {
  return new EventSource(`${BASE}/stacks/${name}/logs`)
}

export async function triggerStackSnapshot(name: string, includeEnv = false): Promise<{ dir: string }> {
  const res = await fetch(`${BASE}/stacks/${name}/snapshot`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ include_env: includeEnv }),
  })
  if (!res.ok) throw new Error(`Snapshot failed: ${res.statusText}`)
  return res.json()
}

export async function triggerSaveStackImages(name: string): Promise<{ job_id: string }> {
  const res = await fetch(`${BASE}/stacks/${name}/save-images`, { method: 'POST' })
  if (!res.ok) throw new Error(`Failed to trigger save: ${res.statusText}`)
  return res.json()
}

export function createStackSaveEventSource(name: string): EventSource {
  return new EventSource(`${BASE}/stacks/${name}/save-status`)
}

export async function triggerSaveAndUpdateStack(name: string, includeEnv = false): Promise<{ job_id: string }> {
  const res = await fetch(`${BASE}/stacks/${name}/save-and-update`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ include_env: includeEnv }),
  })
  if (!res.ok) throw new Error(`Failed to trigger save+update: ${res.statusText}`)
  return res.json()
}

export function createStackSaveUpdateEventSource(name: string): EventSource {
  return new EventSource(`${BASE}/stacks/${name}/save-update-status`)
}

export async function fetchStackRollbacks(name: string): Promise<RollbackFile[]> {
  const res = await fetch(`${BASE}/stacks/${name}/rollbacks`)
  if (!res.ok) throw new Error(`Failed to fetch rollbacks: ${res.statusText}`)
  return res.json()
}

export function stackRollbackDownloadUrl(stackName: string, filename: string): string {
  return `${BASE}/stacks/${stackName}/rollbacks/${filename}`
}

export async function fetchContainers(): Promise<Container[]> {
  const res = await fetch(`${BASE}/containers`)
  if (!res.ok) throw new Error(`Failed to fetch containers: ${res.statusText}`)
  return res.json()
}

export async function triggerUpdate(id: string, includeEnv = false): Promise<{ job_id: string }> {
  const res = await fetch(`${BASE}/containers/${id}/update`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ include_env: includeEnv }),
  })
  if (!res.ok) throw new Error(`Failed to trigger update: ${res.statusText}`)
  return res.json()
}

export function createPullStatusEventSource(id: string): EventSource {
  return new EventSource(`${BASE}/containers/${id}/pull-status`)
}

export function createLogsEventSource(id: string): EventSource {
  return new EventSource(`${BASE}/containers/${id}/logs`)
}

export async function fetchRollbacks(id: string): Promise<RollbackFile[]> {
  const res = await fetch(`${BASE}/containers/${id}/rollbacks`)
  if (!res.ok) throw new Error(`Failed to fetch rollbacks: ${res.statusText}`)
  return res.json()
}

export function rollbackDownloadUrl(id: string, filename: string): string {
  return `${BASE}/containers/${id}/rollbacks/${filename}`
}

export async function triggerSaveImage(id: string): Promise<{ job_id: string }> {
  const res = await fetch(`${BASE}/containers/${id}/save-image`, { method: 'POST' })
  if (!res.ok) throw new Error(`Failed to trigger save: ${res.statusText}`)
  return res.json()
}

export function createSaveStatusEventSource(id: string): EventSource {
  return new EventSource(`${BASE}/containers/${id}/save-status`)
}

export async function fetchSavedImages(): Promise<SavedImage[]> {
  const res = await fetch(`${BASE}/images`)
  if (!res.ok) throw new Error(`Failed to fetch saved images: ${res.statusText}`)
  return res.json()
}

export function savedImageDownloadUrl(filename: string): string {
  return `${BASE}/images/${filename}`
}
