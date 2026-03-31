'use client'

import { useState } from 'react'
import { Upload, Shield, Share2, BarChart3, Layers, Zap } from 'lucide-react'
import { cn } from '@/lib/utils'
import { track } from '@/lib/analytics'

const FEATURES = [
  {
    icon: Upload,
    title: 'Replace Manual File Inspection',
    desc: 'Drop a Gerber or ODB++ file and get a scored DFM report in seconds. No more opening layers manually to inspect traces and clearances.',
  },
  {
    icon: Layers,
    title: 'Pinpoint Issues Instantly',
    desc: 'Click any violation and the board viewer zooms to the exact location. Toggle layers, filter by rule, and resolve issues in minutes instead of hours.',
  },
  {
    icon: Share2,
    title: 'Eliminate Email Back-and-Forth',
    desc: 'Send a branded share link to your customer. They see exactly what needs fixing, upload a revised design, and you track the improvement — no screenshots or phone calls.',
  },
  {
    icon: BarChart3,
    title: 'Standardize Your DFM Output',
    desc: 'Every design gets the same 16-rule check and a 0-100 manufacturability score. Export to PDF for a consistent, professional report — regardless of which engineer reviews it.',
  },
  {
    icon: Shield,
    title: 'Encode Your Shop Capabilities',
    desc: 'Define manufacturing lines with your exact process limits — trace widths, drill sizes, clearances. Different profiles for FR4, HDI, flex. Your rules, applied automatically.',
  },
  {
    icon: Zap,
    title: 'Process a Week of Designs in Minutes',
    desc: 'Upload an entire batch of incoming jobs at once. Parallel analysis with per-file tracking. Screen dozens of designs before they reach your engineering queue.',
  },
]

export function LandingFeatures() {
  const [expanded, setExpanded] = useState<number | null>(null)

  return (
    <div className="grid sm:grid-cols-2 lg:grid-cols-3 gap-5">
      {FEATURES.map((f, i) => {
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
