import { NextRequest, NextResponse } from 'next/server'
import {
  CognitoIdentityProviderClient,
  InitiateAuthCommand,
  NotAuthorizedException,
  UserNotFoundException,
} from '@aws-sdk/client-cognito-identity-provider'

const REGION = process.env.NEXT_PUBLIC_COGNITO_REGION || 'us-east-1'
const CLIENT_ID = process.env.NEXT_PUBLIC_COGNITO_CLIENT_ID || ''

const cognito = new CognitoIdentityProviderClient({ region: REGION })

export async function POST(req: NextRequest) {
  const { email, password } = await req.json()

  if (!CLIENT_ID) {
    return NextResponse.json({ token: 'dev-token' })
  }

  try {
    const res = await cognito.send(
      new InitiateAuthCommand({
        AuthFlow: 'USER_PASSWORD_AUTH',
        ClientId: CLIENT_ID,
        AuthParameters: {
          USERNAME: email,
          PASSWORD: password,
        },
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
    const msg = err instanceof Error ? err.message : 'Sign in failed'
    console.error('[auth/signin]', msg)
    return NextResponse.json({ error: msg }, { status: 500 })
  }
}
