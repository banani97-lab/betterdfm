import { NextRequest, NextResponse } from 'next/server'
import {
  CognitoIdentityProviderClient,
  ConfirmForgotPasswordCommand,
  CodeMismatchException,
  ExpiredCodeException,
} from '@aws-sdk/client-cognito-identity-provider'

const REGION = process.env.NEXT_PUBLIC_COGNITO_REGION || 'us-east-1'
const CLIENT_ID = process.env.NEXT_PUBLIC_COGNITO_CLIENT_ID || ''

const cognito = new CognitoIdentityProviderClient({ region: REGION })

export async function POST(req: NextRequest) {
  const { email, code, newPassword } = await req.json()

  if (!CLIENT_ID) {
    return NextResponse.json({ ok: true })
  }

  try {
    await cognito.send(
      new ConfirmForgotPasswordCommand({
        ClientId: CLIENT_ID,
        Username: email,
        ConfirmationCode: code,
        Password: newPassword,
      })
    )
    return NextResponse.json({ ok: true })
  } catch (err) {
    if (err instanceof CodeMismatchException) {
      return NextResponse.json({ error: 'Invalid verification code.' }, { status: 400 })
    }
    if (err instanceof ExpiredCodeException) {
      return NextResponse.json({ error: 'Verification code has expired. Please request a new one.' }, { status: 400 })
    }
    const msg = err instanceof Error ? err.message : 'Failed to reset password'
    console.error('[auth/reset-password]', msg)
    return NextResponse.json({ error: msg }, { status: 500 })
  }
}
