import { NextRequest, NextResponse } from 'next/server'

const API_URL = (process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080').replace(/\/+$/, '')
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

function fallbackOverview(job: JobResponse, counts: OverviewCounts, topIssues: string[]): string {
  const risk =
    counts.errors > 0 ? 'high' :
    counts.warnings > 0 ? 'moderate' :
    'low'

  const firstIssue = topIssues[0]
    ? topIssues[0].replace(/^\d+\.\s*/, '').split('Suggestion:')[0].trim()
    : 'No major issues are currently flagged.'

  return [
    `This submission is currently ${risk} risk with an MFG score of ${job.mfgScore} (${job.mfgGrade}).`,
    `Active findings include ${counts.errors} errors and ${counts.warnings} warnings.`,
    `Top priority: ${firstIssue}`,
  ].join(' ')
}

async function fetchApiJson<T>(path: string, authHeader: string | null): Promise<T> {
  const res = await fetch(`${API_URL}${path}`, {
    headers: {
      'Content-Type': 'application/json',
      ...(authHeader ? { Authorization: authHeader } : {}),
    },
    cache: 'no-store',
  })

  if (!res.ok) {
    const text = await res.text().catch(() => res.statusText)
    throw new Error(`API ${path}: ${res.status} ${text}`)
  }

  return res.json() as Promise<T>
}

async function generateOverviewWithAI(args: {
  job: JobResponse
  counts: OverviewCounts
  topIssues: string[]
}): Promise<string> {
  const { job, counts, topIssues } = args

  const prompt = [
    'Write a concise DFM overview for a non-expert PCB engineer.',
    'Use plain English and no markdown.',
    'Keep it to 2-4 sentences, focusing on manufacturing risk and top fix priorities.',
    `Job status: ${job.status}. MFG score: ${job.mfgScore}. Grade: ${job.mfgGrade}.`,
    `Active counts: errors=${counts.errors}, warnings=${counts.warnings}, infos=${counts.infos}.`,
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
      max_tokens: 220,
      messages: [
        {
          role: 'system',
          content:
            'You summarize PCB design-for-manufacturability analysis results for engineers. Be concise, direct, and practical.',
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

    let overview = fallbackOverview(job, counts, topIssues)
    let generatedWith: 'ai' | 'fallback' = 'fallback'

    if (OPENAI_API_KEY) {
      try {
        overview = await generateOverviewWithAI({ job, counts, topIssues })
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

