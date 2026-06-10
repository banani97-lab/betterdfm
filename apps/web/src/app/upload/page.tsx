'use client'

import { Suspense, useEffect, useRef, useState } from 'react'
import { useRouter, useSearchParams } from 'next/navigation'
import Link from 'next/link'
import { CheckCircle, XCircle, Upload, File, X, FolderOpen, Clock, AlertTriangle } from 'lucide-react'
import {
  createSubmission,
  getSubmission,
  uploadToS3,
  startAnalysis,
  getProfiles,
  createBatch,
  analyzeBatch,
  ApiError,
  type CapabilityProfile,
  type AnalysisJob,
  type BatchDetail,
  type Submission,
} from '@/lib/api'
import { pollJobUntilDone, pollBatchUntilDone, PollTimeoutError } from '@/lib/poll'
import { isLoggedIn, canWrite } from '@/lib/auth'
import { useUsage } from '@/lib/useUsage'
import { AppBackButton } from '@/components/ui/app-back-button'
import { Button } from '@/components/ui/button'
import { RapidDFMLogo } from '@/components/ui/rapiddfm-logo'
import { AppTaskbar } from '@/components/ui/app-taskbar'
import { cn } from '@/lib/utils'
import { track } from '@/lib/analytics'
import { readDirectoryEntry, packageFilesAsTar } from '@/lib/tarball'

type Step = 'select' | 'resume' | 'packaging' | 'uploading' | 'analyzing' | 'partial' | 'timeout' | 'done' | 'error'

const STEPS = ['select', 'uploading', 'analyzing', 'done']

/** Client-side cap on uploads — matches the folder-packaging cap in tarball.ts. */
const MAX_UPLOAD_BYTES = 500 * 1024 * 1024 // 500 MB

const formatMB = (bytes: number) => `${(bytes / (1024 * 1024)).toFixed(0)} MB`

/** Map quota/rate-limit API errors to friendly copy instead of the raw message. */
function friendlyError(e: unknown): string {
  if (e instanceof ApiError) {
    if (e.status === 402) return 'Your analysis limit has been reached — upgrade your plan to continue.'
    if (e.status === 429) return 'Too many requests — wait a moment and try again.'
  }
  return e instanceof Error ? e.message : String(e)
}

interface FileEntry {
  file: File
  fileType: 'ODB_PLUS_PLUS'
  status: 'pending' | 'uploading' | 'uploaded' | 'failed'
  progress: number
}

export default function UploadPage() {
  return (
    <Suspense fallback={<div className="flex items-center justify-center min-h-screen">Loading...</div>}>
      <UploadPageInner />
    </Suspense>
  )
}

function UploadPageInner() {
  const router = useRouter()
  const searchParams = useSearchParams()
  const { usage } = useUsage()
  const projectId = searchParams.get('projectId') || undefined
  // The dashboard's "Analyze" button links here with ?submissionId=… — the
  // file is already in S3, so we resume at the analyze step, never re-upload.
  const resumeId = searchParams.get('submissionId')
  const backHref = projectId ? `/projects/${projectId}` : '/dashboard'
  const backLabel = projectId ? 'Project' : 'Dashboard'
  // Single-file state (existing flow)
  const [file, setFile] = useState<File | null>(null)
  const fileType = 'ODB_PLUS_PLUS' as const
  // Multi-file state (batch flow)
  const [files, setFiles] = useState<FileEntry[]>([])
  const isBatch = files.length > 1

  const [profiles, setProfiles] = useState<CapabilityProfile[]>([])
  const [profileId, setProfileId] = useState<string>('')
  const [step, setStep] = useState<Step>('select')
  const [progress, setProgress] = useState(0)
  const [job, setJob] = useState<AnalysisJob | null>(null)
  const [errorMsg, setErrorMsg] = useState<string>('')
  const [batchId, setBatchId] = useState<string | null>(null)
  // Non-CUI alpha guardrail: uploads are blocked until the user affirms the
  // design is not ITAR-controlled or CUI / export-controlled technical data.
  const [nonCuiAck, setNonCuiAck] = useState(false)
  // Inline messages on the select step (size rejections, batch-cap truncation).
  const [selectError, setSelectError] = useState<string>('')
  const [capWarning, setCapWarning] = useState<string>('')
  // Submission that uploaded successfully but whose analysis didn't finish —
  // lets the error screen offer "Retry analysis" without re-uploading.
  const [uploadedSubmissionId, setUploadedSubmissionId] = useState<string | null>(null)
  // Already-uploaded submission being resumed via ?submissionId= (the 'resume' step).
  const [resumeSubmission, setResumeSubmission] = useState<Submission | null>(null)
  // Filenames that failed to upload in a batch (drives the 'partial' step).
  const [failedUploads, setFailedUploads] = useState<string[]>([])
  // Final batch state so the done screen can report partial results honestly.
  const [finalBatch, setFinalBatch] = useState<BatchDetail | null>(null)
  const [submitting, setSubmitting] = useState(false)
  const inFlightRef = useRef(false)

  useEffect(() => {
    if (!isLoggedIn()) { router.replace('/login'); return }
    if (!canWrite()) { router.replace('/dashboard'); return }
    getProfiles().then((ps) => {
      setProfiles(ps ?? [])
      const def = ps?.find((p) => p.isDefault)
      if (def) setProfileId(def.id)
    }).catch(() => {})
  }, [router])

  // Resume an existing submission (?submissionId=…): the file is already in
  // S3, so jump straight to the analyze step instead of the file picker.
  useEffect(() => {
    if (!resumeId) return
    setStep('resume')
    getSubmission(resumeId)
      .then((sub) => {
        if (sub.status === 'ANALYZING' || sub.status === 'DONE') {
          // Already running or finished — the results page shows progress.
          router.replace(sub.latestJobId ? `/results/${sub.latestJobId}` : '/dashboard')
          return
        }
        // UPLOADED or FAILED: offer to (re-)run analysis without re-uploading.
        setResumeSubmission(sub)
        setUploadedSubmissionId(sub.id)
      })
      .catch((e: unknown) => {
        setSelectError(
          e instanceof ApiError && e.status === 404
            ? "Couldn't find that submission — it may have been deleted. Upload the file again to analyze it."
            : `Couldn't load the submission — ${friendlyError(e)}`
        )
        setStep('select')
      })
  }, [resumeId, router])

  // Warn before closing the tab while an upload/analysis is in flight.
  useEffect(() => {
    if (step !== 'packaging' && step !== 'uploading' && step !== 'analyzing') return
    const handler = (e: BeforeUnloadEvent) => {
      e.preventDefault()
      e.returnValue = ''
    }
    window.addEventListener('beforeunload', handler)
    return () => window.removeEventListener('beforeunload', handler)
  }, [step])

  const batchAllowed = usage?.features.batchUpload !== false
  const maxBatchFiles = usage?.features.maxBatchFiles ?? 50

  /** Re-entry guard shared by every submit-ish action (double-click protection). */
  const guard = async (fn: () => Promise<void>) => {
    if (inFlightRef.current) return
    inFlightRef.current = true
    setSubmitting(true)
    try {
      await fn()
    } finally {
      inFlightRef.current = false
      setSubmitting(false)
    }
  }

  /** Shared selection logic for the file input and drag-and-drop (size + cap checks). */
  const applySelection = (selected: File[]) => {
    setSelectError('')
    setCapWarning('')

    if (selected.length === 1 || !batchAllowed) {
      const f = selected[0]
      if (f.size > MAX_UPLOAD_BYTES) {
        setSelectError(`"${f.name}" is ${formatMB(f.size)} — the maximum upload size is 500 MB. Compress the archive or remove unneeded data and try again.`)
        return
      }
      setFile(f)
      setFiles([])
      return
    }

    setFile(null)
    const capped = selected.slice(0, maxBatchFiles)
    const totalBytes = capped.reduce((sum, f) => sum + f.size, 0)
    if (totalBytes > MAX_UPLOAD_BYTES) {
      setSelectError(`Selected files total ${formatMB(totalBytes)} — the maximum per batch is 500 MB. Remove some files and try again.`)
      return
    }
    if (selected.length > maxBatchFiles) {
      setCapWarning(`Selected ${selected.length} files; only ${maxBatchFiles} allowed per batch — proceeding with the first ${maxBatchFiles}.`)
    }
    const entries: FileEntry[] = capped.map((f) => ({
      file: f,
      fileType: 'ODB_PLUS_PLUS' as const,
      status: 'pending' as const,
      progress: 0,
    }))
    setFiles(entries)
  }

  const handleFilesSelected = (e: React.ChangeEvent<HTMLInputElement>) => {
    const selected = e.target.files
    if (!selected || selected.length === 0) return
    applySelection(Array.from(selected))
  }

  const removeFile = (index: number) => {
    setFiles((prev) => prev.filter((_, i) => i !== index))
  }

  const clearAllFiles = () => {
    setFile(null)
    setFiles([])
    setSelectError('')
    setCapWarning('')
  }

  const [dragOver, setDragOver] = useState(false)

  const handleDrop = async (e: React.DragEvent) => {
    e.preventDefault()
    setDragOver(false)

    // Check if a folder was dropped (drag-and-drop directory API).
    const items = e.dataTransfer.items
    if (items && items.length > 0) {
      const firstEntry = items[0].webkitGetAsEntry?.()
      if (firstEntry?.isDirectory) {
        // Package the dropped folder into a tar archive.
        setSelectError('')
        setCapWarning('')
        setStep('packaging')
        try {
          const fileList = await readDirectoryEntry(firstEntry as FileSystemDirectoryEntry)
          const tarBlob = await packageFilesAsTar(fileList)
          const tarFile = new window.File([tarBlob], `${firstEntry.name}.tar`, { type: 'application/x-tar' })
          if (tarFile.size > MAX_UPLOAD_BYTES) {
            throw new Error('Folder exceeds 500 MB — compress it and upload as an archive instead.')
          }
          setFile(tarFile)
          setFiles([])
        } catch (err) {
          // Packaging problems (too big, unreadable, bad paths) are selection
          // problems — surface them inline and let the user pick again.
          setSelectError(err instanceof Error ? err.message : 'Failed to package folder')
        }
        setStep('select')
        return
      }
    }

    const dropped = e.dataTransfer.files
    if (!dropped || dropped.length === 0) return
    applySelection(Array.from(dropped))
  }

  // Handle folder selection via <input webkitdirectory>
  const handleFolderSelected = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const selected = e.target.files
    if (!selected || selected.length === 0) return
    setSelectError('')
    setCapWarning('')
    setStep('packaging')
    try {
      const tarBlob = await packageFilesAsTar(selected)
      // Use the common folder prefix as the archive name.
      const firstPath = (selected[0] as File & { webkitRelativePath?: string }).webkitRelativePath || ''
      const folderName = firstPath.split('/')[0] || 'upload'
      const tarFile = new window.File([tarBlob], `${folderName}.tar`, { type: 'application/x-tar' })
      if (tarFile.size > MAX_UPLOAD_BYTES) {
        throw new Error('Folder exceeds 500 MB — compress it and upload as an archive instead.')
      }
      setFile(tarFile)
      setFiles([])
    } catch (err) {
      setSelectError(err instanceof Error ? err.message : 'Failed to package folder')
    }
    setStep('select')
  }

  /** Kick off analysis for a submission and poll it to a terminal state. */
  const runAnalysis = async (submissionId: string) => {
    const newJob = await startAnalysis(submissionId, profileId || undefined)
    track('Analysis Requested', { submissionId, profileId })
    setJob(newJob)

    const jobData = await pollJobUntilDone(newJob.id, { onUpdate: setJob })
    if (jobData.status === 'DONE') {
      setStep('done')
    } else {
      throw new Error(jobData.errorMsg || 'Analysis failed')
    }
  }

  // Single-file upload (existing flow)
  const handleSingleUpload = async () => {
    if (!file) return
    if (file.size > MAX_UPLOAD_BYTES) {
      setSelectError(`"${file.name}" is ${formatMB(file.size)} — the maximum upload size is 500 MB.`)
      return
    }
    setStep('uploading')
    setErrorMsg('')
    setProgress(0)
    setUploadedSubmissionId(null)
    try {
      const { submissionId, presignedUrl } = await createSubmission(file.name, fileType, projectId, nonCuiAck)
      await uploadToS3(presignedUrl, file, setProgress)
      // The file is in S3 now — remember the submission so an analysis
      // failure can be retried without re-uploading.
      setUploadedSubmissionId(submissionId)
      track('Submission Created', { fileType, projectId })
      setStep('analyzing')
      await runAnalysis(submissionId)
    } catch (e: unknown) {
      if (e instanceof PollTimeoutError) {
        // Not a failure — the job is still running in the background.
        setStep('timeout')
        return
      }
      setErrorMsg(friendlyError(e))
      setStep('error')
    }
  }

  /** Retry analysis on the already-uploaded submission (no re-upload). */
  const handleRetryAnalysis = async () => {
    if (!uploadedSubmissionId) return
    setStep('analyzing')
    setErrorMsg('')
    try {
      await runAnalysis(uploadedSubmissionId)
    } catch (e: unknown) {
      if (e instanceof PollTimeoutError) {
        setStep('timeout')
        return
      }
      setErrorMsg(friendlyError(e))
      setStep('error')
    }
  }

  /** Start batch analysis and poll it to a terminal state. */
  const runBatchAnalysis = async (id: string) => {
    setStep('analyzing')
    await analyzeBatch(id, profileId || undefined)
    const finalData = await pollBatchUntilDone(id)
    setFinalBatch(finalData)
    setStep('done')
  }

  // Batch upload
  const handleBatchUpload = async () => {
    if (files.length === 0) return
    setStep('uploading')
    setErrorMsg('')
    setFailedUploads([])
    setFinalBatch(null)
    track('Batch Started', { fileCount: files.length, projectId })
    try {
      // 1. Create batch and get presigned URLs
      const batchFiles = files.map((f) => ({
        filename: f.file.name,
        fileType: f.fileType,
      }))
      const batchResp = await createBatch(batchFiles, undefined, profileId || undefined, nonCuiAck)
      setBatchId(batchResp.batchId)

      // 2. Upload all files in parallel, collecting failures
      const failures: string[] = []
      const uploadPromises = batchResp.submissions.map((sub, index) => {
        const fileEntry = files[index]
        setFiles((prev) =>
          prev.map((f, i) => (i === index ? { ...f, status: 'uploading' } : f))
        )
        return uploadToS3(sub.presignedUrl, fileEntry.file, (pct) => {
          setFiles((prev) =>
            prev.map((f, i) => (i === index ? { ...f, progress: pct } : f))
          )
        })
          .then(() => {
            setFiles((prev) =>
              prev.map((f, i) => (i === index ? { ...f, status: 'uploaded', progress: 100 } : f))
            )
          })
          .catch(() => {
            failures.push(fileEntry.file.name)
            setFiles((prev) =>
              prev.map((f, i) => (i === index ? { ...f, status: 'failed' } : f))
            )
          })
      })

      await Promise.allSettled(uploadPromises)

      // Update overall progress
      setProgress(100)

      // 3. Don't proceed silently past upload failures — let the user decide.
      if (failures.length >= files.length) {
        throw new Error(`None of the ${files.length} files could be uploaded. Check your connection and try again.`)
      }
      if (failures.length > 0) {
        setFailedUploads(failures)
        setStep('partial')
        return
      }

      // 4. Start batch analysis and poll until terminal (with timeout)
      await runBatchAnalysis(batchResp.batchId)
    } catch (e: unknown) {
      if (e instanceof PollTimeoutError) {
        setStep('timeout')
        return
      }
      setErrorMsg(friendlyError(e))
      setStep('error')
    }
  }

  /** Proceed with the successfully uploaded files after a partial batch failure. */
  const continueBatchAfterPartial = async () => {
    if (!batchId) return
    try {
      await runBatchAnalysis(batchId)
    } catch (e: unknown) {
      if (e instanceof PollTimeoutError) {
        setStep('timeout')
        return
      }
      setErrorMsg(friendlyError(e))
      setStep('error')
    }
  }

  /** Cancel out of a partial batch: back to selection with statuses reset. */
  const cancelPartialBatch = () => {
    setFiles((prev) => prev.map((f) => ({ ...f, status: 'pending' as const, progress: 0 })))
    setFailedUploads([])
    setBatchId(null)
    setStep('select')
  }

  const resetToSelect = () => {
    clearAllFiles()
    setErrorMsg('')
    setJob(null)
    setBatchId(null)
    setUploadedSubmissionId(null)
    setResumeSubmission(null)
    setFailedUploads([])
    setFinalBatch(null)
    setStep('select')
  }

  const handleUpload = () => {
    guard(isBatch ? handleBatchUpload : handleSingleUpload)
  }

  const stepForIndicator =
    step === 'error' ? 'done' :
    step === 'timeout' ? 'analyzing' :
    step === 'partial' ? 'uploading' :
    step === 'resume' ? 'select' :
    step
  const stepIndex = STEPS.indexOf(stepForIndicator)
  const hasFiles = file !== null || files.length > 0
  const uploadedCount = files.length - failedUploads.length

  // Compute aggregate upload progress for batch
  const batchProgress = files.length > 0
    ? Math.round(files.reduce((sum, f) => sum + f.progress, 0) / files.length)
    : progress

  return (
    <div className="min-h-screen bg-background">
      <header className="bg-card border-b px-6 py-5 flex items-center gap-4">
        <RapidDFMLogo className="shrink-0" />
        <h1 className="text-xl font-semibold text-foreground truncate">Upload & Analyze</h1>
        <AppTaskbar expandOnHover={false} className="w-auto ml-auto shrink-0" />
      </header>

      <main className="max-w-2xl mx-auto px-6 py-10">
        <div className="mb-6">
          <AppBackButton href={backHref} label={backLabel} />
        </div>

        {/* Step indicator */}
        <div className="flex items-center justify-center mb-10">
          {['Select File', 'Uploading', 'Analyzing', 'Done'].map((label, i) => (
            <div key={label} className="flex items-center">
              <div className={cn(
                'flex items-center justify-center w-8 h-8 rounded-full text-xs font-bold border-2 transition-colors',
                i < stepIndex ? 'bg-green-500 border-green-500 text-white' :
                i === stepIndex ? 'bg-blue-600 border-blue-600 text-white' :
                'bg-card border-border text-muted-foreground'
              )}>
                {i < stepIndex ? '✓' : i + 1}
              </div>
              <span className={cn(
                'ml-2 text-xs font-medium',
                i <= stepIndex ? 'text-foreground' : 'text-muted-foreground'
              )}>{label}</span>
              {i < 3 && <div className={cn('w-12 h-0.5 mx-3', i < stepIndex ? 'bg-green-400' : 'bg-muted')} />}
            </div>
          ))}
        </div>

        {/* Step: resume an already-uploaded submission (no re-upload) */}
        {step === 'resume' && (
          !resumeSubmission ? (
            <div className="flex flex-col items-center justify-center gap-3 py-12">
              <div className="animate-spin h-8 w-8 border-4 border-primary border-t-transparent rounded-full" />
              <p className="text-sm font-medium text-foreground">Loading submission...</p>
            </div>
          ) : (
            <div className="space-y-6">
              <div className="flex items-center gap-3 px-4 py-4 border border-border rounded-lg bg-muted/40">
                <File className="h-8 w-8 text-primary flex-shrink-0" />
                <div className="flex-1 min-w-0">
                  <p className="text-sm font-medium text-foreground truncate">{resumeSubmission.filename}</p>
                  <p className="text-xs text-muted-foreground">
                    {resumeSubmission.status === 'FAILED'
                      ? 'Previous analysis failed — you can run it again.'
                      : 'Already uploaded — ready to analyze.'}
                  </p>
                </div>
                <CheckCircle className="h-5 w-5 text-green-500 flex-shrink-0" />
              </div>

              <div>
                <label className="block text-sm font-medium text-foreground mb-1">Capability Profile</label>
                <select
                  value={profileId}
                  onChange={(e) => setProfileId(e.target.value)}
                  className="w-full border border-input bg-background rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-ring"
                >
                  <option value="">Default</option>
                  {profiles.map((p) => (
                    <option key={p.id} value={p.id}>{p.name}{p.isDefault ? ' (default)' : ''}</option>
                  ))}
                </select>
              </div>

              <Button onClick={() => guard(handleRetryAnalysis)} disabled={submitting} className="w-full">
                {resumeSubmission.status === 'FAILED' ? 'Re-run Analysis' : 'Start Analysis'}
              </Button>
              <Button onClick={resetToSelect} variant="ghost" className="w-full">
                Upload a different file instead
              </Button>
            </div>
          )
        )}

        {/* Step: packaging folder into tar */}
        {step === 'packaging' && (
          <div className="flex flex-col items-center justify-center gap-3 py-12">
            <div className="animate-spin h-8 w-8 border-4 border-primary border-t-transparent rounded-full" />
            <p className="text-sm font-medium text-foreground">Packaging folder...</p>
          </div>
        )}

        {/* Step: select */}
        {step === 'select' && (
          <div className="space-y-6">
            {/* Multi-file drop zone */}
            <div
              className={cn(
                'relative flex flex-col items-center justify-center w-full h-48 border-2 border-dashed rounded-lg cursor-pointer transition-colors',
                dragOver ? 'border-primary bg-primary/5' : 'border-border hover:border-ring bg-muted/40'
              )}
              onDragOver={(e) => { e.preventDefault(); setDragOver(true) }}
              onDragLeave={() => setDragOver(false)}
              onDrop={handleDrop}
            >
              <input
                type="file"
                accept=".zip,.tar,.tgz,.tar.gz"
                multiple={batchAllowed}
                onChange={handleFilesSelected}
                className="absolute inset-0 w-full h-full opacity-0 cursor-pointer"
              />
              {!hasFiles ? (
                <div className="flex flex-col items-center gap-2 text-center px-4">
                  <Upload className="h-10 w-10 text-muted-foreground" />
                  <div>
                    <p className="text-sm font-medium text-foreground">Drop your ODB++ archive or folder here</p>
                    <p className="text-xs text-muted-foreground mt-1">or click to browse &mdash; .zip, .tar, .tgz accepted. You can also drop a folder directly.</p>
                  </div>
                </div>
              ) : file && !isBatch ? (
                <div className="flex items-center gap-3 px-4">
                  <File className="h-8 w-8 text-primary flex-shrink-0" />
                  <div className="flex-1 min-w-0">
                    <p className="text-sm font-medium text-foreground truncate">{file.name}</p>
                    <p className="text-xs text-muted-foreground">{(file.size / 1024).toFixed(1)} KB</p>
                  </div>
                  <button onClick={(e) => { e.stopPropagation(); clearAllFiles() }} className="p-1 rounded hover:bg-muted text-muted-foreground" type="button">
                    <X className="h-4 w-4" />
                  </button>
                </div>
              ) : (
                <div className="flex flex-col items-center gap-1 px-4">
                  <File className="h-8 w-8 text-primary" />
                  <p className="text-sm font-medium text-foreground">{files.length} files selected (batch upload)</p>
                  <button onClick={(e) => { e.stopPropagation(); clearAllFiles() }} className="text-xs text-muted-foreground hover:text-foreground underline" type="button">
                    Clear all
                  </button>
                </div>
              )}
            </div>

            {/* Inline selection errors / warnings */}
            {selectError && (
              <div className="p-3 rounded-lg border border-red-500/30 bg-red-500/10 flex items-start gap-2">
                <XCircle className="h-4 w-4 text-red-500 mt-0.5 shrink-0" />
                <p className="text-xs font-medium text-red-700 dark:text-red-400">{selectError}</p>
              </div>
            )}
            {capWarning && (
              <div className="p-3 rounded-lg border border-yellow-500/30 bg-yellow-500/10 flex items-start gap-2">
                <AlertTriangle className="h-4 w-4 text-yellow-600 dark:text-yellow-400 mt-0.5 shrink-0" />
                <p className="text-xs font-medium text-yellow-700 dark:text-yellow-400">{capWarning}</p>
              </div>
            )}

            {/* Batch file list */}
            {isBatch && (
              <div className="border border-border rounded-lg divide-y divide-border max-h-64 overflow-y-auto">
                {files.map((entry, i) => (
                  <div key={i} className="flex items-center gap-3 px-4 py-2">
                    <File className="h-4 w-4 text-muted-foreground flex-shrink-0" />
                    <span className="flex-1 text-sm truncate">{entry.file.name}</span>
                    <span className="text-xs text-muted-foreground">{(entry.file.size / 1024).toFixed(0)} KB</span>
                    <span className="text-xs text-muted-foreground">ODB++</span>
                    <button onClick={() => removeFile(i)} className="p-0.5 rounded hover:bg-muted text-muted-foreground" type="button">
                      <X className="h-3.5 w-3.5" />
                    </button>
                  </div>
                ))}
              </div>
            )}

            <div className="grid grid-cols-2 gap-4">
              {!isBatch && (
                <div>
                  <label className="block text-sm font-medium text-foreground mb-1">Select Folder</label>
                  <label className="flex items-center gap-2 w-full border border-input bg-background rounded-md px-3 py-2 text-sm cursor-pointer hover:bg-muted/50 transition-colors">
                    <FolderOpen className="h-4 w-4 text-muted-foreground" />
                    <span className="text-muted-foreground">Choose ODB++ folder...</span>
                    <input
                      type="file"
                      // @ts-expect-error -- webkitdirectory is non-standard but widely supported
                      webkitdirectory=""
                      onChange={handleFolderSelected}
                      className="hidden"
                    />
                  </label>
                </div>
              )}

              <div className={isBatch ? 'col-span-2' : ''}>
                <label className="block text-sm font-medium text-foreground mb-1">Capability Profile</label>
                <select
                  value={profileId}
                  onChange={(e) => setProfileId(e.target.value)}
                  className="w-full border border-input bg-background rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-ring"
                >
                  <option value="">Default</option>
                  {profiles.map((p) => (
                    <option key={p.id} value={p.id}>{p.name}{p.isDefault ? ' (default)' : ''}</option>
                  ))}
                </select>
              </div>
            </div>

            {/* Non-CUI alpha guardrail */}
            <div className="rounded-lg border border-amber-500/40 bg-amber-500/10 p-4 space-y-3">
              <p className="text-xs font-medium text-amber-800 dark:text-amber-300">
                Alpha access: this environment is not authorized for export-controlled data.
                Do not upload ITAR-controlled or CUI / export-controlled technical data.
              </p>
              <label className="flex items-start gap-2 cursor-pointer">
                <input
                  type="checkbox"
                  checked={nonCuiAck}
                  onChange={(e) => setNonCuiAck(e.target.checked)}
                  className="mt-0.5 h-4 w-4 shrink-0 accent-amber-600"
                />
                <span className="text-xs text-foreground">
                  I confirm this design is not ITAR-controlled or CUI / export-controlled technical data.
                </span>
              </label>
            </div>

            <Button onClick={handleUpload} disabled={!hasFiles || !nonCuiAck || submitting} className="w-full">
              {isBatch ? `Upload & Analyze ${files.length} Files` : 'Upload & Analyze'}
            </Button>

            {usage && (
              <div className="space-y-2">
                <p className="text-xs text-muted-foreground text-center">
                  {usage.analyses.used} of {usage.analyses.limit === -1 ? 'unlimited' : usage.analyses.limit} analyses used this period
                </p>
                {usage.analyses.overage > 0 && (
                  <div className="p-3 rounded-lg border border-yellow-500/30 bg-yellow-500/10">
                    <p className="text-xs font-medium text-yellow-700 dark:text-yellow-400 text-center">
                      You&apos;ve exceeded your included analyses. Additional analyses are $2 each.
                    </p>
                  </div>
                )}
              </div>
            )}
          </div>
        )}

        {/* Step: uploading */}
        {step === 'uploading' && (
          <div className="text-center space-y-4">
            <div className="animate-spin h-10 w-10 border-4 border-blue-600 border-t-transparent rounded-full mx-auto" />
            <p className="font-medium text-foreground">
              {isBatch ? `Processing ${files.length} files...` : 'Processing File...'}
            </p>
            {isBatch ? (
              <div className="space-y-2">
                {files.map((entry, i) => (
                  <div key={i} className="flex items-center gap-2 text-sm">
                    <span className="truncate flex-1 text-left">{entry.file.name}</span>
                    <div className="w-24 bg-muted rounded-full h-1.5">
                      <div
                        className={cn(
                          'h-1.5 rounded-full transition-all',
                          entry.status === 'failed' ? 'bg-red-500' : 'bg-blue-600'
                        )}
                        style={{ width: `${entry.progress}%` }}
                      />
                    </div>
                    <span className="text-xs text-muted-foreground w-8 text-right">
                      {entry.status === 'uploaded' ? '✓' : entry.status === 'failed' ? '✗' : `${entry.progress}%`}
                    </span>
                  </div>
                ))}
                <p className="text-sm text-muted-foreground">Overall: {batchProgress}%</p>
              </div>
            ) : (
              <>
                <div className="w-full bg-muted rounded-full h-2">
                  <div className="bg-blue-600 h-2 rounded-full transition-all" style={{ width: `${progress}%` }} />
                </div>
                <p className="text-sm text-muted-foreground">{progress}%</p>
              </>
            )}
          </div>
        )}

        {/* Step: analyzing */}
        {step === 'analyzing' && (
          <div className="text-center space-y-4">
            <div className="animate-spin h-10 w-10 border-4 border-purple-600 border-t-transparent rounded-full mx-auto" />
            <p className="font-medium text-foreground">
              {isBatch ? `Running DFM analysis on ${files.length} files...` : 'Running DFM analysis...'}
            </p>
            {!isBatch && (
              <p className="text-sm text-muted-foreground">
                Status: <span className="font-mono">{job?.status ?? 'PENDING'}</span>
              </p>
            )}
            <p className="text-xs text-muted-foreground">This typically takes 10-60 seconds per file</p>
          </div>
        )}

        {/* Step: partial batch upload failure — let the user decide */}
        {step === 'partial' && (
          <div className="text-center space-y-4">
            <AlertTriangle className="h-14 w-14 text-yellow-500 mx-auto" />
            <h2 className="text-xl font-bold text-foreground">Some files failed to upload</h2>
            <p className="text-sm text-muted-foreground">
              {uploadedCount} of {files.length} files uploaded successfully. These did not:
            </p>
            <ul className="text-sm text-left border border-border rounded-lg divide-y divide-border max-h-40 overflow-y-auto">
              {failedUploads.map((name, i) => (
                <li key={`${name}-${i}`} className="flex items-center gap-2 px-4 py-2">
                  <XCircle className="h-4 w-4 text-red-500 shrink-0" />
                  <span className="truncate text-foreground">{name}</span>
                </li>
              ))}
            </ul>
            <Button onClick={() => guard(continueBatchAfterPartial)} disabled={submitting} className="w-full">
              Analyze the {uploadedCount} uploaded {uploadedCount === 1 ? 'file' : 'files'}
            </Button>
            <Button onClick={cancelPartialBatch} variant="outline" className="w-full">
              Cancel and go back
            </Button>
          </div>
        )}

        {/* Step: poll timed out — the job is still running, not failed */}
        {step === 'timeout' && (
          <div className="text-center space-y-4">
            <Clock className="h-14 w-14 text-blue-500 mx-auto" />
            <h2 className="text-xl font-bold text-foreground">Analysis is taking longer than expected</h2>
            <p className="text-sm text-muted-foreground">
              It&apos;s still running in the background &mdash; you can safely leave this page and check its progress from the dashboard.
            </p>
            <Button onClick={() => router.push('/dashboard')} className="w-full">
              Go to Dashboard
            </Button>
            {isBatch && batchId && (
              <Button onClick={() => router.push(`/batches/${batchId}`)} variant="outline" className="w-full">
                View Batch Progress
              </Button>
            )}
          </div>
        )}

        {/* Step: done */}
        {step === 'done' && (
          <div className="text-center space-y-4">
            <CheckCircle className="h-14 w-14 text-green-500 mx-auto" />
            <h2 className="text-xl font-bold text-foreground">Analysis Complete</h2>
            <p className="text-muted-foreground">
              {isBatch
                ? finalBatch && finalBatch.batch.failed > 0
                  ? `${finalBatch.batch.completed} of ${finalBatch.batch.total} boards analyzed successfully — ${finalBatch.batch.failed} failed. See the batch page for details.`
                  : failedUploads.length > 0
                    ? `${uploadedCount} of ${files.length} boards have been analyzed (${failedUploads.length} failed to upload).`
                    : `All ${files.length} boards have been analyzed.`
                : 'Your board has been analyzed successfully.'}
            </p>
            {isBatch && batchId ? (
              <Button onClick={() => router.push(`/batches/${batchId}`)} className="w-full">
                View Batch Results
              </Button>
            ) : job ? (
              <Button onClick={() => router.push(`/results/${job.id}`)} className="w-full">
                View Results
              </Button>
            ) : null}
            <Link href="/dashboard" className="block">
              <Button variant="ghost" className="w-full">Back to Dashboard</Button>
            </Link>
          </div>
        )}

        {/* Step: error */}
        {step === 'error' && (
          <div className="text-center space-y-4">
            <XCircle className="h-14 w-14 text-red-500 mx-auto" />
            <h2 className="text-xl font-bold text-foreground">Something went wrong</h2>
            <p className="text-sm text-muted-foreground">{errorMsg}</p>
            {uploadedSubmissionId && !isBatch && (
              <>
                <Button onClick={() => guard(handleRetryAnalysis)} disabled={submitting} className="w-full">
                  Retry analysis
                </Button>
                <p className="text-xs text-muted-foreground">
                  Your file was uploaded successfully &mdash; retrying only re-runs the analysis.
                </p>
              </>
            )}
            <Button onClick={resetToSelect} variant="outline" className="w-full">Try again</Button>
          </div>
        )}
      </main>
    </div>
  )
}
