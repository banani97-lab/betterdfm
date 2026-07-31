import mixpanel from 'mixpanel-browser'

// ITAR/CUI boundary: never load analytics in the GovCloud build, regardless of
// token configuration. Mixpanel's ingestion endpoint is outside the boundary.
// The gov web bundle bakes a us-gov-* region, so gate on that (fail-safe: even
// if a token were baked in, IS_GOV forces analytics off).
const IS_GOV = (process.env.NEXT_PUBLIC_COGNITO_REGION || '').toLowerCase().startsWith('us-gov-')
const TOKEN = IS_GOV ? '' : (process.env.NEXT_PUBLIC_MIXPANEL_TOKEN || '')
let initialized = false

function init() {
  if (initialized || !TOKEN) return
  mixpanel.init(TOKEN, { track_pageview: false, persistence: 'localStorage' })
  initialized = true
}

export function track(event: string, props?: Record<string, any>) {
  if (!TOKEN) return
  init()
  mixpanel.track(event, props)
}

export function identify(id: string, traits?: Record<string, any>) {
  if (!TOKEN) return
  init()
  mixpanel.identify(id)
  if (traits) mixpanel.people.set(traits)
}

export function reset() {
  if (!TOKEN) return
  init()
  mixpanel.reset()
}
