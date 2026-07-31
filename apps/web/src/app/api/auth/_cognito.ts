import { CognitoIdentityProviderClient } from '@aws-sdk/client-cognito-identity-provider'

// Shared Cognito config for the auth routes. Both the app and admin user-pool
// clients are secretless (generate_secret = false), so no SECRET_HASH is
// computed anywhere. MFA is enforced pool-wide, so both clients surface the
// same MFA_SETUP / SOFTWARE_TOKEN_MFA challenges.

export const REGION = process.env.NEXT_PUBLIC_COGNITO_REGION || 'us-east-1'
export const APP_CLIENT_ID = process.env.NEXT_PUBLIC_COGNITO_CLIENT_ID || ''
export const ADMIN_CLIENT_ID = process.env.NEXT_PUBLIC_ADMIN_COGNITO_CLIENT_ID || ''

export const cognito = new CognitoIdentityProviderClient({ region: REGION })

/** Resolve the Cognito app-client id for an MFA route's `client` field. */
export function clientIdFor(client: unknown): string {
  return client === 'admin' ? ADMIN_CLIENT_ID : APP_CLIENT_ID
}
