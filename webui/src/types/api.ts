export interface Port {
  host_port: string
  container_port: string
  protocol: string
}

export interface ComposeService {
  name: string
  image: string
  container_id: string
  state: string
  status: string
  ports: Port[]
  update_available: boolean
  local_digest?: string
  remote_digest?: string
  image_labels?: Record<string, string>
}

export interface ComposeStack {
  name: string
  status: string
  config_files: string[]
  working_dir: string
  services: ComposeService[]
  update_available: boolean
  rollback_mode: 'compose' | 'inspect'
}

export interface Capabilities {
  compose_files_available: boolean
  rollback_mode: 'auto' | 'compose' | 'inspect'
}

export interface StackEvent {
  type: 'progress' | 'done' | 'error'
  step?: string
  service?: string
  line?: string
  error?: string
  layer_id?: string
  current?: number
  total?: number
}

export interface Container {
  id: string
  short_id: string
  name: string
  image: string
  image_id: string
  status: string
  state: string
  created_at: string
  ports: Port[]
  update_available: boolean
  remote_digest?: string
  local_digest?: string
}

export interface PullEvent {
  type: 'progress' | 'done' | 'error'
  status?: string
  progress?: string
  id?: string
  error?: string
  message?: string
  current?: number
  total?: number
}

export interface LogEvent {
  line: string
}

export interface RollbackFile {
  filename: string
  path: string
  created_at: string
  previous_image: string
}

export interface SaveProgress {
  type: 'progress' | 'done' | 'error'
  written_bytes?: number
  message?: string
  filename?: string
  size_bytes?: number
  error?: string
}

export interface SavedImage {
  filename: string
  path: string
  image_ref: string
  size_bytes: number
  saved_at: string
}

// target (stack name or container name) → ISO datetime of last update start
export type UpdateLog = Record<string, string>
