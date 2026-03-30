import { NextRequest, NextResponse } from 'next/server'

/**
 * Route by hostname:
 * - rapiddfm.com / www.rapiddfm.com → landing page (only "/" and "/landing/*")
 * - portal.rapiddfm.com / localhost  → app (dashboard, upload, results, etc.)
 */

const LANDING_HOSTS = ['rapiddfm.com', 'www.rapiddfm.com']

// Paths that exist on the landing site — everything else redirects to portal
const LANDING_PATHS = ['/', '/landing']

export function middleware(req: NextRequest) {
  const host = req.headers.get('host')?.split(':')[0] ?? ''
  const { pathname } = req.nextUrl

  const isLandingHost = LANDING_HOSTS.includes(host)

  if (isLandingHost) {
    // On the marketing domain: rewrite "/" to the landing page
    if (pathname === '/') {
      return NextResponse.rewrite(new URL('/landing', req.url))
    }
    // Allow /landing paths through
    if (pathname.startsWith('/landing')) {
      return NextResponse.next()
    }
    // Everything else on the marketing domain → redirect to portal
    const portalUrl = new URL(pathname, 'https://portal.rapiddfm.com')
    portalUrl.search = req.nextUrl.search
    return NextResponse.redirect(portalUrl)
  }

  // On portal/localhost: if hitting "/landing", redirect to marketing site
  if (pathname.startsWith('/landing')) {
    const marketingUrl = new URL('/', 'https://www.rapiddfm.com')
    return NextResponse.redirect(marketingUrl)
  }

  return NextResponse.next()
}

export const config = {
  // Don't run middleware on static files, API routes, or _next
  matcher: ['/((?!_next/static|_next/image|favicon.ico|api/).*)'],
}
