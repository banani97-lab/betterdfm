import { NextRequest, NextResponse } from 'next/server'
import { RespondToAuthChallengeCommand } from '@aws-sdk/client-cognito-identity-provider'
import { cognito, clientIdFor } from '../../_cognito'
import { setRefreshCookie } from '../../refresh-cookie'

// Returning-user MFA: answer a SOFTWARE_TOKEN_MFA challenge with the current
// authenticator code. `client` selects the app vs admin pool client.
export async function POST(req: NextRequest) {
  const { email, code, session, client } = await req.json()
  if (!email || !code || !session) {
    return NextResponse.json({ error: 'Missing email, code, or session' }, { status: 400 })
  }
  try {
    const res = await cognito.send(
      new RespondToAuthChallengeCommand({
        ChallengeName: 'SOFTWARE_TOKEN_MFA',
        ClientId: clientIdFor(client),
        Session: session,
        ChallengeResponses: { USERNAME: email, SOFTWARE_TOKEN_MFA_CODE: code },
      })
    )

    const token = res.AuthenticationResult?.IdToken
    if (!token) {
      return NextResponse.json({ error: 'No token returned from Cognito' }, { status: 502 })
    }

    const response = NextResponse.json({ token })
    const refresh = res.AuthenticationResult?.RefreshToken
    if (client !== 'admin' && refresh) setRefreshCookie(response, refresh)
    return response
  } catch (err) {
    const msg = err instanceof Error ? err.message : 'Invalid authenticator code'
    console.error('[auth/mfa/challenge]', msg)
    return NextResponse.json({ error: msg }, { status: 400 })
  }
}
