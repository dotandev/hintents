import type { Metadata } from 'next'
import './globals.css'

export const metadata: Metadata = {
    title: 'ERST Trace Showcase',
    description: 'Interactive showcase for ERST traces',
}

export default function RootLayout({
    children,
}: {
    children: React.ReactNode
}) {
    return (
        <html lang="en">
            <body>{children}</body>
        </html>
    )
}
