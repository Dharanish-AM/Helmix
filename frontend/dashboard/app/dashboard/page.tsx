import { DashboardShell } from "@/components/dashboard-shell";

type DashboardPageProps = {
  searchParams?: {
    token?: string;
  };
};

export default function DashboardPage({ searchParams }: DashboardPageProps) {
  return <DashboardShell tokenFromQuery={searchParams?.token ?? null} />;
}
