'use client'

interface UpgradeGateProps {
  allowed: boolean
  featureName: string
  tierRequired?: string
  children: React.ReactNode
}

export function UpgradeGate({ allowed, featureName, tierRequired, children }: UpgradeGateProps) {
  if (allowed) return <>{children}</>
  return (
    <div className="text-center py-6 px-4 rounded-xl border border-border bg-muted/30">
      <p className="text-sm font-medium text-muted-foreground">
        {featureName} is available on the {tierRequired || 'Professional'} plan.
      </p>
      <p className="text-xs text-muted-foreground mt-1">Contact your administrator to upgrade.</p>
    </div>
  )
}
