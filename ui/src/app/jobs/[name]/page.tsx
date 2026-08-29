import { JobDetail } from "@/components/job-detail";

export default async function JobPage({
  params,
}: {
  params: Promise<{ name: string }>;
}) {
  const { name } = await params;
  return <JobDetail name={decodeURIComponent(name)} />;
}
