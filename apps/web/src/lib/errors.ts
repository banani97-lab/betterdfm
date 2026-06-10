import { ApiError } from './api'

/**
 * Map an error to a short reason phrase for toast copy, joined as
 * `Couldn't <verb> <object> — <reason>`. Never returns the raw ApiError
 * message — those contain API paths and status codes ("API /foo: 500 ...").
 */
export function friendlyReason(e: unknown): string {
  if (e instanceof ApiError) {
    if (e.status === 402) return 'analysis limit reached'
    if (e.status === 403) return "you don't have permission"
    if (e.status === 404) return 'not found'
    if (e.status === 429) return 'too many requests, try again shortly'
    if (e.status >= 500) return 'server error, try again'
  }
  // fetch network failures surface as TypeError ("Failed to fetch")
  if (e instanceof TypeError) return 'server error, try again'
  return 'something went wrong, try again'
}
