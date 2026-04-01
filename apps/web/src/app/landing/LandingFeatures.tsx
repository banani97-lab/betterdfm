'use client'

import { useState } from 'react'
import { Search, Eye, Shield, Share2 } from 'lucide-react'
import { cn } from '@/lib/utils'
import { track } from '@/lib/analytics'

const FEATURE_GROUPS = [
  {
    icon: Search,
    title: 'Catch Issues Early',
    desc: 'Trace width, clearance, annular ring, drill size, solder mask dam, edge clearance, aspect ratio — 16 checks that catch fab-killing issues before your engineers open a CAM tool.',
  },
  {
    icon: Eye,
    title: 'See Exactly What\u2019s Wrong',
    desc: 'Every violation is mapped to the exact board location. Click to zoom in. Toggle layers. Filter by rule or severity. Resolve issues in minutes, not hours of manual layer inspection.',
  },
  {
    icon: Shield,
    title: 'Standardize Your Process',
    desc: 'Define capability profiles with your exact manufacturing limits. Different profiles for different lines. Every design gets the same checks — day shift or night shift, junior or senior.',
  },
  {
    icon: Share2,
    title: 'Communicate Clearly',
    desc: 'Send your customer a branded link with every violation highlighted. They upload a revision, you track the score improvement. No more email threads with annotated screenshots.',
  },
]

export function LandingFeatures() {
  const [expanded, setExpanded] = useState<number | null>(null)

  return (
    <div className="grid sm:grid-cols-2 gap-5 max-w-4xl mx-auto">
      {FEATURE_GROUPS.map((f, i) => {
        const isOpen = expanded === i
        return (
          <div
            key={f.title}
            className={cn(
              'rounded-2xl border bg-white/[0.03] p-6 cursor-pointer transition-all duration-200',
              isOpen
                ? 'border-[#1565c0]/50 bg-[#1565c0]/5'
                : 'border-white/10 hover:border-white/20'
            )}
            onMouseEnter={() => { setExpanded(i); track('Feature Expanded', { feature: f.title }) }}
            onMouseLeave={() => setExpanded(null)}
            onClick={() => { setExpanded(isOpen ? null : i); if (!isOpen) track('Feature Expanded', { feature: f.title }) }}
          >
            <f.icon className="h-7 w-7 text-[#4fc3f7] mb-3" />
            <h3 className="text-base font-semibold">{f.title}</h3>
            <div
              className={cn(
                'overflow-hidden transition-all duration-200',
                isOpen ? 'max-h-40 opacity-100 mt-2' : 'max-h-0 opacity-0 mt-0'
              )}
            >
              <p className="text-sm text-slate-400 leading-relaxed">{f.desc}</p>
            </div>
          </div>
        )
      })}
    </div>
  )
}
