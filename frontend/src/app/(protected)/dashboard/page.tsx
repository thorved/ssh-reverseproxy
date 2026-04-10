"use client";

import { useQuery } from "@tanstack/react-query";
import { KeyRound, MonitorCog, Shield } from "lucide-react";
import { PageHeader } from "@/components/layout/page-header";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { useAuth } from "@/contexts/auth-context";
import { api } from "@/lib/api";

function sshPattern(sshPort: number) {
  const portSuffix = sshPort === 22 ? "" : ` -p ${sshPort}`;
  return `ssh <instance-slug>@<proxy-host>${portSuffix}`;
}

export default function DashboardPage() {
  const { user } = useAuth();
  const instancesQuery = useQuery({
    queryKey: ["dashboard", "instances"],
    queryFn: api.listUserInstances,
  });
  const keysQuery = useQuery({
    queryKey: ["dashboard", "keys"],
    queryFn: api.listSSHKeys,
  });

  const stats = [
    {
      label: "Assigned instances",
      value: instancesQuery.data?.instances.length ?? 0,
      icon: MonitorCog,
    },
    {
      label: "Published keys",
      value: keysQuery.data?.length ?? 0,
      icon: KeyRound,
    },
    {
      label: "Role",
      value: user?.role ?? "user",
      icon: Shield,
    },
  ];

  return (
    <div className="space-y-6">
      <PageHeader
        eyebrow="Workspace"
        title="Overview"
        description="A quick view of your SSH access surface, published keys, and the connection format expected by the proxy."
        badge={user?.role}
      />

      <section className="grid gap-4 md:grid-cols-3">
        {stats.map((stat) => {
          const Icon = stat.icon;
          return (
            <Card key={stat.label}>
              <CardHeader className="flex flex-row items-center justify-between">
                <CardTitle className="text-base">{stat.label}</CardTitle>
                <div className="rounded-2xl bg-primary/10 p-3 text-primary">
                  <Icon className="h-5 w-5" />
                </div>
              </CardHeader>
              <CardContent>
                <p className="text-3xl font-semibold">{stat.value}</p>
              </CardContent>
            </Card>
          );
        })}
      </section>

      <section className="grid gap-6 xl:grid-cols-[1.1fr_0.9fr]">
        <Card>
          <CardHeader>
            <CardTitle>Your connection pattern</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="rounded-2xl bg-secondary/70 p-4 font-mono text-sm">
              {sshPattern(instancesQuery.data?.ssh_port ?? 2222)}
            </div>
            <p className="text-sm text-muted-foreground">
              The SSH key identifies you. The SSH username chooses the assigned
              instance slug. If the slug is not assigned to you, the backend
              rejects the session before dialing upstream.
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Assigned instances</CardTitle>
          </CardHeader>
          <CardContent className="space-y-3">
            {instancesQuery.data?.instances.length ? (
              instancesQuery.data.instances.map((instance) => (
                <div
                  key={instance.id}
                  className="flex items-center justify-between gap-4 rounded-2xl border border-border/70 bg-background/70 p-4"
                >
                  <div>
                    <p className="font-medium">{instance.name}</p>
                    <p className="text-sm text-muted-foreground">
                      {instance.slug} {"->"} {instance.upstream_user}@
                      {instance.upstream_host}:{instance.upstream_port}
                    </p>
                  </div>
                  <Badge tone={instance.enabled ? "success" : "muted"}>
                    {instance.enabled ? "enabled" : "disabled"}
                  </Badge>
                </div>
              ))
            ) : (
              <p className="text-sm text-muted-foreground">
                No instances are assigned yet.
              </p>
            )}
          </CardContent>
        </Card>
      </section>
    </div>
  );
}
