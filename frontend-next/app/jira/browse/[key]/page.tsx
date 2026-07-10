import { IssueView } from "@/components/issues/IssueView";

export default async function Page({ params }: { params: Promise<{ key: string }> }) {
  const { key } = await params;
  return <IssueView issueKey={key} />;
}
