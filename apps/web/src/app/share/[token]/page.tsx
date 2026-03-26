'use client'

import { useEffect, useState, useCallback, useMemo } from 'react'
import { useParams } from 'next/navigation'
import { AlertCircle, AlertTriangle, CheckCircle, Info, Upload, ListFilter, ChevronDown, ChevronUp, ChevronLeft, ChevronRight } from 'lucide-react'
import {
  getShareInfo,
  getSharedJob,
  getSharedViolations,
  getSharedBoardData,
  getSharedSubmissions,
  sharedUpload,
  uploadToS3,
  type ShareInfo,
  type AnalysisJob,
  type Violation,
  type BoardData,
  type Submission,
} from '@/lib/api'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { ViolationList, type SeverityFilter } from '@/components/ui/ViolationList'
import { BoardViewer } from '@/components/ui/BoardViewer'
import { cn } from '@/lib/utils'

function scoreColor(n: number): string {
  if (n >= 90) return '#16a34a'
  if (n >= 75) return '#ca8a04'
  if (n >= 60) return '#ea580c'
  return '#dc2626'
}

export default function SharedPage() {
  const { token } = useParams<{ token: string }>()
  const [shareInfo, setShareInfo] = useState<ShareInfo | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  // Job view state
  const [job, setJob] = useState<AnalysisJob | null>(null)
  const [violations, setViolations] = useState<Violation[]>([])
  const [boardData, setBoardData] = useState<BoardData | null>(null)
  const [selectedId, setSelectedId] = useState<string | undefined>()
  const [hiddenLayers, setHiddenLayers] = useState<Set<string>>(new Set())
  const [severityFilter, setSeverityFilter] = useState<SeverityFilter>('ERROR')
  const [violationsOpen, setViolationsOpen] = useState(true)
  const [isPortraitMobile, setIsPortraitMobile] = useState(false)

  // Project view state
  const [submissions, setSubmissions] = useState<Submission[]>([])
  const [selectedJobId, setSelectedJobId] = useState<string | null>(null)

  // Upload state
  const [uploading, setUploading] = useState(false)
  const [uploadProgress, setUploadProgress] = useState(0)
  const [uploadName, setUploadName] = useState('')
  const [uploadEmail, setUploadEmail] = useState('')
  const [uploadFileType, setUploadFileType] = useState<'GERBER' | 'ODB_PLUS_PLUS'>('GERBER')
  const [dragOver, setDragOver] = useState(false)
  const [uploadSuccess, setUploadSuccess] = useState(false)

  const toggleLayer = (name: string) => {
    setHiddenLayers((prev) => {
      const next = new Set(prev)
      if (next.has(name)) next.delete(name); else next.add(name)
      return next
    })
  }

  // Load share info
  useEffect(() => {
    const load = async () => {
      try {
        const info = await getShareInfo(token)
        setShareInfo(info)

        if (info.shareType === 'job' && info.jobId) {
          // Load job data directly
          const [jobData, violationsData] = await Promise.all([
            getSharedJob(token, info.jobId),
            getSharedViolations(token, info.jobId),
          ])
          setJob(jobData)
          setViolations(violationsData ?? [])
          getSharedBoardData(token, info.jobId).then(setBoardData).catch(() => {})
        } else if (info.shareType === 'project') {
          const subs = await getSharedSubmissions(token)
          setSubmissions(subs ?? [])
        }
      } catch (e: unknown) {
        if (e instanceof Error && e.message.includes('410')) {
          setError('This share link has expired.')
        } else if (e instanceof Error && e.message.includes('404')) {
          setError('This share link is not valid or has been deactivated.')
        } else {
          setError(e instanceof Error ? e.message : 'Failed to load shared content')
        }
      } finally {
        setLoading(false)
      }
    }
    load()
  }, [token])

  // Load job when selected from project view
  useEffect(() => {
    if (!selectedJobId) return
    const load = async () => {
      try {
        const [jobData, violationsData] = await Promise.all([
          getSharedJob(token, selectedJobId),
          getSharedViolations(token, selectedJobId),
        ])
        setJob(jobData)
        setViolations(violationsData ?? [])
        getSharedBoardData(token, selectedJobId).then(setBoardData).catch(() => {})
      } catch {
        setError('Failed to load job results')
      }
    }
    load()
  }, [token, selectedJobId])

  // Portrait mobile detection
  useEffect(() => {
    const orientationQuery = window.matchMedia('(orientation: portrait)')
    const mobileWidthQuery = window.matchMedia('(max-width: 900px)')
    const update = () => setIsPortraitMobile(orientationQuery.matches && mobileWidthQuery.matches)
    update()
    if (orientationQuery.addEventListener) {
      orientationQuery.addEventListener('change', update)
      mobileWidthQuery.addEventListener('change', update)
    } else {
      orientationQuery.addListener(update)
      mobileWidthQuery.addListener(update)
    }
    window.addEventListener('resize', update)
    return () => {
      if (orientationQuery.removeEventListener) {
        orientationQuery.removeEventListener('change', update)
        mobileWidthQuery.removeEventListener('change', update)
      } else {
        orientationQuery.removeListener(update)
        mobileWidthQuery.removeListener(update)
      }
      window.removeEventListener('resize', update)
    }
  }, [])

  useEffect(() => {
    setViolationsOpen(!isPortraitMobile)
  }, [isPortraitMobile])

  const handleFileUpload = useCallback(async (file: File) => {
    if (!shareInfo?.allowUpload) return
    setUploading(true)
    setUploadProgress(0)
    setUploadSuccess(false)
    try {
      const result = await sharedUpload(token, {
        filename: file.name,
        fileType: uploadFileType,
        uploaderName: uploadName,
        uploaderEmail: uploadEmail,
      })
      if (result.presignedUrl) {
        await uploadToS3(result.presignedUrl, file, setUploadProgress)
      }
      setUploadProgress(100)
      setUploadSuccess(true)
      // Refresh submissions list to show the new upload
      getSharedSubmissions(token).then(subs => setSubmissions(subs ?? [])).catch(() => {})
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Upload failed')
    } finally {
      setUploading(false)
    }
  }, [token, shareInfo, uploadName, uploadEmail])

  // Computed values for violation display
  const layerFiltered = violations.filter((v) => !v.layer || !hiddenLayers.has(v.layer))
  const visibleViolations = layerFiltered.filter((v) => {
    if (severityFilter === 'NONE') return false
    return v.severity === severityFilter
  })

  const violationLayers = useMemo(() => {
    const s = new Set<string>()
    for (const v of visibleViolations) {
      if (v.layer && !v.ignored) s.add(v.layer)
    }
    return s
  }, [visibleViolations])

  const allIgnoredLayers = useMemo(() => {
    const counts = new Map<string, { total: number; ignored: number }>()
    for (const v of violations) {
      if (!v.layer) continue
      const c = counts.get(v.layer) ?? { total: 0, ignored: 0 }
      c.total++
      if (v.ignored) c.ignored++
      counts.set(v.layer, c)
    }
    const s = new Set<string>()
    for (const [layer, c] of counts) {
      if (c.total > 0 && c.total === c.ignored) s.add(layer)
    }
    return s
  }, [violations])

  const errorCount = violations.filter((v) => v.severity === 'ERROR' && !v.ignored).length
  const warningCount = violations.filter((v) => v.severity === 'WARNING' && !v.ignored).length
  const infoCount = violations.filter((v) => v.severity === 'INFO' && !v.ignored).length
  const collapseToBottom = isPortraitMobile

  if (loading) {
    return (
      <div className="flex items-center justify-center min-h-screen bg-background">
        <div className="animate-spin h-8 w-8 border-4 border-blue-600 border-t-transparent rounded-full" />
      </div>
    )
  }

  if (error) {
    return (
      <div className="flex flex-col items-center justify-center min-h-screen gap-4 bg-background">
        <AlertCircle className="h-12 w-12 text-red-400" />
        <p className="text-muted-foreground text-center max-w-md">{error}</p>
      </div>
    )
  }

  if (!shareInfo) return null

  // Project list view (before a job is selected)
  if (shareInfo.shareType === 'project' && !selectedJobId) {
    return (
      <div className="min-h-screen bg-background">
        <header className="bg-card border-b px-6 py-5 flex items-center gap-4">
          {shareInfo.orgLogoUrl && (
            <img src={shareInfo.orgLogoUrl} alt={shareInfo.orgName} className="h-8 w-auto" />
          )}
          <div>
            <h1 className="text-lg font-semibold text-foreground">{shareInfo.orgName}</h1>
            <p className="text-sm text-muted-foreground">{shareInfo.label}</p>
          </div>
        </header>

        {/* Upload section */}
        {shareInfo.allowUpload && (
          <div className="max-w-2xl mx-auto mt-8 px-4">
            <div className="bg-card border border-border rounded-xl overflow-hidden shadow-sm">
              <div className="px-5 py-4 border-b border-border">
                <h3 className="text-sm font-semibold">Submit a Revision</h3>
                <p className="text-xs text-muted-foreground mt-0.5">Upload an updated design file for DFM analysis</p>
              </div>

              {uploadSuccess ? (
                <div className="px-5 py-10 text-center">
                  <CheckCircle className="h-12 w-12 mx-auto text-green-500 mb-3" />
                  <p className="text-base font-semibold text-green-600">Upload successful</p>
                  <p className="text-sm text-muted-foreground mt-1 max-w-sm mx-auto">
                    Your file has been submitted for analysis. Results will appear in the list below once processing completes.
                  </p>
                  <button
                    onClick={() => setUploadSuccess(false)}
                    className="mt-4 inline-flex items-center gap-1.5 text-sm text-primary hover:text-primary/80 font-medium transition-colors"
                  >
                    <Upload className="h-3.5 w-3.5" />
                    Upload another file
                  </button>
                </div>
              ) : (
                <div className="p-5 space-y-4">
                  {/* Contact info */}
                  <div className="grid grid-cols-2 gap-3">
                    <div>
                      <label className="block text-xs font-medium text-muted-foreground mb-1.5">Your name</label>
                      <input
                        type="text"
                        placeholder="Jane Smith"
                        value={uploadName}
                        onChange={(e) => setUploadName(e.target.value)}
                        className="w-full px-3 py-2 text-sm border border-border rounded-lg bg-background focus:outline-none focus:ring-2 focus:ring-ring transition-shadow"
                      />
                    </div>
                    <div>
                      <label className="block text-xs font-medium text-muted-foreground mb-1.5">Your email</label>
                      <input
                        type="email"
                        placeholder="jane@company.com"
                        value={uploadEmail}
                        onChange={(e) => setUploadEmail(e.target.value)}
                        className="w-full px-3 py-2 text-sm border border-border rounded-lg bg-background focus:outline-none focus:ring-2 focus:ring-ring transition-shadow"
                      />
                    </div>
                  </div>

                  {/* File type */}
                  <div>
                    <label className="block text-xs font-medium text-muted-foreground mb-1.5">File format</label>
                    <div className="flex gap-2">
                      {([['GERBER', 'Gerber'], ['ODB_PLUS_PLUS', 'ODB++']] as const).map(([value, label]) => (
                        <button
                          key={value}
                          type="button"
                          onClick={() => setUploadFileType(value)}
                          className={cn(
                            'flex-1 py-2 text-sm font-medium rounded-lg border transition-all',
                            uploadFileType === value
                              ? 'border-primary bg-primary/10 text-primary'
                              : 'border-border text-muted-foreground hover:border-primary/40 hover:text-foreground'
                          )}
                        >
                          {label}
                        </button>
                      ))}
                    </div>
                  </div>

                  {/* Drop zone */}
                  <label
                    className={cn(
                      "flex flex-col items-center justify-center gap-2 py-8 rounded-lg border-2 border-dashed cursor-pointer transition-all",
                      dragOver
                        ? "border-primary bg-primary/5"
                        : "border-border hover:border-primary/40 hover:bg-muted/30"
                    )}
                    onDragOver={(e) => { e.preventDefault(); setDragOver(true) }}
                    onDragLeave={() => setDragOver(false)}
                    onDrop={(e) => {
                      e.preventDefault()
                      setDragOver(false)
                      const file = e.dataTransfer.files?.[0]
                      if (file) handleFileUpload(file)
                    }}
                  >
                    <div className={cn(
                      "h-10 w-10 rounded-full flex items-center justify-center transition-colors",
                      dragOver ? "bg-primary/10 text-primary" : "bg-muted text-muted-foreground"
                    )}>
                      <Upload className="h-5 w-5" />
                    </div>
                    <div className="text-center">
                      <span className="text-sm font-medium">
                        {dragOver ? 'Drop file here' : 'Drag and drop your file here'}
                      </span>
                      <p className="text-xs text-muted-foreground mt-0.5">or click to browse (.zip, .tar, .tgz)</p>
                    </div>
                    <input
                      type="file"
                      accept=".zip,.tgz,.tar,.tar.gz"
                      disabled={uploading}
                      onChange={(e) => {
                        const file = e.target.files?.[0]
                        if (file) handleFileUpload(file)
                      }}
                      className="sr-only"
                    />
                  </label>

                  {/* Progress bar */}
                  {uploading && (
                    <div className="space-y-1.5">
                      <div className="flex justify-between text-xs text-muted-foreground">
                        <span>Uploading...</span>
                        <span>{uploadProgress}%</span>
                      </div>
                      <div className="w-full bg-muted rounded-full h-2">
                        <div className="bg-primary h-2 rounded-full transition-all duration-300" style={{ width: `${uploadProgress}%` }} />
                      </div>
                    </div>
                  )}
                </div>
              )}
            </div>
          </div>
        )}

        {/* Submissions list */}
        <div className="max-w-3xl mx-auto mt-6 px-4 pb-8">
          <h2 className="text-sm font-medium text-muted-foreground mb-3">Submissions</h2>
          {submissions.length === 0 ? (
            <p className="text-muted-foreground text-sm">No submissions yet.</p>
          ) : (
            <div className="space-y-2">
              {submissions.map((sub) => (
                <button
                  key={sub.id}
                  onClick={() => sub.latestJobId && setSelectedJobId(sub.latestJobId)}
                  disabled={!sub.latestJobId || sub.status !== 'DONE'}
                  className="w-full text-left bg-card border rounded-lg p-4 hover:bg-muted/50 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
                >
                  <div className="flex items-center justify-between">
                    <div>
                      <p className="font-medium text-foreground">{sub.filename}</p>
                      <p className="text-xs text-muted-foreground">
                        {new Date(sub.createdAt).toLocaleDateString()} &middot; {sub.status}
                      </p>
                    </div>
                    {sub.mfgScore > 0 && (
                      <div
                        className="px-2 py-0.5 rounded font-mono text-sm font-bold text-white"
                        style={{ background: scoreColor(sub.mfgScore) }}
                      >
                        {sub.mfgScore} <span className="text-xs opacity-80">{sub.mfgGrade}</span>
                      </div>
                    )}
                  </div>
                </button>
              ))}
            </div>
          )}
        </div>
      </div>
    )
  }

  // Job results view (for job shares or after selecting a submission)
  return (
    <div className="flex flex-col min-h-screen h-[100dvh] md:h-screen bg-background">
      {/* Header */}
      <header className="bg-card border-b px-4 py-3 md:px-6 md:py-5 flex flex-wrap md:flex-nowrap items-center gap-3 md:gap-4 flex-shrink-0">
        <div className="flex items-center gap-3">
          {shareInfo.orgLogoUrl && (
            <img src={shareInfo.orgLogoUrl} alt={shareInfo.orgName} className="h-7 w-auto" />
          )}
          <span className="text-sm font-medium text-muted-foreground">{shareInfo.orgName}</span>
        </div>
        <div className="flex items-center gap-4 flex-1 min-w-0">
          {selectedJobId && shareInfo.shareType === 'project' && (
            <Button variant="outline" size="sm" onClick={() => {
              setSelectedJobId(null)
              setJob(null)
              setViolations([])
              setBoardData(null)
            }}>
              Back
            </Button>
          )}
          <div className="min-w-0">
            <h1 className="text-lg font-semibold text-foreground">DFM Results</h1>
            <p className="text-xs text-muted-foreground">{shareInfo.label}</p>
          </div>
        </div>
        {/* Summary badges */}
        <div className="order-3 md:order-2 w-full md:w-auto flex items-center gap-2">
          <div className="flex items-center gap-1">
            <AlertCircle className="h-4 w-4 text-red-500" />
            <Badge variant="destructive">{errorCount}</Badge>
          </div>
          <div className="flex items-center gap-1">
            <AlertTriangle className="h-4 w-4 text-yellow-500" />
            <Badge variant="warning">{warningCount}</Badge>
          </div>
          <div className="flex items-center gap-1">
            <Info className="h-4 w-4 text-blue-500" />
            <Badge variant="info">{infoCount}</Badge>
          </div>
          {job && job.mfgScore > 0 && (
            <div
              className="px-2 py-0.5 rounded font-mono text-sm font-bold text-white"
              style={{ background: scoreColor(job.mfgScore) }}
            >
              {job.mfgScore} <span className="text-xs opacity-80">{job.mfgGrade}</span>
            </div>
          )}
        </div>
      </header>

      {/* Upload bar */}
      {shareInfo.allowUpload && (
        <div className="bg-card border-b px-4 py-2 flex items-center gap-3">
          <Upload className="h-4 w-4 text-muted-foreground" />
          <span className="text-sm text-muted-foreground">Upload revision:</span>
          <input
            type="text"
            placeholder="Name"
            value={uploadName}
            onChange={(e) => setUploadName(e.target.value)}
            className="px-2 py-1 text-sm border rounded bg-background w-28"
          />
          <input
            type="email"
            placeholder="Email"
            value={uploadEmail}
            onChange={(e) => setUploadEmail(e.target.value)}
            className="px-2 py-1 text-sm border rounded bg-background w-36"
          />
          <input
            type="file"
            accept=".zip,.tgz,.tar.gz"
            disabled={uploading}
            onChange={(e) => {
              const file = e.target.files?.[0]
              if (file) handleFileUpload(file)
            }}
            className="text-sm"
          />
          {uploading && (
            <div className="flex-1 max-w-32">
              <div className="w-full bg-muted rounded-full h-2">
                <div className="bg-blue-600 h-2 rounded-full transition-all" style={{ width: `${uploadProgress}%` }} />
              </div>
            </div>
          )}
        </div>
      )}

      {/* Body: board + collapsible issues panel */}
      <div className={cn('flex flex-1 min-h-0 overflow-hidden', collapseToBottom ? 'flex-col' : 'flex-row')}>
        <div className={cn('order-1 flex-1 min-h-0 min-w-0 overflow-hidden', collapseToBottom ? 'p-2 sm:p-3' : 'p-2 sm:p-3 md:p-4')}>
          <BoardViewer
            violations={visibleViolations.filter((v) => !v.ignored)}
            boardData={boardData}
            selectedViolationId={selectedId}
            onViolationClick={(v) => setSelectedId(v?.id)}
            hiddenLayers={hiddenLayers}
            onToggleLayer={toggleLayer}
            onSetHiddenLayers={setHiddenLayers}
            violationLayers={violationLayers}
            allIgnoredLayers={allIgnoredLayers}
          />
        </div>

        <section
          className={cn(
            'order-2 bg-card overflow-hidden flex',
            collapseToBottom ? 'flex-col border-t border-border/80' : 'flex-row border-l border-border/80',
            collapseToBottom
              ? (violationsOpen ? 'h-[44dvh] min-h-56 max-h-[68dvh]' : 'h-10')
              : (violationsOpen ? 'w-[18.5rem] md:w-96' : 'w-12')
          )}
          aria-label="Violations panel"
        >
          <div
            className={cn(
              'bg-card/85 backdrop-blur supports-[backdrop-filter]:bg-card/75',
              collapseToBottom
                ? 'h-10 px-3 border-b border-border/70 flex items-center justify-between'
                : 'w-12 border-r border-border/70 flex flex-col items-center gap-2 py-2'
            )}
          >
            {collapseToBottom ? (
              <div className="inline-flex items-center gap-2 text-sm font-medium text-foreground">
                <ListFilter className="h-4 w-4 text-muted-foreground" />
                Issues
                <span className="text-xs text-muted-foreground">{layerFiltered.length}</span>
              </div>
            ) : (
              <>
                <ListFilter className="h-4 w-4 text-muted-foreground" />
                <span className="[writing-mode:vertical-rl] rotate-180 text-[11px] tracking-wide uppercase text-muted-foreground select-none">
                  Issues
                </span>
              </>
            )}

            <button
              type="button"
              onClick={() => setViolationsOpen((v) => !v)}
              aria-label={violationsOpen ? 'Collapse issues panel' : 'Expand issues panel'}
              title={violationsOpen ? 'Collapse issues panel' : 'Expand issues panel'}
              className="inline-flex items-center justify-center h-8 w-8 rounded-md border border-border/70 bg-background/70 text-foreground hover:bg-muted/60 transition-colors"
            >
              {collapseToBottom ? (
                violationsOpen ? <ChevronDown className="h-4 w-4" /> : <ChevronUp className="h-4 w-4" />
              ) : (
                violationsOpen ? <ChevronLeft className="h-4 w-4" /> : <ChevronRight className="h-4 w-4" />
              )}
            </button>
          </div>

          {violationsOpen && (
            <div className="flex-1 min-h-0 min-w-0">
              <ViolationList
                violations={visibleViolations}
                allViolations={layerFiltered}
                selectedId={selectedId}
                onSelect={(v) => setSelectedId(prev => prev === v.id ? undefined : v.id)}
                filter={severityFilter}
                onFilterChange={setSeverityFilter}
              />
            </div>
          )}
        </section>
      </div>
    </div>
  )
}
