import NotebookPageClient from './NotebookPageClient'

export async function generateStaticParams() {
  return [{ id: 'default' }]
}

export default async function Page({ params }: { params: Promise<{ id: string }> }) {
  // Await params as required by Next.js 15+ Server Components
  const { id } = await params
  return <NotebookPageClient />
}
