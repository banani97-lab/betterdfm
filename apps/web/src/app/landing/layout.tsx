import type { Metadata } from 'next'

export const metadata: Metadata = {
  icons: {
    icon: '/favicon.png',
  },
  title: 'RapidDFM — Screen PCB Designs Automatically Before They Hit Your Engineers',
  description:
    'Automated DFM screening for PCB contract manufacturers. Upload Gerber or ODB++ files, run 16 manufacturability checks in under 30 seconds, and share results with customers. Reduce manual CAM review time by up to 80%.',
  keywords: [
    'DFM', 'DFM analysis', 'PCB DFM', 'design for manufacturability',
    'Gerber analysis', 'ODB++ analysis', 'PCB manufacturing',
    'contract manufacturer', 'PCB fabrication', 'DFM check',
    'trace width check', 'clearance check', 'annular ring',
    'drill size check', 'PCB design review', 'manufacturing score',
    'RapidDFM', 'Saturn Solutions',
  ],
  openGraph: {
    title: 'RapidDFM — Automated DFM Screening for PCB Shops',
    description: 'Stop reviewing bad designs. Start screening them automatically. 16 DFM checks in under 30 seconds.',
    url: 'https://www.rapiddfm.com',
    siteName: 'RapidDFM',
    type: 'website',
  },
  twitter: {
    card: 'summary_large_image',
    title: 'RapidDFM — Automated DFM Screening for PCB Shops',
    description: 'Stop reviewing bad designs. Start screening them automatically.',
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
