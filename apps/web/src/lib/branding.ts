export const APP_NAME = 'RapidDFM'
export const COMPANY_NAME = 'Saturn Solutions'
export const APP_TITLE = `${APP_NAME} by ${COMPANY_NAME}`
export const ADMIN_APP_NAME = `${APP_NAME} Admin`
export const APP_DESCRIPTION = `PCB Design-for-Manufacturability Analysis by ${COMPANY_NAME}` // release

export const THEME_STORAGE_KEY = 'rapiddfm-theme'
export const LEGACY_THEME_STORAGE_KEY = 'betterdfm-theme'

export const UI_SETTINGS_STORAGE_KEY = 'rapiddfm-ui-settings'

export const TOKEN_STORAGE_KEY = 'rapiddfm_token'
export const LEGACY_TOKEN_STORAGE_KEY = 'betterdfm_token'

export const ADMIN_TOKEN_STORAGE_KEY = 'rapiddfm_admin_token'
export const LEGACY_ADMIN_TOKEN_STORAGE_KEY = 'betterdfm_admin_token'

// localStorage can throw (private/incognito mode, storage disabled, quota
// exceeded). Never crash on storage failures — degrade to in-memory behavior.

export function getStoredValue(primaryKey: string, legacyKey?: string): string | null {
  if (typeof window === 'undefined') return null

  try {
    const primaryValue = localStorage.getItem(primaryKey)
    if (primaryValue !== null) return primaryValue

    if (!legacyKey) return null

    const legacyValue = localStorage.getItem(legacyKey)
    if (legacyValue !== null) {
      localStorage.setItem(primaryKey, legacyValue)
      localStorage.removeItem(legacyKey)
    }

    return legacyValue
  } catch {
    // localStorage unavailable (private mode) — treat as unset.
    return null
  }
}

export function setStoredValue(primaryKey: string, value: string, legacyKey?: string): void {
  try {
    localStorage.setItem(primaryKey, value)
    if (legacyKey) {
      localStorage.removeItem(legacyKey)
    }
  } catch {
    // localStorage unavailable (private mode) — keep the in-memory value.
  }
}

export function clearStoredValue(primaryKey: string, legacyKey?: string): void {
  try {
    localStorage.removeItem(primaryKey)
    if (legacyKey) {
      localStorage.removeItem(legacyKey)
    }
  } catch {
    // localStorage unavailable (private mode) — nothing to clear.
  }
}
