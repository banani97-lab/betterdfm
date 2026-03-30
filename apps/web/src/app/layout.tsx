import type { Metadata } from 'next'
import { ThemeInit } from '@/components/ui/ThemeInit'
import { AnalyticsPageView } from '@/components/ui/AnalyticsPageView'
import darkFavicon from '@/app/dashboard/RapidDFM Dark Mode Favicon.png'
import lightFavicon from '@/app/dashboard/RapidDFM Light Mode Favicon.png'
import {
  APP_DESCRIPTION,
  APP_TITLE,
  LEGACY_THEME_STORAGE_KEY,
  THEME_STORAGE_KEY,
  UI_SETTINGS_STORAGE_KEY,
} from '@/lib/branding'
import './globals.css'

export const metadata: Metadata = {
  title: APP_TITLE,
  description: APP_DESCRIPTION,
}

export default function RootLayout({
  children,
}: {
  children: React.ReactNode
}) {
  return (
    <html lang="en" suppressHydrationWarning>
      <head>
        <link rel="icon" type="image/png" href={lightFavicon.src} />
        <link
          rel="icon"
          type="image/png"
          href={lightFavicon.src}
          media="(prefers-color-scheme: light)"
        />
        <link
          rel="icon"
          type="image/png"
          href={darkFavicon.src}
          media="(prefers-color-scheme: dark)"
        />
        <script
          dangerouslySetInnerHTML={{
            __html: `(() => {
  const key = '${THEME_STORAGE_KEY}';
  const legacyKey = '${LEGACY_THEME_STORAGE_KEY}';
  const uiSettingsKey = '${UI_SETTINGS_STORAGE_KEY}';
  const stored = localStorage.getItem(key) ?? localStorage.getItem(legacyKey);
  if (stored !== null) {
    localStorage.setItem(key, stored);
    localStorage.removeItem(legacyKey);
  }
  const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
  const useDark = stored ? stored === 'dark' : prefersDark;
  document.documentElement.classList.toggle('dark', useDark);
  try {
    const uiSettingsRaw = localStorage.getItem(uiSettingsKey);
    const uiSettings = uiSettingsRaw ? JSON.parse(uiSettingsRaw) : null;
    const background = uiSettings?.background;
    const normalized = background === 'default' ? 'spotlight' : background;
    const valid = normalized === 'spotlight' || normalized === 'studio' || normalized === 'grid' || normalized === 'aurora';
    document.documentElement.setAttribute('data-ui-bg', valid ? normalized : 'studio');
  } catch {
    document.documentElement.setAttribute('data-ui-bg', 'studio');
  }
})();`,
          }}
        />
      </head>
      <body className="min-h-screen bg-background font-sans antialiased">
        <ThemeInit />
        <AnalyticsPageView />
        {children}
      </body>
    </html>
  )
}
