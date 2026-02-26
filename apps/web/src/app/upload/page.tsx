'use client'

import { useEffect, useState } from 'react'
import { useRouter } from 'next/navigation'
import Link from 'next/link'
import { ArrowLeft, CheckCircle, XCircle } from 'lucide-react'
import {
  createSubmission,
  uploadToS3,
  startAnalysis,
  getJob,
  getProfiles,
  type CapabilityProfile,
  type AnalysisJob,
} from '@/lib/api'
import { isLoggedIn } from '@/lib/auth'
import { FileUploader } from '@/components/ui/FileUploader'
import { Button } from '@/components/ui/button'
import { BetterDFMLogo } from '@/components/ui/betterdfm-logo'
import { cn } from '@/lib/utils'

type Step = 'select' | 'uploading' | 'analyzing' | 'done' | 'error'

const STEPS = ['select', 'uploading', 'analyzing', 'done']

export default function UploadPage() {
  const router = useRouter()
  const [file, setFile] = useState<File | null>(null)
  const [fileType, setFileType] = useState<'GERBER' | 'ODB_PLUS_PLUS'>('GERBER')
  const [profiles, setProfiles] = useState<CapabilityProfile[]>([])
  const [profileId, setProfileId] = useState<string>('')
  const [step, setStep] = useState<Step>('select')
  const [progress, setProgress] = useState(0)
  const [job, setJob] = useState<AnalysisJob | null>(null)
  const [errorMsg, setErrorMsg] = useState<string>('')

  useEffect(() => {
    if (!isLoggedIn()) { router.replace('/login'); return }
    getProfiles().then((ps) => {
      setProfiles(ps ?? [])
      const def = ps?.find((p) => p.isDefault)
      if (def) setProfileId(def.id)
    }).catch(() => {})
  }, [router])

  const handleUpload = async () => {
    if (!file) return
    setStep('uploading')
    setErrorMsg('')
    try {
      const { submissionId, presignedUrl } = await createSubmission(file.name, fileType)
      await uploadToS3(presignedUrl, file, setProgress)
      setStep('analyzing')
      const newJob = await startAnalysis(submissionId, profileId || undefined)
      setJob(newJob)

      // Poll until done
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

  const stepIndex = STEPS.indexOf(step === 'error' ? 'done' : step)

  return (
    <div className="min-h-screen bg-background">
      <header className="bg-card border-b px-6 py-5 flex items-center justify-between gap-4">
        <BetterDFMLogo />
        <div className="flex items-center gap-4">
          <Link href="/dashboard">
            <Button variant="ghost" size="lg"><ArrowLeft className="h-4 w-4 mr-1" />Dashboard</Button>
          </Link>
          <h1 className="text-xl font-semibold text-foreground">Upload & Analyze</h1>
        </div>
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

        {/* Step: select */}
        {step === 'select' && (
          <div className="space-y-6">
            <FileUploader accept=".zip,.gbr,.ger,.drl,.exc" onFile={setFile} />

            <div className="grid grid-cols-2 gap-4">
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
            </div>

            <Button onClick={handleUpload} disabled={!file} className="w-full">
              Upload &amp; Analyze
            </Button>
          </div>
        )}

        {/* Step: uploading */}
        {step === 'uploading' && (
          <div className="text-center space-y-4">
            <div className="animate-spin h-10 w-10 border-4 border-blue-600 border-t-transparent rounded-full mx-auto" />
            <p className="font-medium text-foreground">Uploading to S3…</p>
            <div className="w-full bg-muted rounded-full h-2">
              <div className="bg-blue-600 h-2 rounded-full transition-all" style={{ width: `${progress}%` }} />
            </div>
            <p className="text-sm text-muted-foreground">{progress}%</p>
          </div>
        )}

        {/* Step: analyzing */}
        {step === 'analyzing' && (
          <div className="text-center space-y-4">
            <div className="animate-spin h-10 w-10 border-4 border-purple-600 border-t-transparent rounded-full mx-auto" />
            <p className="font-medium text-foreground">Running DFM analysis…</p>
            <p className="text-sm text-muted-foreground">
              Status: <span className="font-mono">{job?.status ?? 'PENDING'}</span>
            </p>
            <p className="text-xs text-muted-foreground">This typically takes 10–60 seconds</p>
          </div>
        )}

        {/* Step: done */}
        {step === 'done' && job && (
          <div className="text-center space-y-4">
            <CheckCircle className="h-14 w-14 text-green-500 mx-auto" />
            <h2 className="text-xl font-bold text-foreground">Analysis Complete</h2>
            <p className="text-muted-foreground">Your board has been analyzed successfully.</p>
            <Button onClick={() => router.push(`/results/${job.id}`)} className="w-full">
              View Results
            </Button>
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
            <Button onClick={() => setStep('select')} variant="outline" className="w-full">Try again</Button>
          </div>
        )}
      </main>
    </div>
  )
}
