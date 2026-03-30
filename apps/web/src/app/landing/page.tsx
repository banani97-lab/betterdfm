import Link from 'next/link'
import Image from 'next/image'
import logoDark from '@/app/dashboard/RapidDFM Dark Mode Logo New.png'
import { CheckCircle, Upload, Shield, Share2, BarChart3, Layers, Zap, ArrowRight } from 'lucide-react'
import { LandingAnalytics } from './LandingAnalytics'
import { ContactForm } from './ContactForm'

const PORTAL_URL = 'https://portal.rapiddfm.com'

const RULES = [
  { name: 'Trace Width', desc: 'Catch underwidth traces before they cause opens' },
  { name: 'Clearance', desc: 'Flag spacing violations that lead to shorts' },
  { name: 'Drill Size', desc: 'Verify hole diameters match your drill inventory' },
  { name: 'Annular Ring', desc: 'Ensure via pads meet registration tolerances' },
  { name: 'Aspect Ratio', desc: 'Prevent drill breakage from high aspect holes' },
  { name: 'Edge Clearance', desc: 'Avoid copper exposure at board edges after routing' },
  { name: 'Solder Mask Dam', desc: 'Prevent solder bridging between tight-pitch pads' },
  { name: 'Copper Sliver', desc: 'Detect thin copper features that flake during etch' },
  { name: 'Silkscreen on Pad', desc: 'Stop ink contamination on solderable surfaces' },
  { name: 'Drill-to-Drill', desc: 'Maintain spacing that prevents wall blowout' },
  { name: 'Drill-to-Copper', desc: 'Protect trace integrity near drilled holes' },
  { name: 'Trace Imbalance', desc: 'Flag thermal asymmetry that causes tombstoning' },
  { name: 'Tombstoning Risk', desc: 'Detect uneven pads on small passives' },
  { name: 'Package Capability', desc: 'Verify component sizes match your placement capability' },
  { name: 'Fiducial Count', desc: 'Confirm alignment marks for pick-and-place' },
  { name: 'Pad Size', desc: 'Validate pad geometry against IPC land patterns' },
]

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

export default function LandingPage() {
  return (
    <div className="min-h-screen bg-gradient-to-b from-slate-950 via-slate-900 to-slate-950 text-white">
      <LandingAnalytics />
      {/* Structured data for SEO */}
      <script
        type="application/ld+json"
        dangerouslySetInnerHTML={{
          __html: JSON.stringify({
            '@context': 'https://schema.org',
            '@type': 'SoftwareApplication',
            name: 'RapidDFM',
            applicationCategory: 'BusinessApplication',
            operatingSystem: 'Web',
            description: 'Automated PCB design-for-manufacturability analysis platform for contract manufacturers.',
            offers: {
              '@type': 'Offer',
              priceCurrency: 'USD',
              price: '0',
              description: 'Contact for pricing',
            },
            creator: {
              '@type': 'Organization',
              name: 'Saturn Solutions',
              url: 'https://www.rapiddfm.com',
            },
          }),
        }}
      />

      {/* Nav */}
      <nav className="sticky top-0 z-50 border-b border-white/10 bg-slate-950/80 backdrop-blur-md">
        <div className="max-w-6xl mx-auto px-6 h-16 flex items-center justify-between">
          <Image src={logoDark} alt="RapidDFM" className="h-10 w-auto" priority />
          <div className="flex items-center gap-4">
            <a href="#features" className="text-sm text-slate-300 hover:text-white transition-colors hidden sm:block">Features</a>
            <a href="#rules" className="text-sm text-slate-300 hover:text-white transition-colors hidden sm:block">DFM Rules</a>
            <a href="#pricing" className="text-sm text-slate-300 hover:text-white transition-colors hidden sm:block">Pricing</a>
            <a href="#contact" className="text-sm text-slate-300 hover:text-white transition-colors hidden sm:block">Contact</a>
            <Link
              href={`${PORTAL_URL}/login`}
              className="text-sm font-medium px-4 py-2 rounded-lg bg-blue-600 hover:bg-blue-500 transition-colors"
              data-track-click="nav-sign-in"
            >
              Sign In
            </Link>
          </div>
        </div>
      </nav>

      {/* Hero */}
      <section data-section="hero" className="max-w-6xl mx-auto px-6 pt-24 pb-20 text-center">
        <p className="text-sm font-medium text-blue-400 mb-4 tracking-wide">Built for real PCB manufacturing workflows</p>
        <h1 className="text-4xl sm:text-5xl md:text-6xl font-bold tracking-tight leading-tight">
          Screen Incoming Designs
          <br />
          <span className="text-blue-400">Before They Hit Your Engineers</span>
        </h1>
        <p className="mt-6 text-lg sm:text-xl text-slate-300 max-w-2xl mx-auto leading-relaxed">
          RapidDFM replaces manual CAM review with automated manufacturability checks that run in seconds. Upload Gerber or ODB++ files, apply your shop&apos;s rules, and share results with customers — cutting first-pass review time and eliminating revision back-and-forth.
        </p>
        <div className="mt-10 flex flex-col sm:flex-row items-center justify-center gap-4">
          <a
            href="#contact"
            className="inline-flex items-center gap-2 px-8 py-3.5 rounded-xl bg-blue-600 hover:bg-blue-500 text-white font-semibold text-lg transition-colors shadow-lg shadow-blue-600/20"
            data-track-click="hero-get-started"
          >
            Start Analyzing Designs <ArrowRight className="h-5 w-5" />
          </a>
          <a
            href="#features"
            className="inline-flex items-center gap-2 px-8 py-3.5 rounded-xl border border-white/20 hover:border-white/40 text-white font-medium text-lg transition-colors"
            data-track-click="hero-see-how"
          >
            See How It Works
          </a>
        </div>
        <p className="mt-6 text-sm text-slate-500">Start analyzing designs in minutes — no complex setup required.</p>
      </section>

      {/* Stats strip */}
      <section data-section="stats" className="border-y border-white/10 bg-white/[0.02]">
        <div className="max-w-6xl mx-auto px-6 py-8 flex flex-wrap items-center justify-center gap-x-12 gap-y-4 text-center">
          <div>
            <div className="text-3xl font-bold text-blue-400">16</div>
            <div className="text-sm text-slate-400">Automated Checks</div>
          </div>
          <div>
            <div className="text-3xl font-bold text-blue-400">&lt;30s</div>
            <div className="text-sm text-slate-400">Per Design</div>
          </div>
          <div>
            <div className="text-3xl font-bold text-blue-400">Gerber + ODB++</div>
            <div className="text-sm text-slate-400">Native Support</div>
          </div>
          <div>
            <div className="text-3xl font-bold text-blue-400">0-100</div>
            <div className="text-sm text-slate-400">Manufacturability Score</div>
          </div>
        </div>
      </section>

      {/* Features */}
      <section id="features" data-section="features" className="max-w-6xl mx-auto px-6 py-24">
        <h2 className="text-3xl sm:text-4xl font-bold text-center mb-4">
          Cut First-Pass Review Time. Standardize DFM Across Your Team.
        </h2>
        <p className="text-slate-400 text-center max-w-2xl mx-auto mb-16">
          Built for real PCB manufacturing workflows. Every feature replaces a manual step your engineers are doing today.
        </p>
        <div className="grid sm:grid-cols-2 lg:grid-cols-3 gap-8">
          {FEATURES.map((f) => (
            <div key={f.title} className="rounded-2xl border border-white/10 bg-white/[0.03] p-6 hover:border-blue-500/30 transition-colors">
              <f.icon className="h-8 w-8 text-blue-400 mb-4" />
              <h3 className="text-lg font-semibold mb-2">{f.title}</h3>
              <p className="text-sm text-slate-400 leading-relaxed">{f.desc}</p>
            </div>
          ))}
        </div>
      </section>

      {/* How it works */}
      <section data-section="how-it-works" className="border-y border-white/10 bg-white/[0.02]">
        <div className="max-w-6xl mx-auto px-6 py-24">
          <h2 className="text-3xl sm:text-4xl font-bold text-center mb-16">From File Upload to Customer-Ready Report in Under a Minute</h2>
          <div className="grid sm:grid-cols-4 gap-8 text-center">
            {[
              { step: '1', title: 'Upload', desc: 'Drop Gerber or ODB++ files — no manual extraction or file prep needed' },
              { step: '2', title: 'Apply Rules', desc: 'Your manufacturing limits are checked automatically against the design' },
              { step: '3', title: 'Review Results', desc: 'See every violation scored, mapped to the board, and ready to act on' },
              { step: '4', title: 'Share & Resolve', desc: 'Send results to your customer and track revisions until the design is fab-ready' },
            ].map((s) => (
              <div key={s.step}>
                <div className="inline-flex items-center justify-center w-12 h-12 rounded-full bg-blue-600/20 text-blue-400 font-bold text-lg mb-4">
                  {s.step}
                </div>
                <h3 className="font-semibold text-lg mb-1">{s.title}</h3>
                <p className="text-sm text-slate-400">{s.desc}</p>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* DFM Rules */}
      <section id="rules" data-section="rules" className="max-w-6xl mx-auto px-6 py-24">
        <h2 className="text-3xl sm:text-4xl font-bold text-center mb-4">
          16 Checks Your Engineers Run Manually. Automated.
        </h2>
        <p className="text-slate-400 text-center max-w-2xl mx-auto mb-16">
          Every design gets the same checks, every time, regardless of who reviews it. No more missed issues from rushed reviews or inconsistent standards between shifts.
        </p>
        <div className="grid sm:grid-cols-2 lg:grid-cols-4 gap-4">
          {RULES.map((r) => (
            <div key={r.name} className="flex items-start gap-3 rounded-xl border border-white/10 bg-white/[0.03] p-4">
              <CheckCircle className="h-5 w-5 text-green-400 mt-0.5 shrink-0" />
              <div>
                <div className="font-medium text-sm">{r.name}</div>
                <div className="text-xs text-slate-500">{r.desc}</div>
              </div>
            </div>
          ))}
        </div>
      </section>

      {/* Pricing */}
      <section id="pricing" data-section="pricing" className="border-y border-white/10 bg-white/[0.02]">
        <div className="max-w-6xl mx-auto px-6 py-24">
          <h2 className="text-3xl sm:text-4xl font-bold text-center mb-4">
            Pricing That Scales With Your Shop
          </h2>
          <p className="text-slate-400 text-center max-w-xl mx-auto mb-16">
            Model your manufacturing capabilities and catch issues before they reach the shop floor.
          </p>
          <div className="grid sm:grid-cols-3 gap-8 max-w-4xl mx-auto">
            {[
              {
                name: 'Starter',
                price: '$799',
                period: '/mo',
                subtitle: 'Evaluate and validate before full adoption',
                features: [
                  'Up to 30 design analyses per month',
                  'Unlimited team access',
                  '1 manufacturing line (capability profile)',
                  'PDF & CSV export',
                  'Interactive board viewer',
                ],
                roi: 'Ideal for single-process shops or initial evaluation',
                cta: 'Get Started',
                highlight: false,
              },
              {
                name: 'Professional',
                price: '$2,499',
                period: '/mo',
                subtitle: 'Automate first-pass DFM and reduce manual CAM review time',
                features: [
                  'Up to 250 design analyses per month',
                  'Unlimited team access',
                  'Up to 5 manufacturing lines (capability profiles)',
                  'Customer portal with branded share links',
                  'Bulk design processing',
                  'Revision-to-revision manufacturability tracking',
                ],
                roi: 'Replace manual first-pass review across your production lines',
                cta: 'Get Started',
                highlight: true,
              },
              {
                name: 'Enterprise',
                price: '',
                period: '',
                subtitle: 'Automatically screen incoming jobs before they reach your engineering team',
                features: [
                  'Scaled for continuous intake workflows',
                  'Unlimited team access',
                  'Custom manufacturing lines',
                  'Everything in Professional',
                  'API access & automated intake',
                  'Admin dashboard & analytics',
                  'Priority processing',
                  'Dedicated onboarding',
                ],
                roi: 'Built for multi-site operations and automated job intake',
                cta: 'Talk to Sales',
                highlight: false,
              },
            ].map((tier) => (
              <div
                key={tier.name}
                className={`rounded-2xl border p-8 flex flex-col ${
                  tier.highlight
                    ? 'border-blue-500 bg-blue-600/10 ring-1 ring-blue-500/30'
                    : 'border-white/10 bg-white/[0.03]'
                }`}
              >
                <h3 className="text-lg font-semibold">{tier.name}</h3>
                <p className="text-xs text-slate-400 mt-1">{tier.subtitle}</p>
                <div className="mt-4 mb-2">
                  {tier.price ? (
                    <>
                      <span className="text-4xl font-bold">{tier.price}</span>
                      <span className="text-slate-400">{tier.period}</span>
                    </>
                  ) : (
                    <span className="text-2xl font-bold text-slate-300">Custom Pricing</span>
                  )}
                </div>
                <p className="text-xs text-blue-400 mb-6">{tier.roi}</p>
                <ul className="flex-1 space-y-3 mb-8">
                  {tier.features.map((f) => (
                    <li key={f} className="flex items-center gap-2 text-sm text-slate-300">
                      <CheckCircle className="h-4 w-4 text-green-400 shrink-0" />
                      {f}
                    </li>
                  ))}
                </ul>
                <a
                  href="#contact"
                  data-track-click={`pricing-${tier.name.toLowerCase()}`}
                  className={`block text-center py-3 rounded-xl font-semibold transition-colors ${
                    tier.highlight
                      ? 'bg-blue-600 hover:bg-blue-500 text-white'
                      : 'border border-white/20 hover:border-white/40 text-white'
                  }`}
                >
                  {tier.cta}
                </a>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* Contact */}
      <section id="contact" data-section="contact" className="max-w-6xl mx-auto px-6 py-24">
        <div className="max-w-xl mx-auto">
          <h2 className="text-3xl sm:text-4xl font-bold text-center mb-4">
            See It On Your Own Designs
          </h2>
          <p className="text-slate-400 text-center mb-10">
            Tell us about your shop and we&apos;ll walk you through RapidDFM with your own board files. No generic demos — we&apos;ll show you what it catches on designs you&apos;ve already built.
          </p>
          <ContactForm />
        </div>
      </section>

      {/* Footer */}
      <footer className="border-t border-white/10">
        <div className="max-w-6xl mx-auto px-6 py-8 flex flex-col sm:flex-row items-center justify-between gap-4">
          <div className="text-sm text-slate-500">
            &copy; {new Date().getFullYear()} Saturn Solutions. All rights reserved.
          </div>
          <div className="flex items-center gap-6 text-sm text-slate-500">
            <a href="#features" className="hover:text-white transition-colors">Features</a>
            <a href="#rules" className="hover:text-white transition-colors">Rules</a>
            <a href="#pricing" className="hover:text-white transition-colors">Pricing</a>
            <a href="#contact" className="hover:text-white transition-colors">Contact</a>
            <Link href={`${PORTAL_URL}/login`} className="hover:text-white transition-colors">Sign In</Link>
          </div>
        </div>
      </footer>
    </div>
  )
}
