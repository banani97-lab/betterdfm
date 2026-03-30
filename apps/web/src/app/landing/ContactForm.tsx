'use client'

import { useState } from 'react'
import { Send, CheckCircle } from 'lucide-react'
import { track } from '@/lib/analytics'

export function ContactForm() {
  const [name, setName] = useState('')
  const [email, setEmail] = useState('')
  const [company, setCompany] = useState('')
  const [message, setMessage] = useState('')
  const [sending, setSending] = useState(false)
  const [sent, setSent] = useState(false)
  const [error, setError] = useState('')

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setSending(true)
    try {
      const res = await fetch('/api/contact', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name, email, company, message }),
      })
      const data = await res.json()
      if (!res.ok) throw new Error(data.error || 'Failed to send')
      setSent(true)
      track('Contact Submitted', { company })
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Something went wrong')
    } finally {
      setSending(false)
    }
  }

  if (sent) {
    return (
      <div className="text-center py-8">
        <CheckCircle className="h-12 w-12 text-green-400 mx-auto mb-4" />
        <h3 className="text-xl font-semibold mb-2">Message sent!</h3>
        <p className="text-slate-400">We&apos;ll get back to you shortly.</p>
      </div>
    )
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-4">
      <div className="grid sm:grid-cols-2 gap-4">
        <div>
          <label htmlFor="contact-name" className="block text-xs font-medium text-slate-400 mb-1.5">Name *</label>
          <input
            id="contact-name"
            type="text"
            required
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="Jane Smith"
            className="w-full px-4 py-2.5 rounded-lg bg-white/5 border border-white/10 text-white placeholder:text-slate-500 focus:outline-none focus:border-blue-500 focus:ring-1 focus:ring-blue-500 text-sm transition-colors"
          />
        </div>
        <div>
          <label htmlFor="contact-email" className="block text-xs font-medium text-slate-400 mb-1.5">Email *</label>
          <input
            id="contact-email"
            type="email"
            required
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            placeholder="jane@pcbshop.com"
            className="w-full px-4 py-2.5 rounded-lg bg-white/5 border border-white/10 text-white placeholder:text-slate-500 focus:outline-none focus:border-blue-500 focus:ring-1 focus:ring-blue-500 text-sm transition-colors"
          />
        </div>
      </div>
      <div>
        <label htmlFor="contact-company" className="block text-xs font-medium text-slate-400 mb-1.5">Company</label>
        <input
          id="contact-company"
          type="text"
          value={company}
          onChange={(e) => setCompany(e.target.value)}
          placeholder="Acme PCB Manufacturing"
          className="w-full px-4 py-2.5 rounded-lg bg-white/5 border border-white/10 text-white placeholder:text-slate-500 focus:outline-none focus:border-blue-500 focus:ring-1 focus:ring-blue-500 text-sm transition-colors"
        />
      </div>
      <div>
        <label htmlFor="contact-message" className="block text-xs font-medium text-slate-400 mb-1.5">Message *</label>
        <textarea
          id="contact-message"
          required
          rows={4}
          value={message}
          onChange={(e) => setMessage(e.target.value)}
          placeholder="Tell us about your shop and what you're looking for..."
          className="w-full px-4 py-2.5 rounded-lg bg-white/5 border border-white/10 text-white placeholder:text-slate-500 focus:outline-none focus:border-blue-500 focus:ring-1 focus:ring-blue-500 text-sm transition-colors resize-none"
        />
      </div>
      {error && (
        <p className="text-sm text-red-400">{error}</p>
      )}
      <button
        type="submit"
        disabled={sending}
        data-track-click="contact-submit"
        className="w-full inline-flex items-center justify-center gap-2 px-6 py-3 rounded-xl bg-blue-600 hover:bg-blue-500 disabled:opacity-50 text-white font-semibold transition-colors"
      >
        {sending ? 'Sending...' : <><Send className="h-4 w-4" /> Send Message</>}
      </button>
    </form>
  )
}
