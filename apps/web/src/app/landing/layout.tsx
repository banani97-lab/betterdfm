import type { Metadata } from 'next'

export const metadata: Metadata = {
  title: 'RapidDFM — Automated PCB DFM Analysis for Contract Manufacturers',
  description:
    'Upload Gerber or ODB++ files and get instant design-for-manufacturability analysis against your shop\'s capabilities. 16 automated rule checks, scored reports, and a customer portal. Built for small to mid-tier PCB contract manufacturers.',
  keywords: [
    'DFM', 'DFM analysis', 'PCB DFM', 'design for manufacturability',
    'Gerber analysis', 'ODB++ analysis', 'PCB manufacturing',
    'contract manufacturer', 'PCB fabrication', 'DFM check',
    'trace width check', 'clearance check', 'annular ring',
    'drill size check', 'PCB design review', 'manufacturing score',
    'RapidDFM', 'Saturn Solutions',
  ],
  openGraph: {
    title: 'RapidDFM — Automated PCB DFM Analysis',
    description: 'Instant design-for-manufacturability checks for PCB contract manufacturers. Upload, analyze, share results with customers.',
    url: 'https://www.rapiddfm.com',
    siteName: 'RapidDFM',
    type: 'website',
  },
  twitter: {
    card: 'summary_large_image',
    title: 'RapidDFM — Automated PCB DFM Analysis',
    description: 'Instant design-for-manufacturability checks for PCB contract manufacturers.',
  },
  alternates: {
    canonical: 'https://www.rapiddfm.com',
  },
  robots: {
    index: true,
    follow: true,
  },
}

export default function LandingLayout({ children }: { children: React.ReactNode }) {
  return <>{children}</>
}
