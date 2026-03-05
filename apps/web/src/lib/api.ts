import { getStoredToken, clearToken } from './auth'

export const API_URL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080'

// ── Types ────────────────────────────────────────────────────────────────────

export interface Submission {
  id: string
  filename: string
  fileType: 'GERBER' | 'ODB_PLUS_PLUS'
  status: 'UPLOADED' | 'ANALYZING' | 'DONE' | 'FAILED'
  createdAt: string
  orgId: string
  userId: string
  latestJobId?: string
  mfgScore: number
  mfgGrade: string
}

export interface AnalysisJob {
  id: string
  submissionId: string
  profileId: string
  status: 'PENDING' | 'PROCESSING' | 'DONE' | 'FAILED'
  startedAt?: string
  completedAt?: string
  errorMsg?: string
  mfgScore: number
  mfgGrade: string
}

export interface Violation {
  id: string
  jobId: string
  ruleId: string
  severity: 'ERROR' | 'WARNING' | 'INFO'
  layer: string
  x: number
  y: number
  message: string
  suggestion: string
  count: number
  measuredMM: number
  limitMM:    number
  unit:       string
  netName:    string
  refDes:     string
  x2:         number
  y2:         number
  ignored:    boolean
}

export interface BoardLayer { name: string; type: string }
export interface BoardTrace { layer: string; widthMM: number; startX: number; startY: number; endX: number; endY: number; netName: string }
export interface BoardPad { layer: string; x: number; y: number; widthMM: number; heightMM: number; shape: 'RECT' | 'CIRCLE' | 'OVAL'; netName: string; refDes: string }
export interface BoardVia { x: number; y: number; outerDiamMM: number; drillDiamMM: number }
export interface BoardDrill { x: number; y: number; diamMM: number; plated: boolean }
export interface BoardData {
  layers: BoardLayer[]
  traces: BoardTrace[]
  pads: BoardPad[]
  vias: BoardVia[]
  drills: BoardDrill[]
  outline: Array<{ x: number; y: number }>
  boardThicknessMM: number
}

export interface ProfileRules {
  minTraceWidthMM: number
  minClearanceMM: number
  minDrillDiamMM: number
  maxDrillDiamMM: number
  minAnnularRingMM: number
  maxAspectRatio: number
  minSolderMaskDamMM: number
  minEdgeClearanceMM: number
}

export interface CapabilityProfile {
  id: string
  orgId: string
  name: string
  isDefault: boolean
  rules: ProfileRules
  createdAt: string
  updatedAt: string
}

export interface SubmissionOverview {
  overview: string
  counts: {
    errors: number
    warnings: number
    infos: number
  }
  generatedWith: 'ai' | 'fallback'
}

// ── Fetch helper ─────────────────────────────────────────────────────────────

async function apiFetch<T>(path: string, init?: RequestInit): Promise<T> {
  const token = getStoredToken()
  const res = await fetch(`${API_URL}${path}`, {
    ...init,
    headers: {
      'Content-Type': 'application/json',
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      ...(init?.headers ?? {}),
    },
  })
  if (res.status === 401) {
    clearToken()
    window.location.replace('/login')
    return undefined as T
  }
  if (!res.ok) {
    const text = await res.text().catch(() => res.statusText)
    throw new Error(`API ${path}: ${res.status} ${text}`)
  }
  if (res.status === 204) return undefined as T
  return res.json()
}

// ── Submissions ───────────────────────────────────────────────────────────────

export async function getSubmissions(): Promise<Submission[]> {
  return apiFetch<Submission[]>('/submissions')
}

export async function createSubmission(
  filename: string,
  fileType: string
): Promise<{ submissionId: string; presignedUrl: string; fileKey: string }> {
  return apiFetch('/submissions', {
    method: 'POST',
    body: JSON.stringify({ filename, fileType }),
  })
}

export async function startAnalysis(
  submissionId: string,
  profileId?: string
): Promise<AnalysisJob> {
  return apiFetch(`/submissions/${submissionId}/analyze`, {
    method: 'POST',
    body: JSON.stringify({ profileId: profileId ?? '' }),
  })
}

// ── S3 direct upload with progress ───────────────────────────────────────────

export function uploadToS3(
  presignedUrl: string,
  file: File,
  onProgress?: (pct: number) => void
): Promise<void> {
  return new Promise((resolve, reject) => {
    if (!presignedUrl) {
      // Dev mode without real S3 — skip upload
      resolve()
      return
    }
    const xhr = new XMLHttpRequest()
    xhr.open('PUT', presignedUrl)
    if (onProgress) {
      xhr.upload.addEventListener('progress', (e) => {
        if (e.lengthComputable) onProgress(Math.round((e.loaded / e.total) * 100))
      })
    }
    xhr.addEventListener('load', () => {
      if (xhr.status >= 200 && xhr.status < 300) resolve()
      else reject(new Error(`S3 upload failed: ${xhr.status}`))
    })
    xhr.addEventListener('error', () => reject(new Error('S3 upload network error')))
    xhr.send(file)
  })
}

// ── Jobs ──────────────────────────────────────────────────────────────────────

export async function getJob(jobId: string): Promise<AnalysisJob> {
  return apiFetch<AnalysisJob>(`/jobs/${jobId}`)
}

export async function getViolations(jobId: string): Promise<Violation[]> {
  return apiFetch<Violation[]>(`/jobs/${jobId}/violations`)
}

export async function getBoardData(jobId: string): Promise<BoardData> {
  return apiFetch<BoardData>(`/jobs/${jobId}/board`)
}

export async function getSubmissionOverview(jobId: string): Promise<SubmissionOverview> {
  const token = getStoredToken()
  const res = await fetch('/api/ai/submission-overview', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
    body: JSON.stringify({ jobId }),
  })

  const data = await res.json().catch(() => ({}))
  if (!res.ok) {
    throw new Error(data.error || 'Failed to generate overview')
  }
  return data as SubmissionOverview
}

export async function ignoreLayerViolations(
  jobId: string,
  layer: string,
  ignored: boolean,
  severity?: string
): Promise<{ layer: string; ignored: boolean; mfgScore: number; mfgGrade: string }> {
  return apiFetch(`/jobs/${jobId}/violations/by-layer`, {
    method: 'PATCH',
    body: JSON.stringify({ layer, ignored, ...(severity ? { severity } : {}) }),
  })
}

export async function patchViolation(
  id: string,
  patch: { ignored: boolean }
): Promise<{ id: string; ignored: boolean; mfgScore: number; mfgGrade: string }> {
  return apiFetch(`/violations/${id}`, { method: 'PATCH', body: JSON.stringify(patch) })
}

// ── Profiles ─────────────────────────────────────────────────────────────────

export async function getProfiles(): Promise<CapabilityProfile[]> {
  return apiFetch<CapabilityProfile[]>('/profiles')
}

export async function createProfile(data: {
  name: string
  isDefault: boolean
  rules: ProfileRules
}): Promise<CapabilityProfile> {
  return apiFetch('/profiles', { method: 'POST', body: JSON.stringify(data) })
}

export async function updateProfile(
  id: string,
  data: Partial<{ name: string; isDefault: boolean; rules: ProfileRules }>
): Promise<CapabilityProfile> {
  return apiFetch(`/profiles/${id}`, { method: 'PUT', body: JSON.stringify(data) })
}

export async function deleteProfile(id: string): Promise<void> {
  return apiFetch(`/profiles/${id}`, { method: 'DELETE' })
}
