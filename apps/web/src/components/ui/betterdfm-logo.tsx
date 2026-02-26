'use client'

import Image from 'next/image'
import logoLight from '@/app/dashboard/BetterDFM Logo Light Mode.png'
import logoDark from '@/app/dashboard/BetterDFM Logo Dark Mode.png'

interface BetterDFMLogoProps {
  className?: string
}

export function BetterDFMLogo({ className }: BetterDFMLogoProps) {
  return (
    <div className={className}>
      <Image
        src={logoLight}
        alt="BetterDFM"
        className="block dark:hidden h-16 w-auto"
        priority
      />
      <Image
        src={logoDark}
        alt="BetterDFM"
        className="hidden dark:block h-16 w-auto"
        priority
      />
    </div>
  )
}
