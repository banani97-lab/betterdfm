import { NextRequest, NextResponse } from 'next/server'
import { AssociateSoftwareTokenCommand } from '@aws-sdk/client-cognito-identity-provider'
import { cognito } from '../../_cognito'

// Step 1 of TOTP enrollment: exchange an MFA_SETUP challenge session for a
// shared secret. Returns the secret (for manual entry into an authenticator
// app) and a fresh session to carry into verify-setup. Client-agnostic:
// AssociateSoftwareToken keys off the session, not the app client.
export async function POST(req: NextRequest) {
  const { session } = await req.json()
  if (!session) {
    return NextResponse.json({ error: 'Missing session' }, { status: 400 })
  }
  try {
    const res = await cognito.send(new AssociateSoftwareTokenCommand({ Session: session }))
    if (!res.SecretCode) {
      return NextResponse.json({ error: 'No secret returned from Cognito' }, { status: 502 })
    }
    return NextResponse.json({ secretCode: res.SecretCode, session: res.Session })
  } catch (err) {
    const msg = err instanceof Error ? err.message : 'Failed to start MFA setup'
    console.error('[auth/mfa/associate]', msg)
    return NextResponse.json({ error: msg }, { status: 400 })
  }
}
