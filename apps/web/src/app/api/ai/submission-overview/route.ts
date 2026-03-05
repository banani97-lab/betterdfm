import { NextRequest, NextResponse } from 'next/server'

const OPENAI_API_KEY = process.env.OPENAI_API_KEY || ''
const OPENAI_MODEL = process.env.OPENAI_OVERVIEW_MODEL || 'gpt-4o-mini'

interface JobResponse {
  id: string
  status: 'PENDING' | 'PROCESSING' | 'DONE' | 'FAILED'
  mfgScore: number
  mfgGrade: string
}

interface ViolationResponse {
  severity: 'ERROR' | 'WARNING' | 'INFO'
  ruleId?: string
  message?: string
  suggestion?: string
  layer?: string
  ignored?: boolean
}

interface OverviewCounts {
  errors: number
  warnings: number
  infos: number
}

interface CauseCluster {
  label: string
  count: number
  example: string
}

function cleanBase(url: string): string {
  return url.replace(/\/+$/, '')
}

function resolveApiBases(): string[] {
  const explicitInternal =
    process.env.INTERNAL_API_URL ||
    process.env.API_INTERNAL_URL ||
    process.env.API_URL_INTERNAL ||
    process.env.API_URL

  const publicBase = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080'
  const raw = [explicitInternal, publicBase].filter((x): x is string => !!x)

  const bases: string[] = []
  for (const item of raw) {
    const base = cleanBase(item)
    bases.push(base)

    // Docker-friendly fallback: localhost from web container can't reach api container.
    if (base.includes('localhost:8080') || base.includes('127.0.0.1:8080')) {
      bases.push(base.replace('localhost', 'api').replace('127.0.0.1', 'api'))
    }
  }

  return Array.from(new Set(bases))
}

function computeCounts(violations: ViolationResponse[]): OverviewCounts {
  return violations.reduce<OverviewCounts>(
    (acc, v) => {
      if (v.ignored) return acc
      if (v.severity === 'ERROR') acc.errors += 1
      else if (v.severity === 'WARNING') acc.warnings += 1
      else if (v.severity === 'INFO') acc.infos += 1
      return acc
    },
    { errors: 0, warnings: 0, infos: 0 }
  )
}

function inferCauseLabel(message: string): string {
  const m = message.toLowerCase()
  if (/clearance|spacing|gap|distance/.test(m)) return 'insufficient spacing / clearance'
  if (/trace width|narrow trace|minimum width|line width|width/.test(m)) return 'trace width below process capability'
  if (/drill|hole|via|annular|ring/.test(m)) return 'drill/via geometry outside fab limits'
  if (/solder mask|mask dam/.test(m)) return 'solder mask opening/dam constraints'
  if (/edge|board edge|outline/.test(m)) return 'features too close to board outline'
  if (/aspect ratio/.test(m)) return 'hole aspect ratio constraint'
  return 'rule-specific geometry mismatch'
}

function dominantCauseLines(violations: ViolationResponse[], limit = 4): string[] {
  const buckets = new Map<string, CauseCluster>()

  for (const v of violations) {
    if (v.ignored) continue
    if (v.severity !== 'ERROR' && v.severity !== 'WARNING') continue
    const message = (v.message || 'Rule violation detected').trim()
    const label = inferCauseLabel(message)
    const existing = buckets.get(label)
    if (existing) {
      existing.count += 1
    } else {
      buckets.set(label, { label, count: 1, example: message })
    }
  }

  return Array.from(buckets.values())
    .sort((a, b) => b.count - a.count)
    .slice(0, limit)
    .map((c, i) => `${i + 1}. ${c.label} (observed ${c.count} times). Example: ${c.example}`)
}

function topIssueLines(violations: ViolationResponse[], limit = 5): string[] {
  const bucket = new Map<string, { count: number; severity: string; message: string; suggestion: string }>()

  for (const v of violations) {
    if (v.ignored) continue
    const message = (v.message || 'Rule violation detected').trim()
    const suggestion = (v.suggestion || '').trim()
    const key = `${v.severity}|${v.ruleId || 'unknown'}|${message}`
    const existing = bucket.get(key)
    if (existing) {
      existing.count += 1
    } else {
      bucket.set(key, {
        count: 1,
        severity: v.severity,
        message,
        suggestion,
      })
    }
  }

  const severityRank: Record<string, number> = { ERROR: 0, WARNING: 1, INFO: 2 }

  return Array.from(bucket.values())
    .sort((a, b) => {
      const sev = (severityRank[a.severity] ?? 99) - (severityRank[b.severity] ?? 99)
      if (sev !== 0) return sev
      return b.count - a.count
    })
    .slice(0, limit)
    .map((x, i) => {
      const suggestion = x.suggestion ? ` Suggestion: ${x.suggestion}` : ''
      return `${i + 1}. [${x.severity}] ${x.message} (count: ${x.count}).${suggestion}`
    })
}

function fallbackOverview(
  _job: JobResponse,
  counts: OverviewCounts,
  topIssues: string[],
  dominantCauses: string[]
): string {
  if (counts.errors === 0 && counts.warnings === 0) {
    return 'No active manufacturing-critical issues are currently flagged. Remaining findings are informational; keep focus on design-for-yield checks before release.'
  }

  const primaryCause = dominantCauses[0]
    ? dominantCauses[0].replace(/^\d+\.\s*/, '').split('Example:')[0].trim()
    : 'rule-specific geometry mismatches'
  const secondaryCause = dominantCauses[1]
    ? dominantCauses[1].replace(/^\d+\.\s*/, '').split('Example:')[0].trim()
    : ''
  const priorityIssue = topIssues[0]
    ? topIssues[0].replace(/^\d+\.\s*/, '').split('Suggestion:')[0].trim()
    : 'No single dominant violation is available yet.'

  return [
    `Most blocking findings appear to stem from ${primaryCause}.`,
    secondaryCause ? `A secondary contributor is ${secondaryCause}.` : '',
    `Highest-impact issue pattern: ${priorityIssue}`,
    'Recommended next pass is to address the dominant geometry constraints first, then re-run analysis to confirm that warning-level checks collapse as a side effect.',
  ]
    .filter(Boolean)
    .join(' ')
}

async function fetchApiJson<T>(path: string, authHeader: string | null): Promise<T> {
  const bases = resolveApiBases()
  const errors: string[] = []

  for (const base of bases) {
    try {
      const res = await fetch(`${base}${path}`, {
        headers: {
          'Content-Type': 'application/json',
          ...(authHeader ? { Authorization: authHeader } : {}),
        },
        cache: 'no-store',
      })

      if (!res.ok) {
        const text = await res.text().catch(() => res.statusText)
        throw new Error(`API ${path} via ${base}: ${res.status} ${text}`)
      }

      return res.json() as Promise<T>
    } catch (err) {
      errors.push(`${base} -> ${err instanceof Error ? err.message : String(err)}`)
    }
  }

  throw new Error(`Fetch failed for ${path}. Tried: ${errors.join(' | ')}`)
}

async function generateOverviewWithAI(args: {
  job: JobResponse
  counts: OverviewCounts
  topIssues: string[]
  dominantCauses: string[]
}): Promise<string> {
  const { job, counts, topIssues, dominantCauses } = args

  const prompt = [
    'Write a technical DFM overview for a PCB engineer.',
    'Use plain English and no markdown.',
    'Length: 4-8 sentences (roughly up to ~2x longer than a short summary).',
    'Focus on root causes driving most errors and warnings, and what to fix first.',
    'Do NOT mention the exact MFG score, grade, or exact counts of errors/warnings because those are already shown elsewhere in the UI.',
    `Job status: ${job.status}.`,
    `Active finding profile: errors=${counts.errors}, warnings=${counts.warnings}, infos=${counts.infos}.`,
    'Dominant cause clusters:',
    ...dominantCauses,
    'Top issues:',
    ...topIssues,
  ].join('\n')

  const res = await fetch('https://api.openai.com/v1/chat/completions', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${OPENAI_API_KEY}`,
    },
    body: JSON.stringify({
      model: OPENAI_MODEL,
      temperature: 0.3,
      max_tokens: 420,
      messages: [
        {
          role: 'system',
          content:
            'You summarize PCB design-for-manufacturability analysis results for engineers. Be technical, direct, and practical.',
        },
        { role: 'user', content: prompt },
      ],
    }),
  })

  if (!res.ok) {
    const text = await res.text().catch(() => res.statusText)
    throw new Error(`OpenAI error: ${res.status} ${text}`)
  }

  const data = await res.json()
  const message = data?.choices?.[0]?.message?.content
  if (typeof message !== 'string' || !message.trim()) {
    throw new Error('OpenAI returned an empty overview')
  }

  return message.trim()
}

export async function POST(req: NextRequest) {
  try {
    const body = await req.json().catch(() => ({}))
    const jobId = typeof body?.jobId === 'string' ? body.jobId.trim() : ''
    if (!jobId) {
      return NextResponse.json({ error: 'jobId is required' }, { status: 400 })
    }

    const authHeader = req.headers.get('authorization')
    const [job, violations] = await Promise.all([
      fetchApiJson<JobResponse>(`/jobs/${jobId}`, authHeader),
      fetchApiJson<ViolationResponse[]>(`/jobs/${jobId}/violations`, authHeader),
    ])

    const counts = computeCounts(violations ?? [])
    const topIssues = topIssueLines(violations ?? [])
    const dominantCauses = dominantCauseLines(violations ?? [])

    let overview = fallbackOverview(job, counts, topIssues, dominantCauses)
    let generatedWith: 'ai' | 'fallback' = 'fallback'

    if (OPENAI_API_KEY) {
      try {
        overview = await generateOverviewWithAI({ job, counts, topIssues, dominantCauses })
        generatedWith = 'ai'
      } catch (err) {
        console.error('[ai/submission-overview] AI generation failed, using fallback:', err)
      }
    }

    return NextResponse.json({
      overview,
      counts,
      generatedWith,
    })
  } catch (err) {
    const message = err instanceof Error ? err.message : 'Failed to generate overview'
    console.error('[ai/submission-overview]', message)
    return NextResponse.json({ error: message }, { status: 500 })
  }
}
