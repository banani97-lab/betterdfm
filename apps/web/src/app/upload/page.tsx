'use client'

import { Suspense, useEffect, useState } from 'react'
import { useRouter, useSearchParams } from 'next/navigation'
import Link from 'next/link'
import { CheckCircle, XCircle, Upload, File, X } from 'lucide-react'
import {
  createSubmission,
  uploadToS3,
  startAnalysis,
  getJob,
  getProfiles,
  createBatch,
  analyzeBatch,
  getBatch,
  type CapabilityProfile,
  type AnalysisJob,
} from '@/lib/api'
import { isLoggedIn, canWrite } from '@/lib/auth'
import { FileUploader } from '@/components/ui/FileUploader'
import { Button } from '@/components/ui/button'
import { BetterDFMLogo } from '@/components/ui/betterdfm-logo'
import { cn } from '@/lib/utils'

type Step = 'select' | 'uploading' | 'analyzing' | 'done' | 'error'

const STEPS = ['select', 'uploading', 'analyzing', 'done']

interface FileEntry {
  file: File
  fileType: 'GERBER' | 'ODB_PLUS_PLUS'
  status: 'pending' | 'uploading' | 'uploaded' | 'failed'
  progress: number
}

function inferFileType(filename: string): 'GERBER' | 'ODB_PLUS_PLUS' {
  const lower = filename.toLowerCase()
  if (lower.endsWith('.tgz') || lower.endsWith('.tar') || lower.endsWith('.tar.gz') || lower.includes('odb')) return 'ODB_PLUS_PLUS'
  return 'GERBER'
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
  const projectId = searchParams.get('projectId') || undefined
  // Single-file state (existing flow)
  const [file, setFile] = useState<File | null>(null)
  const [fileType, setFileType] = useState<'GERBER' | 'ODB_PLUS_PLUS'>('GERBER')
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

  useEffect(() => {
    if (!isLoggedIn()) { router.replace('/login'); return }
    if (!canWrite()) { router.replace('/dashboard'); return }
    getProfiles().then((ps) => {
      setProfiles(ps ?? [])
      const def = ps?.find((p) => p.isDefault)
      if (def) setProfileId(def.id)
    }).catch(() => {})
  }, [router])

  const handleFilesSelected = (e: React.ChangeEvent<HTMLInputElement>) => {
    const selected = e.target.files
    if (!selected || selected.length === 0) return

    if (selected.length === 1) {
      // Single file -- use existing flow
      setFile(selected[0])
      setFiles([])
      return
    }

    // Multiple files -- batch flow
    setFile(null)
    const entries: FileEntry[] = Array.from(selected).map((f) => ({
      file: f,
      fileType: inferFileType(f.name),
      status: 'pending' as const,
      progress: 0,
    }))
    setFiles(entries)
  }

  const removeFile = (index: number) => {
    setFiles((prev) => prev.filter((_, i) => i !== index))
  }

  const clearAllFiles = () => {
    setFile(null)
    setFiles([])
  }

  // Single-file upload (existing flow)
  const handleSingleUpload = async () => {
    if (!file) return
    setStep('uploading')
    setErrorMsg('')
    try {
      const { submissionId, presignedUrl } = await createSubmission(file.name, fileType, projectId)
      await uploadToS3(presignedUrl, file, setProgress)
      setStep('analyzing')
      const newJob = await startAnalysis(submissionId, profileId || undefined)
      setJob(newJob)

      let jobData = newJob
      while (jobData.status === 'PENDING' || jobData.status === 'PROCESSING') {
        await new Promise((r) => setTimeout(r, 3000))
        jobData = await getJob(newJob.id)
        setJob(jobData)
      }
      if (jobData.status === 'DONE') {
        setStep('done')
      } else {
        throw new Error(jobData.errorMsg || 'Analysis failed')
      }
    } catch (e: unknown) {
      setErrorMsg(e instanceof Error ? e.message : String(e))
      setStep('error')
    }
  }

  // Batch upload
  const handleBatchUpload = async () => {
    if (files.length === 0) return
    setStep('uploading')
    setErrorMsg('')
    try {
      // 1. Create batch and get presigned URLs
      const batchFiles = files.map((f) => ({
        filename: f.file.name,
        fileType: f.fileType,
      }))
      const batchResp = await createBatch(batchFiles, undefined, profileId || undefined)
      setBatchId(batchResp.batchId)

      // 2. Upload all files in parallel
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
            setFiles((prev) =>
              prev.map((f, i) => (i === index ? { ...f, status: 'failed' } : f))
            )
          })
      })

      await Promise.allSettled(uploadPromises)

      // Update overall progress
      setProgress(100)

      // 3. Start batch analysis
      setStep('analyzing')
      await analyzeBatch(batchResp.batchId, profileId || undefined)

      // 4. Poll batch status
      let batchData = await getBatch(batchResp.batchId)
      while (batchData.batch.status === 'PENDING' || batchData.batch.status === 'PROCESSING') {
        await new Promise((r) => setTimeout(r, 3000))
        batchData = await getBatch(batchResp.batchId)
      }

      setStep('done')
    } catch (e: unknown) {
      setErrorMsg(e instanceof Error ? e.message : String(e))
      setStep('error')
    }
  }

  const handleUpload = () => {
    if (isBatch) {
      handleBatchUpload()
    } else {
      handleSingleUpload()
    }
  }

  const stepIndex = STEPS.indexOf(step === 'error' ? 'done' : step)
  const hasFiles = file !== null || files.length > 0

  // Compute aggregate upload progress for batch
  const batchProgress = files.length > 0
    ? Math.round(files.reduce((sum, f) => sum + f.progress, 0) / files.length)
    : progress

  return (
    <div className="min-h-screen bg-background">
      <header className="bg-card border-b px-6 py-5 flex items-center justify-between gap-4">
        <BetterDFMLogo />
        <h1 className="text-xl font-semibold text-foreground">Upload & Analyze</h1>
      </header>

      <main className="max-w-2xl mx-auto px-6 py-10">
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
                {i < stepIndex ? '\u2713' : i + 1}
              </div>
              <span className={cn(
                'ml-2 text-xs font-medium',
                i <= stepIndex ? 'text-foreground' : 'text-muted-foreground'
              )}>{label}</span>
              {i < 3 && <div className={cn('w-12 h-0.5 mx-3', i < stepIndex ? 'bg-green-400' : 'bg-muted')} />}
            </div>
          ))}
        </div>

        {/* Step: select */}
        {step === 'select' && (
          <div className="space-y-6">
            {/* Multi-file drop zone */}
            <div
              className={cn(
                'relative flex flex-col items-center justify-center w-full h-48 border-2 border-dashed rounded-lg cursor-pointer transition-colors',
                'border-border hover:border-ring bg-muted/40'
              )}
            >
              <input
                type="file"
                accept=".zip,.gbr,.ger,.drl,.exc"
                multiple
                onChange={handleFilesSelected}
                className="absolute inset-0 w-full h-full opacity-0 cursor-pointer"
              />
              {!hasFiles ? (
                <div className="flex flex-col items-center gap-2 text-center px-4">
                  <Upload className="h-10 w-10 text-muted-foreground" />
                  <div>
                    <p className="text-sm font-medium text-foreground">Drop your Gerber or ODB++ files here</p>
                    <p className="text-xs text-muted-foreground mt-1">or click to browse -- .zip, .gbr, .ger accepted. Select multiple files for batch upload.</p>
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

            {/* Batch file list */}
            {isBatch && (
              <div className="border border-border rounded-lg divide-y divide-border max-h-64 overflow-y-auto">
                {files.map((entry, i) => (
                  <div key={i} className="flex items-center gap-3 px-4 py-2">
                    <File className="h-4 w-4 text-muted-foreground flex-shrink-0" />
                    <span className="flex-1 text-sm truncate">{entry.file.name}</span>
                    <span className="text-xs text-muted-foreground">{(entry.file.size / 1024).toFixed(0)} KB</span>
                    <select
                      value={entry.fileType}
                      onChange={(e) => {
                        const ft = e.target.value as 'GERBER' | 'ODB_PLUS_PLUS'
                        setFiles((prev) => prev.map((f, idx) => idx === i ? { ...f, fileType: ft } : f))
                      }}
                      className="text-xs border border-input bg-background rounded px-1 py-0.5"
                    >
                      <option value="GERBER">Gerber</option>
                      <option value="ODB_PLUS_PLUS">ODB++</option>
                    </select>
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
                  <label className="block text-sm font-medium text-foreground mb-1">File Type</label>
                  <select
                    value={fileType}
                    onChange={(e) => setFileType(e.target.value as 'GERBER' | 'ODB_PLUS_PLUS')}
                    className="w-full border border-input bg-background rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-ring"
                  >
                    <option value="GERBER">Gerber</option>
                    <option value="ODB_PLUS_PLUS">ODB++</option>
                  </select>
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

            <Button onClick={handleUpload} disabled={!hasFiles} className="w-full">
              {isBatch ? `Upload & Analyze ${files.length} Files` : 'Upload & Analyze'}
            </Button>
          </div>
        )}

        {/* Step: uploading */}
        {step === 'uploading' && (
          <div className="text-center space-y-4">
            <div className="animate-spin h-10 w-10 border-4 border-blue-600 border-t-transparent rounded-full mx-auto" />
            <p className="font-medium text-foreground">
              {isBatch ? `Uploading ${files.length} files to S3...` : 'Uploading to S3...'}
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
                      {entry.status === 'uploaded' ? '\u2713' : entry.status === 'failed' ? '\u2717' : `${entry.progress}%`}
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

        {/* Step: done */}
        {step === 'done' && (
          <div className="text-center space-y-4">
            <CheckCircle className="h-14 w-14 text-green-500 mx-auto" />
            <h2 className="text-xl font-bold text-foreground">Analysis Complete</h2>
            <p className="text-muted-foreground">
              {isBatch
                ? `All ${files.length} boards have been analyzed.`
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
            <Button onClick={() => { setStep('select'); clearAllFiles() }} variant="outline" className="w-full">Try again</Button>
          </div>
        )}
      </main>
    </div>
  )
}
