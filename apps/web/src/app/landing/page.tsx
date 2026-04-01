import Link from 'next/link'
import Image from 'next/image'
import logoDark from '@/app/dashboard/RapidDFM Dark Mode Logo New.png'
import { ArrowRight, X, Check } from 'lucide-react'
import { LandingAnalytics } from './LandingAnalytics'
import { LandingFeatures } from './LandingFeatures'
import { ContactForm } from './ContactForm'

const PORTAL_URL = 'https://portal.rapiddfm.com'

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
          <a href="/" aria-label="RapidDFM home">
            <Image src={logoDark} alt="RapidDFM" className="h-10 w-auto" priority />
          </a>
          <div className="flex items-center gap-4">
            <Link
              href={`${PORTAL_URL}/login`}
              className="text-sm font-medium px-4 py-2 rounded-lg bg-[#1565c0] hover:bg-[#1976d2] transition-colors"
              data-track-click="nav-sign-in"
            >
              Sign In
            </Link>
          </div>
        </div>
      </nav>

      {/* ═══════════════════════════════════════════════════════════════════════ */}
      {/* HERO                                                                  */}
      {/* ═══════════════════════════════════════════════════════════════════════ */}
      <section data-section="hero" className="max-w-6xl mx-auto px-6 pt-24 pb-20 text-center">
        <h1 className="text-4xl sm:text-5xl md:text-6xl font-bold tracking-tight leading-tight">
          Stop Reviewing Bad Designs.
          <br />
          <span className="text-[#4fc3f7]">Start Screening Them Automatically.</span>
        </h1>
        <p className="mt-6 text-lg sm:text-xl text-slate-300 max-w-2xl mx-auto leading-relaxed">
          RapidDFM runs 16 manufacturability checks on every incoming Gerber and ODB++ file in under 30 seconds — before your CAM team ever opens them. Reduce first-pass review time by up to 80%, standardize quoting decisions, and eliminate revision loops with customers.
        </p>
        <div className="mt-10 flex flex-col sm:flex-row items-center justify-center gap-4">
          <a
            href="#contact"
            className="inline-flex items-center gap-2 px-8 py-3.5 rounded-xl bg-[#1565c0] hover:bg-[#1976d2] text-white font-semibold text-lg transition-colors shadow-lg shadow-[#1565c0]/20"
            data-track-click="hero-get-demo"
          >
            Get a Demo <ArrowRight className="h-5 w-5" />
          </a>
          <a
            href="#how-it-works"
            className="inline-flex items-center gap-2 px-8 py-3.5 rounded-xl border border-white/20 hover:border-white/40 text-white font-medium text-lg transition-colors"
            data-track-click="hero-see-how"
          >
            See How It Works
          </a>
        </div>
        <p className="mt-6 text-sm text-slate-400">Works in your browser. No installation. Gerber + ODB++ native support.</p>
      </section>

      {/* Stats strip */}
      <section data-section="stats" className="border-y border-white/10 bg-white/[0.02]">
        <div className="max-w-6xl mx-auto px-6 py-8 flex flex-wrap items-center justify-center gap-x-12 gap-y-4 text-center">
          <div>
            <div className="text-3xl font-bold text-[#4fc3f7]">16</div>
            <div className="text-sm text-slate-400">Automated DFM Checks</div>
          </div>
          <div>
            <div className="text-3xl font-bold text-[#4fc3f7]">&lt;30s</div>
            <div className="text-sm text-slate-400">Per Design</div>
          </div>
          <div>
            <div className="text-3xl font-bold text-[#4fc3f7]">80%</div>
            <div className="text-sm text-slate-400">Less Manual Review</div>
          </div>
          <div>
            <div className="text-3xl font-bold text-[#4fc3f7]">0-100</div>
            <div className="text-sm text-slate-400">Manufacturability Score</div>
          </div>
        </div>
      </section>

      {/* ═══════════════════════════════════════════════════════════════════════ */}
      {/* PROBLEM                                                               */}
      {/* ═══════════════════════════════════════════════════════════════════════ */}
      <section data-section="problem" className="max-w-6xl mx-auto px-6 py-24">
        <h2 className="text-3xl sm:text-4xl font-bold text-center mb-4">
          Your CAM Team Shouldn&apos;t Be a Filter
        </h2>
        <p className="text-slate-400 text-center max-w-2xl mx-auto mb-16">
          Every board that hits your inbox costs engineering time — whether it&apos;s manufacturable or not.
        </p>
        <div className="grid sm:grid-cols-2 gap-8 max-w-4xl mx-auto">
          <div className="rounded-2xl border border-red-500/20 bg-red-500/5 p-6">
            <h3 className="text-lg font-semibold text-red-400 mb-4">The reality today</h3>
            <ul className="space-y-3 text-sm text-slate-300">
              <li className="flex gap-3"><X className="h-4 w-4 text-red-400 mt-0.5 shrink-0" />Engineers spend 30-60 minutes per board opening layers, checking clearances, measuring traces — manually.</li>
              <li className="flex gap-3"><X className="h-4 w-4 text-red-400 mt-0.5 shrink-0" />Two engineers review the same file and flag different issues. No consistency between shifts.</li>
              <li className="flex gap-3"><X className="h-4 w-4 text-red-400 mt-0.5 shrink-0" />You email the customer a list of problems. They fix two, break three more. Repeat for a week.</li>
              <li className="flex gap-3"><X className="h-4 w-4 text-red-400 mt-0.5 shrink-0" />RFQs pile up while your best engineers are stuck babysitting revision cycles.</li>
            </ul>
          </div>
          <div className="rounded-2xl border border-[#1565c0]/30 bg-[#1565c0]/5 p-6">
            <h3 className="text-lg font-semibold text-[#4fc3f7] mb-4">With RapidDFM</h3>
            <ul className="space-y-3 text-sm text-slate-300">
              <li className="flex gap-3"><Check className="h-4 w-4 text-[#4fc3f7] mt-0.5 shrink-0" />Every incoming design gets the same 16 checks in under 30 seconds. Before anyone opens a CAM tool.</li>
              <li className="flex gap-3"><Check className="h-4 w-4 text-[#4fc3f7] mt-0.5 shrink-0" />Same rules, same thresholds, same results — regardless of who&apos;s working. Day shift or night shift.</li>
              <li className="flex gap-3"><Check className="h-4 w-4 text-[#4fc3f7] mt-0.5 shrink-0" />Send your customer a link. They see every violation, upload a fix, and you track the improvement. No emails.</li>
              <li className="flex gap-3"><Check className="h-4 w-4 text-[#4fc3f7] mt-0.5 shrink-0" />Process more RFQs with the same team. Your engineers focus on engineering, not file screening.</li>
            </ul>
          </div>
        </div>
      </section>

      {/* ═══════════════════════════════════════════════════════════════════════ */}
      {/* SOLUTION                                                              */}
      {/* ═══════════════════════════════════════════════════════════════════════ */}
      <section data-section="solution" className="border-y border-white/10 bg-white/[0.02]">
        <div className="max-w-6xl mx-auto px-6 py-24 text-center">
          <h2 className="text-3xl sm:text-4xl font-bold mb-4">
            Automatically Screen Every Design in Seconds
          </h2>
          <p className="text-slate-400 max-w-2xl mx-auto mb-6">
            Upload a file. Get a scored manufacturability report. Share it with your customer. Done.
          </p>
          <p className="text-slate-400 max-w-xl mx-auto text-sm">
            RapidDFM applies your shop&apos;s capability limits — trace widths, clearances, drill sizes, annular rings, solder mask dams, and 11 more checks — against every incoming design. No setup per job. No inconsistency between reviewers. Just upload and know.
          </p>
        </div>
      </section>

      {/* ═══════════════════════════════════════════════════════════════════════ */}
      {/* HOW IT WORKS                                                          */}
      {/* ═══════════════════════════════════════════════════════════════════════ */}
      <section id="how-it-works" data-section="how-it-works" className="max-w-6xl mx-auto px-6 py-24">
        <h2 className="text-3xl sm:text-4xl font-bold text-center mb-16">How It Works</h2>
        <div className="grid sm:grid-cols-4 gap-8 text-center">
          {[
            { step: '1', title: 'Upload', desc: 'Drop a Gerber ZIP or ODB++ archive. No file prep, no extraction.' },
            { step: '2', title: 'Apply Your Rules', desc: 'Select a capability profile that matches your process line.' },
            { step: '3', title: 'Instant Analysis', desc: '16 DFM checks run in under 30 seconds. Every violation scored and mapped.' },
            { step: '4', title: 'Review & Share', desc: 'Send a branded report link to your customer. Track revisions until it\'s fab-ready.' },
          ].map((s) => (
            <div key={s.step}>
              <div className="inline-flex items-center justify-center w-12 h-12 rounded-full bg-[#1565c0]/20 text-[#4fc3f7] font-bold text-lg mb-4">
                {s.step}
              </div>
              <h3 className="font-semibold text-lg mb-1">{s.title}</h3>
              <p className="text-sm text-slate-400">{s.desc}</p>
            </div>
          ))}
        </div>
      </section>

      {/* ═══════════════════════════════════════════════════════════════════════ */}
      {/* FEATURES (outcome-grouped, hover to expand)                           */}
      {/* ═══════════════════════════════════════════════════════════════════════ */}
      <section id="features" data-section="features" className="border-y border-white/10 bg-white/[0.02]">
        <div className="max-w-6xl mx-auto px-6 py-24">
          <h2 className="text-3xl sm:text-4xl font-bold text-center mb-16">
            What It Does For Your Shop
          </h2>
          <LandingFeatures />
        </div>
      </section>

      {/* ═══════════════════════════════════════════════════════════════════════ */}
      {/* ROI                                                                   */}
      {/* ═══════════════════════════════════════════════════════════════════════ */}
      <section data-section="roi" className="max-w-6xl mx-auto px-6 py-24">
        <h2 className="text-3xl sm:text-4xl font-bold text-center mb-16">
          The Math Is Simple
        </h2>
        <div className="grid sm:grid-cols-3 gap-8 max-w-4xl mx-auto text-center">
          <div className="rounded-2xl border border-white/10 bg-white/[0.03] p-8">
            <div className="text-4xl font-bold text-[#4fc3f7] mb-2">80%</div>
            <div className="text-sm text-slate-300 font-medium mb-2">Less first-pass review time</div>
            <p className="text-xs text-slate-400">What used to take 45 minutes per board takes seconds. Your engineers review results, not raw files.</p>
          </div>
          <div className="rounded-2xl border border-white/10 bg-white/[0.03] p-8">
            <div className="text-4xl font-bold text-[#4fc3f7] mb-2">3x</div>
            <div className="text-sm text-slate-300 font-medium mb-2">More RFQs without hiring</div>
            <p className="text-xs text-slate-400">Screen incoming designs automatically. Your team focuses on quoting, not filtering.</p>
          </div>
          <div className="rounded-2xl border border-white/10 bg-white/[0.03] p-8">
            <div className="text-4xl font-bold text-[#4fc3f7] mb-2">50%</div>
            <div className="text-sm text-slate-300 font-medium mb-2">Fewer revision cycles</div>
            <p className="text-xs text-slate-400">Customers see every issue upfront with exact locations. Fix once, not three times.</p>
          </div>
        </div>
      </section>

      {/* ═══════════════════════════════════════════════════════════════════════ */}
      {/* PRICING                                                               */}
      {/* ═══════════════════════════════════════════════════════════════════════ */}
      <section id="pricing" data-section="pricing" className="border-y border-white/10 bg-white/[0.02]">
        <div className="max-w-6xl mx-auto px-6 py-24">
          <h2 className="text-3xl sm:text-4xl font-bold text-center mb-4">
            Costs Less Than a Single Engineer Hour Per Day
          </h2>
          <p className="text-slate-400 text-center max-w-xl mx-auto mb-16">
            Every plan pays for itself in the first week of use.
          </p>
          <div className="grid sm:grid-cols-3 gap-8 max-w-4xl mx-auto">
            {[
              {
                name: 'Pilot',
                price: '$799',
                period: '/mo',
                subtitle: 'Prove the value. Minimal risk.',
                features: [
                  'Up to 30 design analyses per month',
                  'Unlimited team access',
                  '1 manufacturing line',
                  'PDF & CSV export',
                  'Interactive board viewer',
                ],
                roi: 'Perfect for evaluating on real production jobs',
                cta: 'Start Pilot',
                highlight: false,
              },
              {
                name: 'Production',
                price: '$2,499',
                period: '/mo',
                subtitle: 'Your core DFM workflow. Automated.',
                features: [
                  'Up to 250 design analyses per month',
                  'Unlimited team access',
                  'Up to 5 manufacturing lines',
                  'Customer portal with branded share links',
                  'Bulk design processing',
                  'Revision-to-revision tracking',
                ],
                roi: 'Replace manual first-pass review across all your lines',
                cta: 'Get Started',
                highlight: true,
              },
              {
                name: 'Enterprise',
                price: '',
                period: '',
                subtitle: 'Full automation. Intake to approval.',
                features: [
                  'Unlimited analyses',
                  'Unlimited team access',
                  'Custom manufacturing lines',
                  'Everything in Production',
                  'API access for automated intake',
                  'Admin dashboard & analytics',
                  'Priority processing',
                  'Dedicated onboarding',
                ],
                roi: 'For high-volume shops and multi-site operations',
                cta: 'Talk to Sales',
                highlight: false,
              },
            ].map((tier) => (
              <div
                key={tier.name}
                className={`rounded-2xl border p-8 flex flex-col ${
                  tier.highlight
                    ? 'border-[#1565c0] bg-[#1565c0]/10 ring-1 ring-[#1565c0]/30'
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
                <p className="text-xs text-[#4fc3f7] mb-6">{tier.roi}</p>
                <ul className="flex-1 space-y-3 mb-8">
                  {tier.features.map((f) => (
                    <li key={f} className="flex items-center gap-2 text-sm text-slate-300">
                      <span className="h-1.5 w-1.5 rounded-full bg-[#4fc3f7] shrink-0" />
                      {f}
                    </li>
                  ))}
                </ul>
                <a
                  href="#contact"
                  data-track-click={`pricing-${tier.name.toLowerCase()}`}
                  className={`block text-center py-3 rounded-xl font-semibold transition-colors ${
                    tier.highlight
                      ? 'bg-[#1565c0] hover:bg-[#1976d2] text-white'
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

      {/* ═══════════════════════════════════════════════════════════════════════ */}
      {/* FINAL CTA                                                             */}
      {/* ═══════════════════════════════════════════════════════════════════════ */}
      <section id="contact" data-section="contact" className="max-w-6xl mx-auto px-6 py-24">
        <div className="max-w-xl mx-auto">
          <h2 className="text-3xl sm:text-4xl font-bold text-center mb-4">
            Stop Reviewing. Start Screening.
          </h2>
          <p className="text-slate-400 text-center mb-10">
            Send us a board file you&apos;ve already reviewed. We&apos;ll run it through RapidDFM and show you what it catches — on your own designs, not a canned demo.
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
            <a href="#how-it-works" className="hover:text-white transition-colors">How It Works</a>
            <a href="#features" className="hover:text-white transition-colors">Features</a>
            <a href="#pricing" className="hover:text-white transition-colors">Pricing</a>
            <a href="#contact" className="hover:text-white transition-colors">Contact</a>
            <Link href={`${PORTAL_URL}/login`} className="hover:text-white transition-colors">Sign In</Link>
          </div>
        </div>
      </footer>
    </div>
  )
}
