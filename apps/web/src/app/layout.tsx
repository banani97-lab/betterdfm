import type { Metadata } from 'next'
import './globals.css'

export const metadata: Metadata = {
  title: 'BetterDFM',
  description: 'PCB Design-for-Manufacturability Analysis',
}

export default function RootLayout({
  children,
}: {
  children: React.ReactNode
}) {
  return (
    <html lang="en" suppressHydrationWarning>
      <head>
        <script
          dangerouslySetInnerHTML={{
            __html: `(() => {
  const key = 'betterdfm-theme';
  const uiSettingsKey = 'betterdfm-ui-settings';
  const stored = localStorage.getItem(key);
  const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
  const useDark = stored ? stored === 'dark' : prefersDark;
  document.documentElement.classList.toggle('dark', useDark);
  try {
    const uiSettingsRaw = localStorage.getItem(uiSettingsKey);
    const uiSettings = uiSettingsRaw ? JSON.parse(uiSettingsRaw) : null;
    const background = uiSettings?.background;
    const valid = background === 'default' || background === 'grid' || background === 'aurora';
    document.documentElement.setAttribute('data-ui-bg', valid ? background : 'default');
  } catch {
    document.documentElement.setAttribute('data-ui-bg', 'default');
  }
})();`,
          }}
        />
      </head>
      <body className="min-h-screen bg-background font-sans antialiased">
        {children}
      </body>
    </html>
  )
}
