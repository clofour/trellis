import { JobDetail } from "@/components/job-detail";

export default async function JobPage({
  params,
  searchParams,
}: {
  params: Promise<{ name: string }>;
  searchParams: Promise<{ allocation?: string }>;
}) {
  const { name } = await params;
  const { allocation } = await searchParams;
  return (
    <JobDetail
      name={decodeURIComponent(name)}
      initialAllocationId={allocation}
    />
  );
}
