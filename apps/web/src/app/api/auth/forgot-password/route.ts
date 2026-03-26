import { NextRequest, NextResponse } from 'next/server'
import {
  CognitoIdentityProviderClient,
  ForgotPasswordCommand,
  UserNotFoundException,
  LimitExceededException,
} from '@aws-sdk/client-cognito-identity-provider'

const REGION = process.env.NEXT_PUBLIC_COGNITO_REGION || 'us-east-1'
const CLIENT_ID = process.env.NEXT_PUBLIC_COGNITO_CLIENT_ID || ''

const cognito = new CognitoIdentityProviderClient({ region: REGION })

export async function POST(req: NextRequest) {
  const { email } = await req.json()

  if (!CLIENT_ID) {
    return NextResponse.json({ delivered: true })
  }

  try {
    await cognito.send(
      new ForgotPasswordCommand({
        ClientId: CLIENT_ID,
        Username: email,
      })
    )
    return NextResponse.json({ delivered: true })
  } catch (err) {
    if (err instanceof UserNotFoundException) {
      // Don't reveal whether the email exists
      return NextResponse.json({ delivered: true })
    }
    if (err instanceof LimitExceededException) {
      return NextResponse.json({ error: 'Too many attempts. Please try again later.' }, { status: 429 })
    }
    const msg = err instanceof Error ? err.message : 'Failed to send reset code'
    console.error('[auth/forgot-password]', msg)
    return NextResponse.json({ error: msg }, { status: 500 })
  }
}
