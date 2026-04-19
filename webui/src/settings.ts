const KEY_ROLLBACK_INCLUDE_ENV = 'rollback_include_env'

export function getRollbackIncludeEnv(): boolean {
  return localStorage.getItem(KEY_ROLLBACK_INCLUDE_ENV) === 'true'
}

export function setRollbackIncludeEnv(value: boolean): void {
  localStorage.setItem(KEY_ROLLBACK_INCLUDE_ENV, String(value))
}
