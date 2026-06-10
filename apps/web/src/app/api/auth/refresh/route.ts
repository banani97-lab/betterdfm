import { NextRequest, NextResponse } from 'next/server'
import {
  CognitoIdentityProviderClient,
  InitiateAuthCommand,
  NotAuthorizedException,
} from '@aws-sdk/client-cognito-identity-provider'
import { clearRefreshCookie, REFRESH_COOKIE_NAME } from '../refresh-cookie'

const REGION = process.env.NEXT_PUBLIC_COGNITO_REGION || 'us-east-1'
const CLIENT_ID = process.env.NEXT_PUBLIC_COGNITO_CLIENT_ID || ''

const cognito = new CognitoIdentityProviderClient({ region: REGION })

export async function POST(req: NextRequest) {
  if (!CLIENT_ID) {
    // Dev mode — auth is bypassed; mirror signin's dev token.
    return NextResponse.json({ token: 'dev-token' })
  }

  const refreshToken = req.cookies.get(REFRESH_COOKIE_NAME)?.value
  if (!refreshToken) {
    return NextResponse.json({ error: 'No refresh token' }, { status: 401 })
  }

  try {
    const res = await cognito.send(
      new InitiateAuthCommand({
        AuthFlow: 'REFRESH_TOKEN_AUTH',
        ClientId: CLIENT_ID,
        AuthParameters: {
          REFRESH_TOKEN: refreshToken,
        },
      })
    )

    const token = res.AuthenticationResult?.IdToken
    if (!token) {
      return NextResponse.json({ error: 'No token returned from Cognito' }, { status: 502 })
    }

    // Cognito does not rotate the refresh token on REFRESH_TOKEN_AUTH —
    // keep the existing cookie as-is.
    return NextResponse.json({ token })
  } catch (err) {
    if (err instanceof NotAuthorizedException) {
      // Refresh token revoked or expired — drop the cookie so the client
      // stops retrying and falls back to the login page.
      const response = NextResponse.json({ error: 'Session expired' }, { status: 401 })
      clearRefreshCookie(response)
      return response
    }
    const msg = err instanceof Error ? err.message : 'Token refresh failed'
    console.error('[auth/refresh]', msg)
    return NextResponse.json({ error: msg }, { status: 500 })
  }
}
