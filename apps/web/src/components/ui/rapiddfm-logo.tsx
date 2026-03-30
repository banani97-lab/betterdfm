'use client'

import Image from 'next/image'
import Link from 'next/link'
import { APP_NAME, APP_TITLE } from '@/lib/branding'
import logoLight from '@/app/dashboard/RapidDFM Light Mode Logo New.png'
import logoDark from '@/app/dashboard/RapidDFM Dark Mode Logo New.png'

interface RapidDFMLogoProps {
  className?: string
}

export function RapidDFMLogo({ className }: RapidDFMLogoProps) {
  return (
    <Link
      href="/dashboard"
      className={className}
      aria-label={`${APP_NAME} dashboard`}
      title={`${APP_NAME} dashboard`}
    >
      <Image
        src={logoLight}
        alt={APP_TITLE}
        className="block dark:hidden h-16 w-auto"
        priority
      />
      <Image
        src={logoDark}
        alt={APP_TITLE}
        className="hidden dark:block h-16 w-auto"
        priority
      />
    </Link>
  )
}
