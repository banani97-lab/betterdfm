import mixpanel from 'mixpanel-browser'

const TOKEN = process.env.NEXT_PUBLIC_MIXPANEL_TOKEN || ''
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
