import { NextRequest, NextResponse } from 'next/server'
import {
  VerifySoftwareTokenCommand,
  RespondToAuthChallengeCommand,
} from '@aws-sdk/client-cognito-identity-provider'
import { cognito, clientIdFor } from '../../_cognito'
import { setRefreshCookie } from '../../refresh-cookie'

// Step 2 of TOTP enrollment: verify the first code from the authenticator, then
// complete the MFA_SETUP challenge to obtain tokens. `client` selects the app
// vs admin pool client (both secretless).
export async function POST(req: NextRequest) {
  const { email, code, session, client } = await req.json()
  if (!email || !code || !session) {
    return NextResponse.json({ error: 'Missing email, code, or session' }, { status: 400 })
  }
  try {
    const verify = await cognito.send(
      new VerifySoftwareTokenCommand({
        Session: session,
        UserCode: code,
        FriendlyDeviceName: 'RapidDFM',
      })
    )
    if (verify.Status !== 'SUCCESS') {
      return NextResponse.json({ error: 'Invalid authenticator code' }, { status: 400 })
    }

    const res = await cognito.send(
      new RespondToAuthChallengeCommand({
        ChallengeName: 'MFA_SETUP',
        ClientId: clientIdFor(client),
        Session: verify.Session,
        ChallengeResponses: { USERNAME: email },
      })
    )

    const token = res.AuthenticationResult?.IdToken
    if (!token) {
      return NextResponse.json({ error: 'No token returned from Cognito' }, { status: 502 })
    }

    const response = NextResponse.json({ token })
    // Only the app flow carries a refresh cookie; the admin flow does not.
    const refresh = res.AuthenticationResult?.RefreshToken
    if (client !== 'admin' && refresh) setRefreshCookie(response, refresh)
    return response
  } catch (err) {
    const msg = err instanceof Error ? err.message : 'Failed to verify authenticator'
    console.error('[auth/mfa/verify-setup]', msg)
    return NextResponse.json({ error: msg }, { status: 400 })
  }
}
