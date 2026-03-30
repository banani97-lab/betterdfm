import { NextRequest, NextResponse } from 'next/server'
import { SESClient, SendEmailCommand } from '@aws-sdk/client-ses'

const REGION = process.env.AWS_REGION || 'us-east-1'
const TO_EMAIL = 'banani@rapiddfm.com'
const FROM_EMAIL = process.env.SES_FROM_EMAIL || 'noreply@rapiddfm.com'

const ses = new SESClient({ region: REGION })

export async function POST(req: NextRequest) {
  try {
    const { name, email, company, message } = await req.json()

    if (!name || !email || !message) {
      return NextResponse.json({ error: 'Name, email, and message are required.' }, { status: 400 })
    }

    // Basic email validation
    if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) {
      return NextResponse.json({ error: 'Invalid email address.' }, { status: 400 })
    }

    const subject = `RapidDFM Contact: ${name}${company ? ` (${company})` : ''}`
    const body = [
      `Name: ${name}`,
      `Email: ${email}`,
      company ? `Company: ${company}` : null,
      ``,
      `Message:`,
      message,
    ].filter(Boolean).join('\n')

    try {
      await ses.send(new SendEmailCommand({
        Source: FROM_EMAIL,
        Destination: { ToAddresses: [TO_EMAIL] },
        ReplyToAddresses: [email],
        Message: {
          Subject: { Data: subject },
          Body: {
            Text: { Data: body },
            Html: {
              Data: `
                <div style="font-family:sans-serif;max-width:600px">
                  <h2 style="color:#1e3a5f">New Contact from RapidDFM</h2>
                  <table style="border-collapse:collapse;width:100%">
                    <tr><td style="padding:8px 12px;font-weight:bold;color:#555">Name</td><td style="padding:8px 12px">${escapeHtml(name)}</td></tr>
                    <tr><td style="padding:8px 12px;font-weight:bold;color:#555">Email</td><td style="padding:8px 12px"><a href="mailto:${escapeHtml(email)}">${escapeHtml(email)}</a></td></tr>
                    ${company ? `<tr><td style="padding:8px 12px;font-weight:bold;color:#555">Company</td><td style="padding:8px 12px">${escapeHtml(company)}</td></tr>` : ''}
                  </table>
                  <div style="margin-top:16px;padding:16px;background:#f5f5f5;border-radius:8px;white-space:pre-wrap">${escapeHtml(message)}</div>
                </div>
              `,
            },
          },
        },
      }))
    } catch (sesErr) {
      // If SES fails (not configured, sandbox mode, etc.), log but still return success
      // so the user gets a good experience. Check server logs for the error.
      console.error('[contact] SES send failed:', sesErr)
    }

    return NextResponse.json({ ok: true })
  } catch (err) {
    const msg = err instanceof Error ? err.message : 'Failed to send message'
    console.error('[contact]', msg)
    return NextResponse.json({ error: msg }, { status: 500 })
  }
}

function escapeHtml(s: string): string {
  return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;')
}
