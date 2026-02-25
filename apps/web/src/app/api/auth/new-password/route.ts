import { NextRequest, NextResponse } from 'next/server'
import {
  CognitoIdentityProviderClient,
  RespondToAuthChallengeCommand,
} from '@aws-sdk/client-cognito-identity-provider'

const REGION = process.env.NEXT_PUBLIC_COGNITO_REGION || 'us-east-1'
const CLIENT_ID = process.env.NEXT_PUBLIC_COGNITO_CLIENT_ID || ''

const cognito = new CognitoIdentityProviderClient({ region: REGION })

export async function POST(req: NextRequest) {
  const { email, newPassword, session } = await req.json()

  try {
    const res = await cognito.send(
      new RespondToAuthChallengeCommand({
        ChallengeName: 'NEW_PASSWORD_REQUIRED',
        ClientId: CLIENT_ID,
        ChallengeResponses: {
          USERNAME: email,
          NEW_PASSWORD: newPassword,
        },
        Session: session,
      })
    )

    const token = res.AuthenticationResult?.IdToken
    if (!token) {
      return NextResponse.json({ error: 'No token returned from Cognito' }, { status: 502 })
    }

    return NextResponse.json({ token })
  } catch (err) {
    const msg = err instanceof Error ? err.message : 'Failed to set new password'
    console.error('[auth/new-password]', msg)
    return NextResponse.json({ error: msg }, { status: 400 })
  }
}
