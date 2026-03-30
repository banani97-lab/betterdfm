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
  projectId?: string
  latestJobId?: string
  mfgScore: number
  mfgGrade: string
}

export interface Project {
  id: string
  orgId: string
  name: string
  description: string
  customerRef: string
  createdBy: string
  archived: boolean
  createdAt: string
  updatedAt: string
  submissionCount: number
  avgScore: number
  latestGrade: string
  lastActivityAt?: string
}

export interface Organization {
  id: string
  slug: string
  name: string
  logoUrl: string
  createdAt: string
}

export interface User {
  id: string
  orgId: string
  cognitoSub: string
  email: string
  role: 'ADMIN' | 'ANALYST' | 'VIEWER'
  createdAt: string
}

export interface AnalysisJob {
  id: string
  orgId: string
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
  orgId: string
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
export interface BoardPad { layer: string; x: number; y: number; widthMM: number; heightMM: number; shape: 'RECT' | 'CIRCLE' | 'OVAL'; netName: string; refDes: string; packageClass?: string }
export interface BoardVia { x: number; y: number; outerDiamMM: number; drillDiamMM: number }
export interface BoardDrill { x: number; y: number; diamMM: number; plated: boolean }
export interface BoardPolygon { layer: string; points: Array<{ x: number; y: number }>; holes?: Array<Array<{ x: number; y: number }>>; netName: string }
export interface BoardData {
  layers: BoardLayer[]
  traces: BoardTrace[]
  pads: BoardPad[]
  vias: BoardVia[]
  drills: BoardDrill[]
  outline: Array<{ x: number; y: number }>
  boardThicknessMM: number
  warnings?: string[]
  polygons?: BoardPolygon[]
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
  minDrillToDrillMM: number
  minDrillToCopperMM: number
  minCopperSliverMM: number
  smallestPackageClass?: string
  maxTraceImbalanceRatio?: number
  enableSilkscreenOnPadCheck?: boolean
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

export interface UsageSummary {
  tier: string // "STARTER" | "PROFESSIONAL" | "ENTERPRISE"
  billingPeriodStart: string
  billingPeriodEnd: string
  analyses: { used: number; limit: number; overage: number }
  users: { used: number; limit: number }
  profiles: { used: number; limit: number }
  projects: { used: number; limit: number }
  shareLinks: { used: number; limit: number }
  features: {
    batchUpload: boolean
    maxBatchFiles: number
    compare: boolean
    adminDashboard: boolean
    customerPortal: boolean
  }
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
  if (res.status === 403) {
    const text = await res.text().catch(() => res.statusText)
    throw new Error(text || `API ${path}: 403 Forbidden`)
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
  fileType: string,
  projectId?: string
): Promise<{ submissionId: string; presignedUrl: string; fileKey: string }> {
  return apiFetch('/submissions', {
    method: 'POST',
    body: JSON.stringify({ filename, fileType, ...(projectId ? { projectId } : {}) }),
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

// ── Batches ──────────────────────────────────────────────────────────────────

export interface BatchSubmission extends Submission {
  latestJobId: string
  jobStatus: string
  mfgScore: number
  mfgGrade: string
}

export interface Batch {
  id: string
  orgId: string
  projectId?: string
  userId: string
  profileId?: string
  status: 'PENDING' | 'PROCESSING' | 'DONE' | 'PARTIAL_FAIL'
  total: number
  completed: number
  failed: number
  createdAt: string
  updatedAt: string
}

export interface BatchDetail {
  batch: Batch
  submissions: BatchSubmission[]
  avgScore: number | null
}

export interface CreateBatchResponse {
  batchId: string
  submissions: Array<{
    submissionId: string
    filename: string
    presignedUrl: string
  }>
}

export async function createBatch(
  files: Array<{ filename: string; fileType: string }>,
  projectId?: string,
  profileId?: string
): Promise<CreateBatchResponse> {
  return apiFetch('/batches', {
    method: 'POST',
    body: JSON.stringify({ files, projectId, profileId }),
  })
}

export async function getBatch(batchId: string): Promise<BatchDetail> {
  return apiFetch<BatchDetail>(`/batches/${batchId}`)
}

export async function analyzeBatch(
  batchId: string,
  profileId?: string
): Promise<{ batchId: string; jobIds: string[] }> {
  return apiFetch(`/batches/${batchId}/analyze`, {
    method: 'POST',
    body: JSON.stringify({ profileId: profileId ?? '' }),
  })
}

export async function retryBatch(
  batchId: string
): Promise<{ batchId: string; jobIds: string[] }> {
  return apiFetch(`/batches/${batchId}/retry`, {
    method: 'POST',
  })
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

// ── Projects ──────────────────────────────────────────────────────────────────

export async function getProjects(q?: string, archived?: boolean): Promise<Project[]> {
  const params = new URLSearchParams()
  if (q) params.set('q', q)
  if (archived !== undefined) params.set('archived', String(archived))
  const qs = params.toString()
  return apiFetch<Project[]>(`/projects${qs ? `?${qs}` : ''}`)
}

export async function getProject(id: string): Promise<Project> {
  return apiFetch<Project>(`/projects/${id}`)
}

export async function createProject(data: {
  name: string
  description?: string
  customerRef?: string
}): Promise<Project> {
  return apiFetch('/projects', { method: 'POST', body: JSON.stringify(data) })
}

export async function updateProject(
  id: string,
  data: Partial<{ name: string; description: string; customerRef: string }>
): Promise<Project> {
  return apiFetch(`/projects/${id}`, { method: 'PUT', body: JSON.stringify(data) })
}

export async function archiveProject(id: string): Promise<Project> {
  return apiFetch(`/projects/${id}`, { method: 'DELETE' })
}

export async function getProjectSubmissions(projectId: string): Promise<Submission[]> {
  return apiFetch<Submission[]>(`/projects/${projectId}/submissions`)
}

// ── Share Links ──────────────────────────────────────────────────────────────

export interface ShareLink {
  id: string
  orgId: string
  token: string
  projectId?: string | null
  jobId?: string | null
  createdBy: string
  expiresAt?: string | null
  allowUpload: boolean
  active: boolean
  label: string
  createdAt: string
  shareUrl?: string
}

export interface ShareUpload {
  id: string
  shareLinkId: string
  submissionId: string
  uploaderName: string
  uploaderEmail: string
  createdAt: string
}

export interface ShareInfo {
  id: string
  label: string
  allowUpload: boolean
  expiresAt?: string | null
  orgName: string
  orgLogoUrl: string
  shareType: 'project' | 'job'
  projectId?: string
  jobId?: string
  job?: {
    id: string
    status: string
    mfgScore: number
    mfgGrade: string
    completedAt?: string
  }
}

export async function createShareLink(data: {
  projectId?: string
  jobId?: string
  label: string
  expiresAt?: string
  allowUpload?: boolean
}): Promise<ShareLink> {
  return apiFetch('/share-links', { method: 'POST', body: JSON.stringify(data) })
}

export async function getShareLinks(): Promise<ShareLink[]> {
  return apiFetch<ShareLink[]>('/share-links')
}

export async function deactivateShareLink(id: string): Promise<{ id: string; active: boolean }> {
  return apiFetch(`/share-links/${id}`, { method: 'DELETE' })
}

export async function getShareUploads(linkId: string): Promise<ShareUpload[]> {
  return apiFetch<ShareUpload[]>(`/share-links/${linkId}/uploads`)
}

// ── Public share fetch (no auth) ─────────────────────────────────────────────

async function shareFetch<T>(token: string, path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${API_URL}/shared/${token}${path}`, {
    ...init,
    headers: {
      'Content-Type': 'application/json',
      ...(init?.headers ?? {}),
    },
  })
  if (!res.ok) {
    const text = await res.text().catch(() => res.statusText)
    throw new Error(`Share API ${path}: ${res.status} ${text}`)
  }
  if (res.status === 204) return undefined as T
  return res.json()
}

export async function getShareInfo(token: string): Promise<ShareInfo> {
  return shareFetch<ShareInfo>(token, '')
}

export async function getSharedSubmissions(token: string): Promise<Submission[]> {
  return shareFetch<Submission[]>(token, '/submissions')
}

export async function getSharedJob(token: string, jobId: string): Promise<AnalysisJob> {
  return shareFetch<AnalysisJob>(token, `/jobs/${jobId}`)
}

export async function getSharedViolations(token: string, jobId: string): Promise<Violation[]> {
  return shareFetch<Violation[]>(token, `/jobs/${jobId}/violations`)
}

export async function getSharedBoardData(token: string, jobId: string): Promise<BoardData> {
  return shareFetch<BoardData>(token, `/jobs/${jobId}/board`)
}

export async function sharedUpload(
  token: string,
  data: { filename: string; fileType: string; uploaderName?: string; uploaderEmail?: string }
): Promise<{ submissionId: string; presignedUrl: string; fileKey: string }> {
  return shareFetch(token, '/upload', { method: 'POST', body: JSON.stringify(data) })
}

export async function sharedAnalyze(
  token: string,
  submissionId: string
): Promise<{ jobId: string }> {
  return shareFetch(token, `/analyze/${submissionId}`, { method: 'POST' })
}

// ── Compare ──────────────────────────────────────────────────────────────────

export interface ComparisonJobSummary {
  id: string
  mfgScore: number
  mfgGrade: string
  filename: string
  completedAt: string | null
}

export interface ComparisonResult {
  jobA: ComparisonJobSummary
  jobB: ComparisonJobSummary
  scoreDelta: number
  summary: { fixedCount: number; newCount: number; unchangedCount: number }
  fixed: Violation[]
  new: Violation[]
  unchanged: Array<{ a: Violation; b: Violation }>
}

export async function compareJobs(jobAId: string, jobBId: string): Promise<ComparisonResult> {
  return apiFetch<ComparisonResult>(`/compare?jobA=${encodeURIComponent(jobAId)}&jobB=${encodeURIComponent(jobBId)}`)
}

// ── Usage ───────────────────────────────────────────────────────────────────

export async function getUsage(): Promise<UsageSummary> {
  return apiFetch<UsageSummary>('/usage')
}
