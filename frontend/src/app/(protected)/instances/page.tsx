"use client";

import { useQuery } from "@tanstack/react-query";
import { PageHeader } from "@/components/layout/page-header";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { api } from "@/lib/api";

export default function InstancesPage() {
  const instancesQuery = useQuery({
    queryKey: ["instances"],
    queryFn: api.listUserInstances,
  });

  return (
    <div className="space-y-6">
      <PageHeader
        eyebrow="Access"
        title="My Instances"
        description="Instances assigned to your account. Use the slug as the SSH username when connecting through the proxy."
      />

      <Card>
        <CardHeader>
          <CardTitle>Assigned inventory</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          {instancesQuery.data?.length ? (
            instancesQuery.data.map((instance) => (
              <div
                key={instance.id}
                className="rounded-[1.5rem] border border-border/70 bg-background/80 p-5"
              >
                <div className="flex flex-wrap items-start justify-between gap-4">
                  <div className="space-y-2">
                    <div className="flex items-center gap-3">
                      <h2 className="text-lg font-semibold">{instance.name}</h2>
                      <Badge tone={instance.enabled ? "success" : "muted"}>
                        {instance.enabled ? "enabled" : "disabled"}
                      </Badge>
                    </div>
                    <p className="text-sm text-muted-foreground">
                      {instance.description || "No description provided."}
                    </p>
                  </div>
                  <div className="rounded-2xl bg-secondary/70 px-4 py-3 font-mono text-sm">
                    ssh {instance.slug}@proxy-host -p 2222
                  </div>
                </div>
                <div className="mt-4 grid gap-3 text-sm text-muted-foreground sm:grid-cols-2">
                  <p>
                    <span className="font-medium text-foreground">
                      Upstream:
                    </span>{" "}
                    {instance.upstream_user}@{instance.upstream_host}:
                    {instance.upstream_port}
                  </p>
                  <p>
                    <span className="font-medium text-foreground">Auth:</span>{" "}
                    {instance.auth_method}
                  </p>
                </div>
              </div>
            ))
          ) : (
            <p className="text-sm text-muted-foreground">
              No assigned instances yet.
            </p>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
