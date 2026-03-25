import { createHmac } from 'crypto'
import { NextRequest, NextResponse } from 'next/server'
import {
  CognitoIdentityProviderClient,
  InitiateAuthCommand,
  NotAuthorizedException,
  UserNotFoundException,
} from '@aws-sdk/client-cognito-identity-provider'

const REGION = process.env.NEXT_PUBLIC_COGNITO_REGION || 'us-east-1'
const ADMIN_CLIENT_ID = process.env.NEXT_PUBLIC_ADMIN_COGNITO_CLIENT_ID || ''
const ADMIN_CLIENT_SECRET = process.env.ADMIN_COGNITO_CLIENT_SECRET || ''

const cognito = new CognitoIdentityProviderClient({ region: REGION })

function computeSecretHash(username: string): string {
  return createHmac('sha256', ADMIN_CLIENT_SECRET)
    .update(username + ADMIN_CLIENT_ID)
    .digest('base64')
}

export async function POST(req: NextRequest) {
  const { email, password } = await req.json()

  if (!ADMIN_CLIENT_ID) {
    return NextResponse.json({ token: 'dev-admin-token' })
  }

  try {
    const authParams: Record<string, string> = {
      USERNAME: email,
      PASSWORD: password,
    }
    if (ADMIN_CLIENT_SECRET) {
      authParams.SECRET_HASH = computeSecretHash(email)
    }

    const res = await cognito.send(
      new InitiateAuthCommand({
        AuthFlow: 'USER_PASSWORD_AUTH',
        ClientId: ADMIN_CLIENT_ID,
        AuthParameters: authParams,
      })
    )

    const token = res.AuthenticationResult?.IdToken
    if (!token) {
      return NextResponse.json({ error: 'No token returned from Cognito' }, { status: 502 })
    }

    return NextResponse.json({ token })
  } catch (err) {
    if (err instanceof NotAuthorizedException || err instanceof UserNotFoundException) {
      return NextResponse.json({ error: 'Incorrect email or password' }, { status: 401 })
    }
    const msg = err instanceof Error ? err.message : 'Admin sign in failed'
    console.error('[auth/admin-signin]', msg)
    return NextResponse.json({ error: msg }, { status: 500 })
  }
}
