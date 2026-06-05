'use client'

import { Fragment, useEffect, useState } from 'react'

const TOC = [
  { id: 'overview', label: 'Overview' },
  { id: 'deployments', label: 'Deployments' },
  { id: 'architecture', label: 'Architecture' },
  { id: 'dfm-engine', label: 'DFM Engine' },
  { id: 'geometry', label: '↳ Geometry' },
  { id: 'clearance', label: '↳ Clearance Rule' },
  { id: 'scoring', label: '↳ Scoring' },
  { id: 'async-pipeline', label: 'Async Pipeline' },
  { id: 'sidecar', label: 'Gerbonara Sidecar' },
  { id: 'odb-format', label: '↳ ODB++ Format' },
  { id: 'frontend', label: 'Frontend' },
  { id: 'design-decisions', label: 'Design Decisions' },
  { id: 'cicd', label: 'CI / CD' },
]

function useTocActive() {
  const [active, setActive] = useState('overview')
  useEffect(() => {
    const els = TOC.map(({ id }) => document.getElementById(id)).filter(Boolean)
    const obs = new IntersectionObserver(
      (entries) => {
        const visible = entries.filter((e) => e.isIntersecting)
        if (visible.length > 0) setActive(visible[0].target.id)
      },
      { rootMargin: '-10% 0px -80% 0px' }
    )
    els.forEach((el) => obs.observe(el!))
    return () => obs.disconnect()
  }, [])
  return active
}

function Code({ children, lang = 'go' }: { children: string; lang?: string }) {
  const lines = children.trim().split('\n')
  return (
    <pre
      className="overflow-x-auto rounded-xl text-sm leading-relaxed"
      style={{
        background: '#0d1117',
        border: '1px solid #1e2432',
        padding: '1.25rem 1.5rem',
        fontFamily: 'ui-monospace, "Cascadia Code", "Source Code Pro", Menlo, monospace',
      }}
    >
      <code>
        {lines.map((line, i) => (
          <div key={i} style={{ color: '#c9d1d9' }}>
            {tokenize(line, lang)}
          </div>
        ))}
      </code>
    </pre>
  )
}

function tokenize(line: string, lang: string) {
  if (lang === 'go') return tokenizeGo(line)
  if (lang === 'python') return tokenizePython(line)
  if (lang === 'bash') return tokenizeBash(line)
  return <span>{line}</span>
}

function tokenizeGo(line: string) {
  const keywords = /\b(type|interface|struct|func|return|if|for|range|var|const|package|import|go|defer|select|case|break|continue|nil|true|false|error)\b/g
  const strings = /"[^"]*"/g
  const comments = /\/\/.*/
  const numbers = /\b\d+(\.\d+)?\b/g

  if (comments.test(line)) {
    const idx = line.search(/\/\//)
    return (
      <>
        {tokenizeGoInline(line.slice(0, idx))}
        <span style={{ color: '#8b949e' }}>{line.slice(idx)}</span>
      </>
    )
  }
  return tokenizeGoInline(line)
}

function tokenizeGoInline(text: string): React.ReactNode {
  const parts: React.ReactNode[] = []
  let remaining = text
  let key = 0

  const patterns: [RegExp, string][] = [
    [/"[^"]*"/, '#a5d6ff'],
    [/`[^`]*`/, '#a5d6ff'],
    [/\b(type|interface|struct|func|return|if|for|range|var|const|package|import|go|defer|select|case|break|continue|nil|true|false|error)\b/, '#ff7b72'],
    [/\b\d+(\.\d+)?\b/, '#79c0ff'],
    [/\b(BoardData|Violation|ProfileRules|Rule|Runner|sync\.WaitGroup|[]Violation)\b/, '#e3b341'],
  ]

  while (remaining.length > 0) {
    let earliest: { idx: number; match: string; color: string } | null = null

    for (const [re, color] of patterns) {
      const m = remaining.match(re)
      if (m && m.index !== undefined) {
        if (!earliest || m.index < earliest.idx) {
          earliest = { idx: m.index, match: m[0], color }
        }
      }
    }

    if (!earliest) {
      parts.push(<span key={key++} style={{ color: '#c9d1d9' }}>{remaining}</span>)
      break
    }

    if (earliest.idx > 0) {
      parts.push(<span key={key++} style={{ color: '#c9d1d9' }}>{remaining.slice(0, earliest.idx)}</span>)
    }
    parts.push(<span key={key++} style={{ color: earliest.color }}>{earliest.match}</span>)
    remaining = remaining.slice(earliest.idx + earliest.match.length)
  }

  return <>{parts}</>
}

function tokenizePython(line: string): React.ReactNode {
  if (line.trim().startsWith('#')) return <span style={{ color: '#8b949e' }}>{line}</span>
  const kwRe = /\b(def|class|return|if|elif|else|for|in|import|from|as|with|raise|try|except|finally|None|True|False|self|yield|async|await)\b/g
  const strRe = /"[^"]*"|'[^']*'/g
  const parts: React.ReactNode[] = []
  const tokens: { start: number; end: number; color: string }[] = []

  let m: RegExpExecArray | null
  const kwCopy = new RegExp(kwRe.source, 'g')
  while ((m = kwCopy.exec(line)) !== null) tokens.push({ start: m.index, end: m.index + m[0].length, color: '#ff7b72' })
  const strCopy = new RegExp(strRe.source, 'g')
  while ((m = strCopy.exec(line)) !== null) tokens.push({ start: m.index, end: m.index + m[0].length, color: '#a5d6ff' })

  tokens.sort((a, b) => a.start - b.start)
  let pos = 0, key = 0
  for (const t of tokens) {
    if (t.start < pos) continue
    if (t.start > pos) parts.push(<span key={key++} style={{ color: '#c9d1d9' }}>{line.slice(pos, t.start)}</span>)
    parts.push(<span key={key++} style={{ color: t.color }}>{line.slice(t.start, t.end)}</span>)
    pos = t.end
  }
  if (pos < line.length) parts.push(<span key={key++} style={{ color: '#c9d1d9' }}>{line.slice(pos)}</span>)
  return <>{parts}</>
}

function tokenizeBash(line: string): React.ReactNode {
  if (line.trim().startsWith('#')) return <span style={{ color: '#8b949e' }}>{line}</span>
  return <span style={{ color: '#c9d1d9' }}>{line}</span>
}

function SectionAnchor({ id }: { id: string }) {
  return <span id={id} style={{ position: 'relative', top: '-80px', display: 'block' }} />
}

function Tag({ type }: { type: 'ERROR' | 'WARNING' | 'INFO' }) {
  const styles = {
    ERROR: { bg: 'rgba(239,68,68,0.12)', color: '#f87171', border: 'rgba(239,68,68,0.3)' },
    WARNING: { bg: 'rgba(245,158,11,0.12)', color: '#fbbf24', border: 'rgba(245,158,11,0.3)' },
    INFO: { bg: 'rgba(96,165,250,0.12)', color: '#93c5fd', border: 'rgba(96,165,250,0.3)' },
  }[type]
  return (
    <span style={{
      background: styles.bg,
      color: styles.color,
      border: `1px solid ${styles.border}`,
      padding: '1px 8px',
      borderRadius: '4px',
      fontSize: '11px',
      fontWeight: 700,
      fontFamily: 'ui-monospace, monospace',
      letterSpacing: '0.04em',
    }}>
      {type}
    </span>
  )
}

function Callout({ number, title, children }: { number: number; title: string; children: React.ReactNode }) {
  return (
    <div style={{
      display: 'flex',
      gap: '1rem',
      padding: '1rem 1.25rem',
      background: 'rgba(212,137,26,0.05)',
      border: '1px solid rgba(212,137,26,0.2)',
      borderLeft: '3px solid #d4891a',
      borderRadius: '0 8px 8px 0',
      marginBottom: '0.75rem',
    }}>
      <span style={{
        color: '#d4891a',
        fontWeight: 700,
        fontSize: '13px',
        fontFamily: 'ui-monospace, monospace',
        minWidth: '20px',
        paddingTop: '1px',
      }}>
        {String(number).padStart(2, '0')}
      </span>
      <div>
        <div style={{ color: '#e2e8f0', fontWeight: 600, fontSize: '14px', marginBottom: '2px' }}>{title}</div>
        <div style={{ color: '#94a3b8', fontSize: '13px', lineHeight: 1.6 }}>{children}</div>
      </div>
    </div>
  )
}

const RULES = [
  // Bare-board / fab (12)
  { id: 'trace-width', group: 'fab', sev: 'ERROR', desc: 'Trace width ≥ minTraceWidthMM' },
  { id: 'clearance', group: 'fab', sev: 'ERROR', desc: 'Trace/pad spacing ≥ minClearanceMM' },
  { id: 'drill-size', group: 'fab', sev: 'ERROR', desc: 'Drill diameter within min/max bounds' },
  { id: 'annular-ring', group: 'fab', sev: 'ERROR', desc: 'Copper ring around vias ≥ minAnnularRingMM' },
  { id: 'drill-to-drill', group: 'fab', sev: 'ERROR', desc: 'Hole-to-hole spacing ≥ minDrillToDrillMM' },
  { id: 'drill-to-copper', group: 'fab', sev: 'ERROR', desc: 'Hole edge to nearest copper ≥ minDrillToCopperMM' },
  { id: 'aspect-ratio', group: 'fab', sev: 'ERROR', desc: 'Board thickness ÷ drill diameter ≤ maxAspectRatio' },
  { id: 'solder-mask-dam', group: 'fab', sev: 'WARNING', desc: 'Solder mask bridge between pads ≥ minSolderMaskDamMM' },
  { id: 'edge-clearance', group: 'fab', sev: 'ERROR', desc: 'Copper distance from board outline ≥ minEdgeClearanceMM' },
  { id: 'copper-sliver', group: 'fab', sev: 'WARNING', desc: 'Narrow copper features ≥ minCopperSliverMM' },
  { id: 'mounting-hole-keepout', group: 'fab', sev: 'WARNING', desc: 'Copper keepout around non-plated mounting holes ≥ minMountingHoleKeepoutMM (IPC-2221B)' },
  { id: 'silkscreen-on-pad', group: 'fab', sev: 'ERROR', desc: 'Silkscreen does not overlap pads' },
  // Assembly (10)
  { id: 'fiducial-count', group: 'assembly', sev: 'WARNING', desc: 'Board has ≥ 3 fiducials for pick-and-place (skipped if none found; enableFiducialCountCheck)' },
  { id: 'pad-size-for-package', group: 'assembly', sev: ['ERROR', 'INFO'], desc: 'Pad geometry within IPC-7351 envelope for the package class; ERROR when undersized, INFO when oversized (enablePadSizeForPackageCheck)' },
  { id: 'package-capability', group: 'assembly', sev: 'ERROR', desc: 'No package smaller than the CM’s smallestPackageClass' },
  { id: 'tombstoning-risk', group: 'assembly', sev: 'ERROR', desc: 'Pad area ratio on small 2-pad passives ≤ 1.3 (reflow imbalance; enableTombstoningRiskCheck)' },
  { id: 'trace-imbalance', group: 'assembly', sev: 'ERROR', desc: 'Thermal trace/pour balance into 2-pad components ≤ maxTraceImbalanceRatio' },
  { id: 'component-height', group: 'assembly', sev: ['ERROR', 'INFO'], desc: 'SMT component height within per-side limits; ERROR over limit, INFO when parts lack height data (maxComponentHeightTop/BottomMM)' },
  { id: 'component-spacing', group: 'assembly', sev: ['WARNING', 'ERROR'], desc: 'Same-side courtyard edge-to-edge gap (IPC-7351B): flat minComponentSpacingMM, or per-package-class keepouts via componentSpacing (discrete/leaded/BGA/through-hole, pair limit = larger of the two); WARNING under the gap, ERROR on courtyard overlap' },
  { id: 'via-in-pad', group: 'assembly', sev: ['WARNING', 'INFO'], desc: 'Via landing in an SMT land; WARNING fine-pitch/BGA, INFO otherwise (IPC-4761/7093; enableViaInPadCheck)' },
  { id: 'through-hole-on-bottom', group: 'assembly', sev: 'WARNING', desc: 'Through-hole / press-fit parts on the bottom side (flagThroughHoleOnBottom)' },
  { id: 'fiducial-placement', group: 'assembly', sev: ['WARNING', 'INFO'], desc: 'WARNING when global fiducials are collinear; INFO when fine-pitch/BGA parts lack a local fiducial (IPC-7351, enableFiducialPlacementCheck)' },
] as const

export default function TechnicalPage() {
  const active = useTocActive()

  return (
    <div style={{
      background: '#0c0e12',
      minHeight: '100vh',
      color: '#e2e8f0',
      fontFamily: '"Segoe UI", system-ui, -apple-system, sans-serif',
    }}>
      {/* Hero */}
      <div className="tech-hero" style={{
        background: 'linear-gradient(180deg, rgba(212,137,26,0.06) 0%, transparent 100%)',
        borderBottom: '1px solid #1e2432',
        padding: '4rem 2rem 3rem',
        textAlign: 'center',
      }}>
        <div style={{
          display: 'inline-block',
          background: 'rgba(212,137,26,0.12)',
          border: '1px solid rgba(212,137,26,0.3)',
          color: '#d4891a',
          fontSize: '11px',
          fontWeight: 700,
          letterSpacing: '0.12em',
          padding: '4px 12px',
          borderRadius: '4px',
          marginBottom: '1.25rem',
          fontFamily: 'ui-monospace, monospace',
        }}>
          TECHNICAL DEEP DIVE
        </div>
        <h1 style={{
          fontSize: 'clamp(2rem, 5vw, 3.25rem)',
          fontWeight: 800,
          letterSpacing: '-0.03em',
          lineHeight: 1.1,
          marginBottom: '1rem',
          background: 'linear-gradient(135deg, #e2e8f0 0%, #94a3b8 100%)',
          WebkitBackgroundClip: 'text',
          WebkitTextFillColor: 'transparent',
        }}>
          Building a PCB DFM Analysis Engine
        </h1>
        <p style={{ color: '#64748b', fontSize: '1.05rem', maxWidth: '560px', margin: '0 auto 2rem', lineHeight: 1.7 }}>
          Architecture, geometric algorithms, and engineering trade-offs behind RapidDFM — a full-stack SaaS platform that automatically screens PCB designs for manufacturability in under 30 seconds.
        </p>
        <div style={{ display: 'flex', gap: '1.5rem', justifyContent: 'center', flexWrap: 'wrap' }}>
          {[
            ['Go 1.23', '#00acd7'],
            ['Next.js 14', '#e2e8f0'],
            ['Python FastAPI', '#3776ab'],
            ['PostgreSQL 16', '#336791'],
            ['AWS ECS / SQS / S3', '#ff9900'],
          ].map(([label, color]) => (
            <span key={label} style={{
              fontSize: '12px',
              fontFamily: 'ui-monospace, monospace',
              color,
              background: `${color}15`,
              border: `1px solid ${color}30`,
              padding: '3px 10px',
              borderRadius: '4px',
            }}>
              {label}
            </span>
          ))}
        </div>
      </div>

      {/* Layout: TOC + Content */}
      <div className="tech-layout" style={{
        display: 'grid',
        gridTemplateColumns: '1fr',
        maxWidth: '1100px',
        margin: '0 auto',
        gap: '0',
        padding: '0 1rem',
      }}>
        {/* TOC */}
        <aside style={{
          position: 'sticky',
          top: '80px',
          height: 'fit-content',
          padding: '2.5rem 1.5rem 2rem 0',
          display: 'none',
        }}
          className="tech-toc"
        >
          <div style={{ fontSize: '10px', fontWeight: 700, letterSpacing: '0.1em', color: '#475569', marginBottom: '0.75rem', fontFamily: 'ui-monospace, monospace' }}>
            ON THIS PAGE
          </div>
          {TOC.map(({ id, label }) => (
            <a
              key={id}
              href={`#${id}`}
              style={{
                display: 'block',
                padding: '5px 8px',
                fontSize: '12.5px',
                color: active === id ? '#d4891a' : '#64748b',
                background: active === id ? 'rgba(212,137,26,0.08)' : 'transparent',
                borderLeft: `2px solid ${active === id ? '#d4891a' : 'transparent'}`,
                borderRadius: '0 4px 4px 0',
                marginBottom: '2px',
                textDecoration: 'none',
                transition: 'all 0.15s',
                fontFamily: label.startsWith('↳') ? 'ui-monospace, monospace' : 'inherit',
              }}
            >
              {label}
            </a>
          ))}
        </aside>

        {/* Main content */}
        <main className="tech-main" style={{ padding: '3rem 0 6rem', maxWidth: '760px', minWidth: 0 }}>

          {/* ─── OVERVIEW ─────────────────────────────────── */}
          <SectionAnchor id="overview" />
          <section style={{ marginBottom: '4rem' }}>
            <h2 style={h2Style}>What is RapidDFM?</h2>
            <p style={pStyle}>
              RapidDFM is a SaaS Design-for-Manufacturability (DFM) analysis platform aimed at contract PCB manufacturers. A CM white-labels it as a portal — their customers upload ODB++ files, the platform runs 22 manufacturing rule checks against a configurable capability profile, and returns a scored manufacturability report with violations pinpointed on an interactive SVG board viewer.
            </p>
            <p style={pStyle}>
              The core insight: CMs today do this review manually, spending 30–60 minutes per board opening CAM tools and checking clearances, drill sizes, and annular rings by eye. RapidDFM automates the entire first pass in under 30 seconds, surfaces all violations with coordinates and severity, and gives customers a shareable link they can use to track revisions.
            </p>
            <div style={{
              display: 'grid',
              gridTemplateColumns: 'repeat(auto-fit, minmax(140px, 1fr))',
              gap: '1rem',
              marginTop: '1.5rem',
            }}>
              {[
                { label: '22', sub: 'DFM rules' },
                { label: '<30s', sub: 'analysis time' },
                { label: '2', sub: 'file formats' },
                { label: 'A–F', sub: 'mfg grade' },
              ].map(({ label, sub }) => (
                <div key={sub} style={{
                  background: '#111418',
                  border: '1px solid #1e2432',
                  borderRadius: '10px',
                  padding: '1rem',
                  textAlign: 'center',
                }}>
                  <div style={{ fontSize: '1.75rem', fontWeight: 800, color: '#d4891a', lineHeight: 1 }}>{label}</div>
                  <div style={{ fontSize: '12px', color: '#64748b', marginTop: '4px' }}>{sub}</div>
                </div>
              ))}
            </div>
          </section>

          {/* ─── DEPLOYMENT EDITIONS ──────────────────────── */}
          <SectionAnchor id="deployments" />
          <section style={{ marginBottom: '4rem' }}>
            <h2 style={h2Style}>Deployment Editions</h2>
            <p style={pStyle}>
              RapidDFM ships in two deployment editions from one codebase. The <strong style={{ color: '#e2e8f0' }}>Public Cloud</strong> edition is the commercial multi-tenant SaaS detailed throughout the rest of this page. The <strong style={{ color: '#e2e8f0' }}>GovCloud</strong> edition runs the same services inside an AWS GovCloud (US) boundary so contract manufacturers can analyze export-controlled (ITAR) and CUI designs, with access restricted to US persons. The two run in completely separate AWS accounts and partitions.
            </p>

            {/* Public Cloud vs GovCloud comparison */}
            <div style={{
              background: '#0d1117',
              border: '1px solid #1e2432',
              borderRadius: '12px',
              marginTop: '1.5rem',
              marginBottom: '1.5rem',
              overflowX: 'auto',
            }}>
              <table style={{ width: '100%', borderCollapse: 'collapse', minWidth: '560px' }}>
                <thead>
                  <tr>
                    <th style={thStyle}></th>
                    <th style={thStyle}>Public Cloud</th>
                    <th style={thStyle}>GovCloud</th>
                  </tr>
                </thead>
                <tbody>
                  {[
                    { aspect: 'Region / partition', pub: 'us-east-1 (aws)', gov: 'us-gov-west-1 (aws-us-gov), FIPS 140 endpoints' },
                    { aspect: 'Web hosting', pub: 'Vercel', gov: 'ECS / Fargate behind an ALB' },
                    { aspect: 'API hosting', pub: 'AWS App Runner', gov: 'ECS / Fargate behind an ALB' },
                    { aspect: 'Worker + sidecar', pub: 'ECS / Fargate', gov: 'ECS / Fargate, private subnets' },
                    { aspect: 'Database', pub: 'RDS Postgres', gov: 'RDS Postgres, CMK-encrypted, TLS forced, private' },
                    { aspect: 'Object storage', pub: 'S3', gov: 'S3, SSE-KMS (customer CMK), TLS-only, versioned' },
                    { aspect: 'Encryption at rest', pub: 'AWS-managed keys', gov: 'Customer-managed KMS CMK (US-person key policy)' },
                    { aspect: 'Identity', pub: 'Cognito', gov: 'Cognito with MFA, invite-only' },
                    { aspect: 'AI overview', pub: 'OpenAI (external call)', gov: 'On-box, no external egress' },
                    { aspect: 'Audit', pub: 'Standard', gov: 'Full audit logging + alerting' },
                    { aspect: 'CI / CD', pub: 'GitHub Actions', gov: 'Keyless OIDC deploys' },
                    { aspect: 'Network', pub: 'Public services', gov: 'VPC, private subnets, ALB-only ingress' },
                    { aspect: 'Compliance posture', pub: 'Commercial SaaS', gov: 'ITAR / CUI boundary; targeting FedRAMP Moderate equivalency' },
                  ].map((row, i) => (
                    <tr key={row.aspect} style={{ borderTop: i === 0 ? '1px solid #1e2432' : '1px solid #161b26' }}>
                      <td style={{ ...tdStyle, color: '#94a3b8', fontWeight: 600, whiteSpace: 'nowrap' }}>{row.aspect}</td>
                      <td style={{ ...tdStyle, color: '#94a3b8' }}>{row.pub}</td>
                      <td style={{ ...tdStyle, color: '#cbd5e1' }}>{row.gov}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>

            <h3 style={h3Style}>US-person boundary</h3>
            <p style={pStyle}>
              The GovCloud edition runs in an isolated AWS GovCloud (US) environment with FIPS 140-validated endpoints and a private network. Only the load balancer is internet-facing, and access is limited to US persons.
            </p>

            <h3 style={h3Style}>Encryption you control</h3>
            <p style={pStyle}>
              A customer-managed encryption key, held under US-person control, protects data at rest and in transit. This is the basis for handling ITAR technical data under the <code style={inlineCode}>22 CFR 120.54</code> encryption carve-out: data that is decrypted only in-boundary by US persons is not an export.
            </p>

            <h3 style={h3Style}>In-boundary by design</h3>
            <p style={pStyle}>
              Sign-in uses multi-factor authentication with invite-only, US-person provisioning. The edition makes no third-party API calls, so analysis stays entirely within the boundary, and all activity is captured by comprehensive audit logging and security alerting.
            </p>

            <h3 style={h3Style}>Strict tenant isolation</h3>
            <p style={pStyle}>
              Each customer&apos;s data is isolated per organization, enforced in the application and verified by an automated test suite.
            </p>
          </section>

          {/* ─── ARCHITECTURE ─────────────────────────────── */}
          <SectionAnchor id="architecture" />
          <section style={{ marginBottom: '4rem' }}>
            <h2 style={h2Style}>Architecture</h2>
            <p style={pStyle}>
              Five services in a monorepo, orchestrated via Docker Compose locally and GitHub Actions for production deploys. The key principle: the API never touches file bytes — uploads go directly from the browser to S3 via presigned URLs, and analysis result blobs (board geometry, violations) are served back the same way.
            </p>

            {/* Architecture diagram */}
            <div className="tech-arch-diagram" style={{
              background: '#0d1117',
              border: '1px solid #1e2432',
              borderRadius: '12px',
              padding: '2rem 1.5rem',
              marginTop: '1.5rem',
              marginBottom: '1.5rem',
              overflowX: 'auto',
            }}>
              <div style={{ display: 'flex', flexDirection: 'column', gap: '1.5rem', minWidth: '500px' }}>
                {/* Row 1: Browser + S3 */}
                <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem' }}>
                  <ServiceBox label="Browser" sub="Next.js 14" color="#4a9eff" />
                  <Arrow label="① presigned PUT" />
                  <ServiceBox label="Amazon S3" sub="file storage" color="#ff9900" />
                  <div style={{ color: '#475569', fontSize: '11px', marginLeft: '0.5rem', maxWidth: '160px', lineHeight: 1.4 }}>
                    Browser uploads directly. API never handles bytes.
                  </div>
                </div>

                {/* Row 2: Browser → API → SQS */}
                <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem' }}>
                  <ServiceBox label="Browser" sub="Next.js 14" color="#4a9eff" />
                  <Arrow label="② startAnalysis()" />
                  <ServiceBox label="Go API" sub="Echo · :8080" color="#00acd7" />
                  <Arrow label="③ enqueue {jobId}" />
                  <ServiceBox label="Amazon SQS" sub="job queue" color="#ff9900" />
                </div>

                {/* Row 3: Worker → Sidecar */}
                <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem' }}>
                  <ServiceBox label="Go Worker" sub="ECS · 5 goroutines" color="#00acd7" />
                  <Arrow label="④ POST /parse" />
                  <ServiceBox label="Gerbonara" sub="Python FastAPI · :8001" color="#3776ab" />
                  <Arrow label="⑤ BoardData JSON" dir="left" />
                </div>

                {/* Row 4: Worker → DFM → DB + S3 result blobs */}
                <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem' }}>
                  <ServiceBox label="Go Worker" sub="ECS · 5 goroutines" color="#00acd7" />
                  <Arrow label="⑥ run 22 rules" />
                  <ServiceBox label="DFM Engine" sub="Go library" color="#d4891a" />
                  <Arrow label="⑦ score + metadata" />
                  <ServiceBox label="PostgreSQL" sub="RDS · GORM" color="#336791" />
                </div>

                {/* Row 5: Worker → S3 result blobs */}
                <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem' }}>
                  <ServiceBox label="Go Worker" sub="ECS · 5 goroutines" color="#00acd7" />
                  <Arrow label="⑧ PUT board.json + violations.json" />
                  <ServiceBox label="Amazon S3" sub="results/{jobId}/" color="#ff9900" />
                  <div style={{ color: '#475569', fontSize: '11px', marginLeft: '0.5rem', maxWidth: '180px', lineHeight: 1.4 }}>
                    Large geometric blobs offloaded; DB stores S3 keys only.
                  </div>
                </div>

                {/* Row 6: Frontend poll + presigned fetch */}
                <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem' }}>
                  <ServiceBox label="Browser" sub="Next.js 14" color="#4a9eff" />
                  <Arrow label="⑨ poll · then GET /jobs/:id/board" />
                  <ServiceBox label="Go API" sub="Echo · :8080" color="#00acd7" />
                  <Arrow label="⑩ {url: presigned}" dir="left" />
                </div>

                {/* Row 7: Browser → S3 direct fetch */}
                <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem' }}>
                  <ServiceBox label="Browser" sub="Next.js 14" color="#4a9eff" />
                  <Arrow label="⑪ GET presigned URL" />
                  <ServiceBox label="Amazon S3" sub="results/{jobId}/" color="#ff9900" />
                  <div style={{ color: '#475569', fontSize: '11px', marginLeft: '0.5rem', maxWidth: '200px', lineHeight: 1.4 }}>
                    Browser pulls board + violations directly from S3, renders SVG board viewer.
                  </div>
                </div>
              </div>
            </div>

            <p style={pStyle}>
              The monorepo uses a Go <code style={inlineCode}>replace</code> directive so the DFM engine package is imported by both the API and the worker from the local filesystem — no versioned publishing required.
            </p>
          </section>

          {/* ─── DFM ENGINE ───────────────────────────────── */}
          <SectionAnchor id="dfm-engine" />
          <section style={{ marginBottom: '4rem' }}>
            <h2 style={h2Style}>DFM Engine</h2>
            <p style={pStyle}>
              The engine lives at <code style={inlineCode}>engine/dfm-engine/</code> as a pure Go library with no external dependencies. Every rule implements a two-method interface:
            </p>

            <Code lang="go">{`type Rule interface {
    ID() string
    Run(board BoardData, profile ProfileRules) []Violation
}

// Runner executes all rules concurrently and merges results.
func (r *Runner) Run(board BoardData, profile ProfileRules) []Violation {
    var mu sync.Mutex
    var wg sync.WaitGroup
    results := make([][]Violation, len(r.rules))

    for i, rule := range r.rules {
        wg.Add(1)
        go func(idx int, rl Rule) {
            defer wg.Done()
            results[idx] = rl.Run(board, profile)
        }(i, rule)
    }

    wg.Wait()
    // Merge deterministically: rule 0 violations first, then rule 1, etc.
    var all []Violation
    for _, vs := range results {
        all = append(all, vs...)
    }
    return all
}`}</Code>

            <p style={pStyle}>
              All 22 rules run concurrently via <code style={inlineCode}>sync.WaitGroup</code>. Since every rule receives read-only board data, there's no contention — no mutexes needed inside individual rules.
            </p>

            {/* Rules table */}
            <div style={{ overflowX: 'auto', marginTop: '1.5rem' }}>
              <table style={{
                width: '100%',
                borderCollapse: 'collapse',
                fontSize: '13px',
              }}>
                <thead>
                  <tr style={{ background: '#111418', borderBottom: '1px solid #1e2432' }}>
                    <th style={thStyle}>Rule ID</th>
                    <th style={thStyle}>Severity</th>
                    <th style={thStyle}>What it checks</th>
                  </tr>
                </thead>
                <tbody>
                  {RULES.map((rule, i) => {
                    const prev = RULES[i - 1]
                    const showGroup = !prev || prev.group !== rule.group
                    return (
                      <Fragment key={rule.id}>
                        {showGroup && (
                          <tr style={{ background: '#111418' }}>
                            <td colSpan={3} style={{
                              padding: '7px 12px',
                              fontSize: '10px',
                              fontWeight: 700,
                              letterSpacing: '0.08em',
                              textTransform: 'uppercase',
                              color: '#d4891a',
                              fontFamily: 'ui-monospace, monospace',
                              borderTop: '1px solid #1e2432',
                              borderBottom: '1px solid #1a1f2a',
                            }}>
                              {rule.group === 'fab'
                                ? 'Bare-board / fab — 12'
                                : 'Assembly — 10'}
                            </td>
                          </tr>
                        )}
                        <tr style={{
                          background: i % 2 === 0 ? 'transparent' : '#0d1117',
                          borderBottom: '1px solid #1a1f2a',
                        }}>
                          <td style={{ ...tdStyle, fontFamily: 'ui-monospace, monospace', color: '#d4891a', fontSize: '12px' }}>
                            {rule.id}
                          </td>
                          <td style={tdStyle}>
                            <span style={{ display: 'inline-flex', gap: '4px', flexWrap: 'wrap' }}>
                              {((Array.isArray(rule.sev) ? rule.sev : [rule.sev]) as Array<'ERROR' | 'WARNING' | 'INFO'>).map((s) => (
                                <Tag key={s} type={s} />
                              ))}
                            </span>
                          </td>
                          <td style={{ ...tdStyle, color: '#94a3b8' }}>{rule.desc}</td>
                        </tr>
                      </Fragment>
                    )
                  })}
                </tbody>
              </table>
            </div>
          </section>

          {/* ─── GEOMETRY ─────────────────────────────────── */}
          <SectionAnchor id="geometry" />
          <section style={{ marginBottom: '4rem' }}>
            <h2 style={h2Style}>Geometric Algorithms</h2>
            <p style={pStyle}>
              Clearance and edge-clearance checks need exact minimum distances between PCB features. Treating every pad as a circle produces false positives on rectangular and oval pads, so the engine computes shape-aware distances for all common pad geometries.
            </p>

            <h3 style={h3Style}>Shape-aware pad distance</h3>
            <div style={{
              display: 'grid',
              gridTemplateColumns: 'repeat(auto-fit, minmax(200px, 1fr))',
              gap: '0.75rem',
              marginTop: '1rem',
              marginBottom: '1.5rem',
            }}>
              {[
                { shape: 'CIRCLE', detail: 'Exact edge distance' },
                { shape: 'RECT', detail: 'Exact edge distance for rectangular pads' },
                { shape: 'OVAL / Stadium', detail: 'Exact edge distance for oval / stadium pads' },
                { shape: 'POLYGON', detail: 'Exact edge distance for arbitrary polygon pads' },
              ].map(({ shape, detail }) => (
                <div key={shape} style={{
                  background: '#111418',
                  border: '1px solid #1e2432',
                  borderRadius: '8px',
                  padding: '0.875rem',
                }}>
                  <div style={{ color: '#d4891a', fontFamily: 'ui-monospace, monospace', fontSize: '12px', fontWeight: 700, marginBottom: '4px' }}>
                    {shape}
                  </div>
                  <div style={{ color: '#64748b', fontSize: '12px', lineHeight: 1.5 }}>{detail}</div>
                </div>
              ))}
            </div>

            <h3 style={h3Style}>Trace-to-trace clearance</h3>
            <p style={pStyle}>
              Trace spacing is measured with exact segment-to-segment distance, with a small numerical tolerance so features sitting right on a rule boundary do not produce float-drift false positives.
            </p>

            <h3 style={h3Style}>Fast queries on dense boards</h3>
            <p style={pStyle}>
              Edge-clearance compares every copper feature against the board outline. A spatial index keeps these lookups fast even on dense multi-layer boards, where a naive scan would be the bottleneck.
            </p>
          </section>

          {/* ─── CLEARANCE RULE ───────────────────────────── */}
          <SectionAnchor id="clearance" />
          <section style={{ marginBottom: '4rem' }}>
            <h2 style={h2Style}>Clearance Rule Optimisations</h2>
            <p style={pStyle}>
              Clearance is the heaviest rule — it must check every trace segment against every pad and every other trace segment. On a dense 10-layer board this can generate millions of candidate pairs. Five optimisations keep it tractable:
            </p>

            {[
              {
                title: 'Violation cap',
                body: 'Hard limit of 500 violations per run prevents OOM on pathologically dense boards (e.g. a copper pour with thousands of nearby traces). The cap preserves worst-case violations first.',
              },
              {
                title: 'Grid-based deduplication',
                body: 'After collection, violations are bucketed into a 2 mm spatial grid. Multiple trace segments of the same copper pour hitting the same pads collapse into one violation. The worst-case (smallest measured clearance) is kept along with a count.',
              },
              {
                title: 'Binary search on X-sorted pads',
                body: 'For trace-to-pad checks, pads are sorted by X coordinate. For each trace segment\'s bounding box, a binary search on X skips all pads that cannot possibly be within clearance range — eliminates the O(traces × pads) inner loop for sparse boards.',
              },
              {
                title: 'Panel filtering',
                body: 'Copper features more than 2 mm outside the board outline are ignored entirely. These are typically fiducials, tooling marks, and test coupons that appear in panel frames — not part of the actual board.',
              },
            ].map(({ title, body }, i) => (
              <Callout key={title} number={i + 1} title={title}>{body}</Callout>
            ))}
          </section>

          {/* ─── SCORING ──────────────────────────────────── */}
          <SectionAnchor id="scoring" />
          <section style={{ marginBottom: '4rem' }}>
            <h2 style={h2Style}>Scoring Algorithm</h2>
            <p style={pStyle}>
              The score (0–100) models yield impact. A board is penalised proportionally to how far each violation is from the rule limit, weighted by how consequential that rule class is to manufacturability.
            </p>

            {/* Formula breakdown */}
            <div style={{
              background: '#0d1117',
              border: '1px solid #1e2432',
              borderRadius: '12px',
              padding: '1.5rem',
              marginTop: '1.5rem',
              marginBottom: '1.5rem',
            }}>
              <div style={{ marginBottom: '1.25rem', fontSize: '13px', color: '#94a3b8', lineHeight: 1.7 }}>
                Each violation contributes a penalty that scales with the rule&apos;s severity and with how far the measured value deviates from the limit. Per-rule weighting reflects how consequential each rule class is to manufacturability. The result is normalised to a 0&ndash;100 score and a letter grade.
              </div>

              <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(180px, 1fr))', gap: '1rem' }}>
                <div>
                  <div style={{ fontSize: '11px', color: '#94a3b8', fontFamily: 'ui-monospace, monospace', marginBottom: '6px' }}>GRADE THRESHOLDS</div>
                  {[['A', '90 – 100', '#34d399'], ['B', '75 – 90', '#4a9eff'], ['C', '60 – 75', '#fbbf24'], ['D', '40 – 60', '#fb923c'], ['F', '< 40', '#f87171']].map(([grade, range, color]) => (
                    <div key={grade} style={{ display: 'flex', alignItems: 'center', gap: '8px', padding: '3px 0' }}>
                      <span style={{
                        display: 'inline-flex',
                        alignItems: 'center',
                        justifyContent: 'center',
                        width: '24px',
                        height: '24px',
                        borderRadius: '50%',
                        background: `${color}20`,
                        border: `1px solid ${color}40`,
                        color,
                        fontWeight: 800,
                        fontSize: '12px',
                      }}>{grade}</span>
                      <span style={{ color: '#94a3b8', fontSize: '12px' }}>{range}</span>
                    </div>
                  ))}
                  <p style={{ fontSize: '11px', color: '#475569', marginTop: '0.75rem', lineHeight: 1.5 }}>
                    Per-rule caps keep any single rule from dominating the overall score.
                  </p>
                </div>
              </div>
            </div>

            <p style={pStyle}>
              Penalties are weighted so a board barely under a limit is treated very differently from one that grossly violates it, small deviations cost little, large ones cost a lot.
            </p>
          </section>

          {/* ─── ASYNC PIPELINE ───────────────────────────── */}
          <SectionAnchor id="async-pipeline" />
          <section style={{ marginBottom: '4rem' }}>
            <h2 style={h2Style}>Async Job Pipeline</h2>

            <h3 style={h3Style}>Presigned URL upload</h3>
            <p style={pStyle}>
              The API never handles file bytes. Instead, it generates a 15-minute expiring presigned S3 PUT URL — the browser uploads directly. This eliminates the API as a bandwidth bottleneck and makes large ODB++ archives (often 50–100 MB) viable without timeouts.
            </p>
            <Code lang="go">{`// Client-side upload flow
const { submissionId, presignedUrl } = await createSubmission(filename)
await fetch(presignedUrl, { method: 'PUT', body: file })   // direct S3
const { jobId } = await startAnalysis(submissionId)

// Poll until done
while (true) {
    const job = await getJob(jobId)
    if (job.status === 'DONE' || job.status === 'FAILED') break
    await sleep(2000)
}`}</Code>

            <h3 style={h3Style}>Worker goroutine pool</h3>
            <p style={pStyle}>
              The worker runs five goroutines, all consuming from a shared <code style={inlineCode}>jobs</code> channel fed by SQS long-polling (20-second wait, up to 10 messages per batch). Each goroutine processes one job at a time: fetch ODB++ from S3, parse, run rules, write the result blobs back to S3, persist score + S3 keys to PostgreSQL, mark DONE.
            </p>

            <h3 style={h3Style}>Result blobs in S3</h3>
            <p style={pStyle}>
              A single 14-layer board can contain 125,000+ pads, hundreds of thousands of trace segments, and tens of thousands of violations. Stuffing that into a PostgreSQL JSONB column blows up row size, kills replica replication latency, and forces every <code style={inlineCode}>SELECT</code> to drag megabytes through the API process. So once the worker finishes the run, it writes two blobs to S3 under <code style={inlineCode}>results/{`{jobId}`}/</code>:
            </p>
            <ul style={{ ...pStyle, paddingLeft: '1.5rem', listStyle: 'disc' }}>
              <li><code style={inlineCode}>board.json</code> — full <code style={inlineCode}>BoardData</code> (layers, traces, pads, vias, drills, polygons)</li>
              <li><code style={inlineCode}>violations.json</code> — the entire violation array from the rule run</li>
            </ul>
            <p style={pStyle}>
              PostgreSQL keeps only lightweight metadata: the score, grade, the two S3 keys (<code style={inlineCode}>board_data_key</code>, <code style={inlineCode}>violations_key</code>), and a small <code style={inlineCode}>board_outline</code> JSONB (~1 KB) used for server-side score recalculation when a user waives a violation. The <code style={inlineCode}>violations</code> table itself is retained only for ignore/waive mutations — it is no longer the source of truth for bulk delivery.
            </p>
            <p style={pStyle}>
              When the frontend calls <code style={inlineCode}>GET /jobs/:id/board</code> or <code style={inlineCode}>GET /jobs/:id/violations</code>, the API responds with <code style={inlineCode}>{`{ url: <presigned-s3-url> }`}</code> (15-minute expiry; 60 minutes for share links). The browser then fetches the blob directly from S3, bypassing the API entirely. Old jobs predating the migration still have inline JSONB <code style={inlineCode}>board_data</code>; the API transparently falls back to that path when the S3 key is empty.
            </p>

            <h3 style={h3Style}>SQS recovery loop</h3>
            <p style={pStyle}>
              SQS guarantees at-least-once delivery, but silent failures happen. A background goroutine starts after a 5-minute delay (to avoid racing with initial processing), then scans the database every 10 minutes for jobs that have been in <code style={inlineCode}>PENDING</code> status for more than 5 minutes and re-enqueues them. No ops intervention required.
            </p>

            <h3 style={h3Style}>Dev mode</h3>
            <p style={pStyle}>
              When <code style={inlineCode}>SQS_QUEUE_URL</code> is empty, the worker polls the PostgreSQL <code style={inlineCode}>analysis_jobs</code> table directly instead of SQS. No AWS account needed for local development — the entire job pipeline works end-to-end on a laptop.
            </p>
          </section>

          {/* ─── SIDECAR ──────────────────────────────────── */}
          <SectionAnchor id="sidecar" />
          <section style={{ marginBottom: '4rem' }}>
            <h2 style={h2Style}>Gerbonara Sidecar</h2>
            <p style={pStyle}>
              Parsing ODB++ files in Go would require reimplementing a complex, underspecified format. Instead, a Python FastAPI sidecar exposes a single <code style={inlineCode}>POST /parse</code> endpoint. The worker calls the sidecar with an S3 key; the sidecar downloads the file and returns a <code style={inlineCode}>BoardData</code> JSON blob.
            </p>

            <h3 style={h3Style}>ODB++ format</h3>
            <div style={{ marginTop: '1rem', marginBottom: '1.5rem' }}>
              <div style={{ background: '#111418', border: '1px solid #1e2432', borderRadius: '8px', padding: '1rem' }}>
                <div style={{ color: '#d4891a', fontWeight: 700, fontSize: '13px', marginBottom: '6px' }}>ODB++</div>
                <div style={{ color: '#64748b', fontSize: '12px', lineHeight: 1.6 }}>
                  Custom archive extractor handles double-gzip (outer gzip wrapping inner tar), plain tar, and zip. Parses feature records, symbol definitions, and component lists. Includes full netlist for same-net filtering in clearance checks.
                </div>
              </div>
            </div>

            <h3 style={h3Style}>Coordinate invariant</h3>
            <p style={pStyle}>
              Every coordinate leaving the sidecar is in millimetres — enforced as a documented invariant, not a runtime check. INCH files convert symbol dimensions from mils (× 0.0254); MM files convert from microns (× 0.001). This prevents unit errors from accumulating through the DFM engine.
            </p>

            <h3 style={h3Style}>ODB++ symbol parsing</h3>
            <Code lang="python">{`# Symbol name → (width_mm, height_mm, shape)
# Examples: donut_r50x25, rect100x50, oval80, s50, r30
def _sym_to_mm(name: str, units: str) -> tuple[float, float, str]:
    scale = 0.0254 if units == "INCH" else 0.001  # mils or microns → mm
    if name.startswith("donut_r"):
        outer, inner = name[7:].split("x")
        return float(outer) * scale, float(inner) * scale, "CIRCLE"
    if name.startswith("rect"):
        w, h = name[4:].split("x") if "x" in name else (name[4:], name[4:])
        return float(w) * scale, float(h) * scale, "RECT"
    if name.startswith("oval"):
        d = float(name[4:]) * scale
        return d, d * 1.5, "OVAL"
    # fallback: 0.1mm circle
    return 0.1, 0.1, "CIRCLE"`}</Code>

            <h3 style={h3Style}>Graceful degradation</h3>
            <p style={pStyle}>
              If the S3 download fails (no AWS credentials in dev mode), the sidecar returns a deterministic mock <code style={inlineCode}>BoardData</code> object with a representative set of traces, pads, and vias. This allows full end-to-end testing — upload → worker → rules → results — without a real PCB file or AWS access.
            </p>
          </section>

          {/* ─── ODB++ FORMAT ────────────────────────────── */}
          <SectionAnchor id="odb-format" />
          <section style={{ marginBottom: '4rem' }}>
            <h2 style={h2Style}>What an ODB++ File Actually Looks Like</h2>
            <p style={pStyle}>
              ODB++ is a directory-based format — not a single file. It arrives as a <code style={inlineCode}>.tgz</code> or <code style={inlineCode}>.zip</code> archive. Once extracted, it is a structured tree of plain-text files describing every aspect of the board: layer stack, board outline, netlists, component placements, and per-layer geometry. Here's an annotated walk-through of a real board.
            </p>

            <h3 style={h3Style}>Archive directory structure</h3>
            <Code lang="bash">{`my_board.tgz
└── my_board/                    ← job root
    ├── matrix/
    │   └── matrix               ← layer stack definitions
    ├── steps/
    │   └── pcb/                 ← step (board variant; "pcb" is the default)
    │       ├── stephdr           ← step header: UNITS=INCH or MM
    │       ├── profile           ← board outline (closed polygon)
    │       ├── netlists/
    │       │   └── cadnet/
    │       │       └── netlist   ← net names + which pads belong to which net
    │       ├── components/
    │       │   ├── comp_+_top    ← component placements, top side
    │       │   └── comp_+_bot    ← component placements, bottom side
    │       └── layers/
    │           ├── top/
    │           │   └── features  ← top copper: traces, pads, vias
    │           ├── bot/
    │           │   └── features  ← bottom copper
    │           ├── smt/
    │           │   └── features  ← top solder mask
    │           ├── smb/
    │           │   └── features  ← bottom solder mask
    │           ├── sst/
    │           │   └── features  ← top silkscreen
    │           └── drill/
    │               └── features  ← drill hits`}</Code>

            <h3 style={h3Style}>matrix/matrix — layer stack</h3>
            <p style={pStyle}>
              Each layer block declares its name, physical type, and stacking order. The parser reads <code style={inlineCode}>TYPE</code> to decide which DFM rules apply to a layer (copper gets clearance and trace-width checks; solder mask gets solder-mask-dam checks; etc.).
            </p>
            <Code lang="bash">{`LAYER {
  NAME=top
  TYPE=SIGNAL          # → COPPER in our model
  ROW=1
  CONTEXT=BOARD
  POLARITY=POSITIVE
}

LAYER {
  NAME=inner1
  TYPE=POWER_GROUND    # → COPPER (power plane)
  ROW=2
  CONTEXT=BOARD
  POLARITY=NEGATIVE
}

LAYER {
  NAME=bot
  TYPE=SIGNAL
  ROW=8
  CONTEXT=BOARD
  POLARITY=POSITIVE
}

LAYER {
  NAME=smt              # solder mask top
  TYPE=SOLDER_MASK
  ROW=9
  CONTEXT=BOARD
  POLARITY=NEGATIVE
}

LAYER {
  NAME=drill
  TYPE=DRILL
  ROW=11
  CONTEXT=BOARD
}`}</Code>

            <h3 style={h3Style}>steps/pcb/profile — board outline</h3>
            <p style={pStyle}>
              The profile file describes the board boundary as a set of contours. <code style={inlineCode}>OB</code> opens a new contour (the flag on the end is <code style={inlineCode}>I</code> for island / outer boundary, <code style={inlineCode}>H</code> for hole). <code style={inlineCode}>OS</code> adds a straight segment endpoint; <code style={inlineCode}>OC</code> adds an arc. Coordinates are in the job's native units (INCH here, so these are decimal inches).
            </p>
            <Code lang="bash">{`# Board outline — 100 mm × 80 mm rectangle (in inches: 3.937 × 3.150)
OB 0.000 0.000 I        # start outer contour at origin
OS 3.937 0.000          # bottom-right corner
OS 3.937 3.150          # top-right corner
OS 0.000 3.150          # top-left corner
OE                      # close contour back to start

# Mounting hole cutout (hole in the board)
OB 0.200 0.200 H        # start hole contour
OC 0.200 0.280 0.200 0.240 90.0 180.0   # arc: cx cy x y start_angle end_angle
OC 0.280 0.200 0.200 0.240 180.0 270.0
OC 0.280 0.280 0.200 0.240 270.0 360.0
OE`}</Code>

            <h3 style={h3Style}>layers/top/features — copper geometry</h3>
            <p style={pStyle}>
              This is the most complex file. It has two sections: a symbol table (pad shape definitions) followed by feature records (one per trace, pad, or via). The symbol table maps integer indices to named shapes. Feature records reference those indices.
            </p>
            <Code lang="bash">{`# ── Symbol table ─────────────────────────────────────────────────
# Format: $<index> <symbol_name>
# Symbol names encode shape + dimensions in mils (1 mil = 0.0254 mm)

$0 r25              # round, 25-mil diameter  → 0.635 mm circle
$1 rect60x40        # rectangle, 60×40 mils   → 1.524 × 1.016 mm
$2 oval50x80        # oval/stadium, 50×80 mils → 1.27 × 2.032 mm
$3 donut_r100x55    # via annular ring: 100-mil outer, 55-mil drill → 2.54 / 1.397 mm
$4 rect100x60       # SMD pad, 100×60 mils     → 2.54 × 1.524 mm
$5 s80              # square, 80 mils          → 2.032 mm
$6 chamf_rect120x80 # chamfered rectangle      → 3.048 × 2.032 mm

# ── Feature records ───────────────────────────────────────────────
# Line (trace):   L x1 y1 x2 y2  P|N  sym_num  [;attr=val,...]
# Pad:            P x  y  rot    P|N  sym_num  [mirror]  [;attr=val,...]
# Arc:            A cx cy sr  ea  sa  ea  P|N  sym_num

# Traces on top copper (sym $0 = 25-mil round trace)
L 1.200 0.500 1.200 1.800 P 0             # vertical trace, x=1.2", y 0.5→1.8"
L 1.200 1.800 2.400 1.800 P 0             # horizontal trace continuing right
L 0.450 0.300 0.780 0.620 P 0 ;net=GND   # diagonal trace, net attribute

# SMD pads (sym $4 = 100×60 mil rect pad)
P 1.150 0.450  0.0  P 4   ;net=VCC,comp=U1,pin=1   # pad at (1.15, 0.45), 0° rotation
P 1.250 0.450  0.0  P 4   ;net=GND,comp=U1,pin=2
P 1.350 0.450  0.0  P 1   ;net=SDA,comp=U1,pin=3   # different sym ($1 = rect60x40)

# Via (sym $3 = donut — annular ring with drill hole)
P 1.200 1.800  0.0  P 3   ;net=VCC                 # via at trace junction

# Through-hole component pin (oval pad, sym $2)
P 0.600 0.600  90.0 P 2   ;net=CLK,comp=J1,pin=1   # rotated 90°`}</Code>

            <h3 style={h3Style}>netlists/cadnet/netlist — connectivity</h3>
            <p style={pStyle}>
              The netlist maps net names to pad references. This is what enables the clearance rule to skip same-net copper — two traces on the <code style={inlineCode}>GND</code> net touching each other is intentional, not a violation.
            </p>
            <Code lang="bash">{`# Each NET block lists the pads belonging to that net
NET {
  NAME=VCC
  COMP=U1 ; PIN=1          # component U1, pin 1
  COMP=C1 ; PIN=1          # decoupling cap positive
  COMP=J1 ; PIN=1
}

NET {
  NAME=GND
  COMP=U1 ; PIN=2
  COMP=C1 ; PIN=2
  COMP=J1 ; PIN=4
}

NET {
  NAME=SDA
  COMP=U1 ; PIN=3
  COMP=U2 ; PIN=7
}`}</Code>

            <h3 style={h3Style}>What the parser extracts</h3>
            <p style={pStyle}>
              After parsing all of the above, the sidecar produces a single <code style={inlineCode}>BoardData</code> JSON blob. A medium-complexity 4-layer board typically yields numbers like these:
            </p>
            <div style={{
              display: 'grid',
              gridTemplateColumns: 'repeat(auto-fit, minmax(130px, 1fr))',
              gap: '0.75rem',
              marginTop: '1rem',
              marginBottom: '1.25rem',
            }}>
              {[
                { label: '~97 000', sub: 'trace segments' },
                { label: '~30 000', sub: 'pads' },
                { label: '~8 000', sub: 'vias' },
                { label: '~485', sub: 'outline points' },
                { label: '15', sub: 'layers' },
                { label: '~3 MB', sub: 'parsed JSON' },
              ].map(({ label, sub }) => (
                <div key={sub} style={{
                  background: '#111418',
                  border: '1px solid #1e2432',
                  borderRadius: '8px',
                  padding: '0.875rem',
                  textAlign: 'center',
                }}>
                  <div style={{ fontSize: '1.35rem', fontWeight: 800, color: '#d4891a', lineHeight: 1 }}>{label}</div>
                  <div style={{ fontSize: '11px', color: '#64748b', marginTop: '4px' }}>{sub}</div>
                </div>
              ))}
            </div>
            <p style={pStyle}>
              The DFM engine then walks this data structure — not the raw file — so every rule operates purely on typed Go structs with all coordinates already in millimetres.
            </p>
          </section>

          {/* ─── FRONTEND ─────────────────────────────────── */}
          <SectionAnchor id="frontend" />
          <section style={{ marginBottom: '4rem' }}>
            <h2 style={h2Style}>Frontend</h2>
            <p style={pStyle}>
              The frontend is a Next.js 14 App Router application. All state is managed with React hooks and <code style={inlineCode}>localStorage</code> — no Redux, no Zustand.
            </p>

            <h3 style={h3Style}>BoardViewer</h3>
            <p style={pStyle}>
              The board visualiser is split into two modules: a pure <code style={inlineCode}>boardPainter.ts</code> (testable, no DOM dependencies) and an impure <code style={inlineCode}>canvasRenderer.ts</code> that owns the canvas context. The viewer renders directly to SVG with pan/zoom state, making it straightforward to implement compare mode — two viewers sharing a synchronised transform via a callback.
            </p>
            <p style={pStyle}>
              The board geometry and violations the viewer needs are not embedded in the API response. <code style={inlineCode}>getBoardData()</code> and <code style={inlineCode}>getViolations()</code> in <code style={inlineCode}>src/lib/api.ts</code> hit the API, detect the <code style={inlineCode}>{`{ url: ... }`}</code> envelope, and follow the presigned URL straight to S3. Result: the API process never has to read or stream multi-megabyte blobs, and a busy results page produces almost no API load at all.
            </p>

            <div style={{
              display: 'grid',
              gridTemplateColumns: 'repeat(auto-fit, minmax(120px, 1fr))',
              gap: '0.75rem',
              marginTop: '1rem',
              marginBottom: '1.5rem',
            }}>
              {[
                { label: 'Top copper', color: '#f0a830' },
                { label: 'Bottom copper', color: '#60b8f0' },
                { label: 'Silkscreen', color: '#f0e8d8' },
                { label: 'Solder mask', color: '#00dd66' },
                { label: 'ERROR overlay', color: '#ff3333' },
                { label: 'WARNING overlay', color: '#ff6b00' },
                { label: 'INFO overlay', color: '#44aaff' },
              ].map(({ label, color }) => (
                <div key={label} style={{
                  display: 'flex',
                  alignItems: 'center',
                  gap: '8px',
                  background: '#111418',
                  border: '1px solid #1e2432',
                  borderRadius: '6px',
                  padding: '8px 10px',
                }}>
                  <div style={{
                    width: '12px',
                    height: '12px',
                    borderRadius: '3px',
                    background: color,
                    flexShrink: 0,
                  }} />
                  <span style={{ fontSize: '11px', color: '#64748b', lineHeight: 1.3 }}>{label}</span>
                </div>
              ))}
            </div>

            <p style={pStyle}>
              Layer type is inferred from layer metadata in the ODB++ matrix and filename heuristics. External transform sync enables the compare view where two boards pan/zoom in lockstep — useful for diffing before/after a revision.
            </p>

            <h3 style={h3Style}>Auth</h3>
            <p style={pStyle}>
              Auth is via AWS Cognito (OIDC + JWT). Production builds validate every request against Cognito; the GovCloud edition additionally enforces multi-factor authentication and US-person provisioning. A separate local development mode, excluded from production builds, lets engineers run the full stack without an AWS account.
            </p>
          </section>

          {/* ─── DESIGN DECISIONS ─────────────────────────── */}
          <SectionAnchor id="design-decisions" />
          <section style={{ marginBottom: '4rem' }}>
            <h2 style={h2Style}>Production Design Decisions</h2>
            <p style={{ ...pStyle, marginBottom: '1.5rem' }}>
              Non-obvious choices that shaped the architecture:
            </p>
            {[
              {
                title: 'Presigned URL delegation',
                body: 'The API never handles file bytes. The bandwidth cost of large ODB++ files (50–100 MB) would be prohibitive through an App Runner instance with limited egress. Presigned URLs push the transfer directly to S3.',
              },
              {
                title: 'Result blobs in S3, not PostgreSQL',
                body: 'A 14-layer board produces board.json and violations.json blobs in the multi-MB range. Storing them as JSONB columns blew up row size, hurt replica replication, and forced the API to drag megabytes through every read. The worker now writes both blobs to results/{jobId}/ in S3 and the database keeps only the S3 keys plus the score. The API responds to GET /jobs/:id/board with {url: presigned} and the browser fetches directly — the API process never sees the bytes. Inline JSONB is kept as a fallback for jobs predating the migration.',
              },
              {
                title: 'Numerical tolerance at rule boundaries',
                body: 'Floating-point arithmetic on board coordinates can make features sitting exactly on a rule limit register as violations. A small tolerance eliminates this class of false positive without any meaningful measurement error at PCB scale.',
              },
              {
                title: 'Bounded, normalised scoring',
                body: 'The manufacturability score is normalised so that no single rule class can dominate or auto-fail the result. A board with many hits of one rule type still receives a fair, actionable grade rather than a misleading zero.',
              },
              {
                title: 'Panel-level copper filtering',
                body: 'Copper more than 2 mm outside the board outline is fiducials, test coupons, or tooling marks in the panel frame. Including them in DFM checks would flag every panelised design. A single-line bounding-box check removes the class entirely.',
              },
              {
                title: 'SQS recovery loop with startup delay',
                body: 'The 5-minute startup delay prevents the recovery goroutine from racing with normal processing on a fresh deploy — a new job that just entered PENDING state isn\'t stuck, it\'s just queued. The 5-minute staleness threshold is longer than any normal job, shorter than anything a customer would tolerate.',
              },
              {
                title: 'Go replace directive for shared engine',
                body: 'The DFM engine is imported by both the API and the worker via a local replace directive in their go.mod files. This avoids publishing a versioned module while the engine is still evolving rapidly. Both services always use the same engine version from the monorepo.',
              },
              {
                title: 'Concurrent rule execution',
                body: 'All 22 rules receive the same read-only BoardData struct. No synchronisation is needed inside individual rules. The WaitGroup pattern keeps the runner simple and the per-rule code free of concurrency concerns.',
              },
            ].map(({ title, body }, i) => (
              <Callout key={title} number={i + 1} title={title}>{body}</Callout>
            ))}
          </section>

          {/* ─── CI/CD ────────────────────────────────────── */}
          <SectionAnchor id="cicd" />
          <section style={{ marginBottom: '4rem' }}>
            <h2 style={h2Style}>CI / CD</h2>
            <p style={pStyle}>
              GitHub Actions runs two pipelines: one for CI (tests + builds on every push/PR), one for deployment (path-filtered, only rebuilds changed services).
            </p>

            <div className="tech-cicd-grid" style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '1rem', marginTop: '1.25rem' }}>
              <div style={{ background: '#111418', border: '1px solid #1e2432', borderRadius: '8px', padding: '1.25rem' }}>
                <div style={{ fontSize: '12px', fontWeight: 700, color: '#e2e8f0', marginBottom: '0.75rem' }}>CI Pipeline</div>
                {[
                  'Go format check (gofmt -l)',
                  'Engine tests (go test ./...)',
                  'Worker build (go build)',
                  'API build (go build)',
                  'Sidecar tests (pytest)',
                  'Frontend build + tests (vitest)',
                ].map((step) => (
                  <div key={step} style={{ display: 'flex', gap: '8px', alignItems: 'flex-start', marginBottom: '6px' }}>
                    <span style={{ color: '#34d399', marginTop: '1px', flexShrink: 0 }}>✓</span>
                    <span style={{ fontSize: '12px', color: '#64748b' }}>{step}</span>
                  </div>
                ))}
              </div>
              <div style={{ background: '#111418', border: '1px solid #1e2432', borderRadius: '8px', padding: '1.25rem' }}>
                <div style={{ fontSize: '12px', fontWeight: 700, color: '#e2e8f0', marginBottom: '0.75rem' }}>Deploy Pipeline</div>
                {[
                  ['Worker + sidecar changed', '→ ECS (built together to prevent race condition)'],
                  ['API changed', '→ App Runner'],
                  ['Web changed', '→ Vercel (auto-deploy)'],
                ].map(([trigger, action]) => (
                  <div key={trigger} style={{ marginBottom: '8px' }}>
                    <div style={{ fontSize: '11px', color: '#d4891a', fontFamily: 'ui-monospace, monospace' }}>{trigger}</div>
                    <div style={{ fontSize: '12px', color: '#64748b' }}>{action}</div>
                  </div>
                ))}
                <p style={{ fontSize: '11px', color: '#475569', marginTop: '0.75rem', lineHeight: 1.5 }}>
                  Worker and gerbonara are deployed together: the worker calls the sidecar at startup. A version mismatch between the two would produce parsing errors, so they always roll together.
                </p>
              </div>
            </div>
          </section>

          {/* Footer */}
          <div style={{
            borderTop: '1px solid #1e2432',
            paddingTop: '2rem',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            flexWrap: 'wrap',
            gap: '1rem',
          }}>
            <div>
              <div style={{ fontWeight: 700, color: '#e2e8f0', marginBottom: '4px' }}>RapidDFM</div>
              <div style={{ fontSize: '12px', color: '#475569' }}>PCB DFM analysis platform · Built by Basel Anani</div>
            </div>
            <a
              href="/"
              style={{
                fontSize: '13px',
                color: '#4a9eff',
                textDecoration: 'none',
                padding: '6px 14px',
                border: '1px solid rgba(74,158,255,0.3)',
                borderRadius: '6px',
              }}
            >
              ← Back to RapidDFM
            </a>
          </div>
        </main>
      </div>

      <style>{`
        @media (min-width: 768px) {
          .tech-toc { display: block !important; }
          .tech-layout { grid-template-columns: 220px 1fr !important; }
        }
        @media (max-width: 767px) {
          .tech-hero { padding: 2.5rem 1.25rem 2rem !important; }
          .tech-arch-diagram { padding: 1.25rem 1rem !important; }
          .tech-cicd-grid { grid-template-columns: 1fr !important; }
          .tech-main { padding: 2rem 0 4rem !important; }
        }
      `}</style>
    </div>
  )
}

// ─── Style constants ───────────────────────────────────────────────────────────

const h2Style: React.CSSProperties = {
  fontSize: '1.4rem',
  fontWeight: 700,
  letterSpacing: '-0.02em',
  color: '#e2e8f0',
  marginBottom: '0.875rem',
  paddingBottom: '0.625rem',
  borderBottom: '1px solid #1e2432',
}

const h3Style: React.CSSProperties = {
  fontSize: '1rem',
  fontWeight: 600,
  color: '#cbd5e1',
  marginTop: '1.5rem',
  marginBottom: '0.5rem',
}

const pStyle: React.CSSProperties = {
  color: '#94a3b8',
  fontSize: '14.5px',
  lineHeight: 1.75,
  marginBottom: '1rem',
}

const inlineCode: React.CSSProperties = {
  fontFamily: 'ui-monospace, "Cascadia Code", "Source Code Pro", Menlo, monospace',
  fontSize: '12.5px',
  background: '#111418',
  border: '1px solid #1e2432',
  color: '#d4891a',
  padding: '1px 6px',
  borderRadius: '4px',
}

const thStyle: React.CSSProperties = {
  padding: '10px 12px',
  textAlign: 'left',
  fontSize: '11px',
  fontWeight: 700,
  letterSpacing: '0.06em',
  color: '#475569',
  textTransform: 'uppercase',
}

const tdStyle: React.CSSProperties = {
  padding: '9px 12px',
  fontSize: '13px',
  color: '#e2e8f0',
  verticalAlign: 'top',
}

// ─── Small layout components ───────────────────────────────────────────────────

function ServiceBox({ label, sub, color }: { label: string; sub: string; color: string }) {
  return (
    <div style={{
      background: `${color}10`,
      border: `1px solid ${color}35`,
      borderRadius: '8px',
      padding: '8px 12px',
      minWidth: '120px',
      flexShrink: 0,
    }}>
      <div style={{ fontSize: '13px', fontWeight: 700, color }}>{label}</div>
      <div style={{ fontSize: '10px', color: '#475569', marginTop: '2px', fontFamily: 'ui-monospace, monospace' }}>{sub}</div>
    </div>
  )
}

function Arrow({ label, dir = 'right' }: { label: string; dir?: 'right' | 'left' }) {
  return (
    <div style={{
      display: 'flex',
      flexDirection: 'column',
      alignItems: 'center',
      gap: '2px',
      flexShrink: 0,
    }}>
      <div style={{ fontSize: '10px', color: '#475569', fontFamily: 'ui-monospace, monospace', whiteSpace: 'nowrap' }}>{label}</div>
      <div style={{ color: '#334155', fontSize: '16px' }}>{dir === 'right' ? '→' : '←'}</div>
    </div>
  )
}
