"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Plus } from "lucide-react";
import { useId, useState } from "react";
import { PageHeader } from "@/components/layout/page-header";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select } from "@/components/ui/select";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Textarea } from "@/components/ui/textarea";
import { api, type Instance } from "@/lib/api";

type InstanceFormState = {
  id: number | null;
  name: string;
  slug: string;
  description: string;
  upstream_host: string;
  upstream_port: number;
  upstream_user: string;
  auth_method: "none" | "password" | "key";
  auth_password: string;
  auth_key_inline: string;
  auth_passphrase: string;
  assigned_user_id: number | null;
  enabled: boolean;
};

const emptyState: InstanceFormState = {
  id: null,
  name: "",
  slug: "",
  description: "",
  upstream_host: "",
  upstream_port: 22,
  upstream_user: "",
  auth_method: "key",
  auth_password: "",
  auth_key_inline: "",
  auth_passphrase: "",
  assigned_user_id: null,
  enabled: true,
};

export default function AdminInstancesPage() {
  const nameId = useId();
  const slugId = useId();
  const descriptionId = useId();
  const hostId = useId();
  const portId = useId();
  const upstreamUserId = useId();
  const authMethodId = useId();
  const assignedUserId = useId();
  const authPasswordId = useId();
  const authKeyId = useId();
  const authPassphraseId = useId();
  const queryClient = useQueryClient();
  const instancesQuery = useQuery({
    queryKey: ["admin-instances"],
    queryFn: api.listAdminInstances,
  });
  const usersQuery = useQuery({
    queryKey: ["admin-users"],
    queryFn: api.listUsers,
  });

  const [form, setForm] = useState<InstanceFormState>(emptyState);
  const [error, setError] = useState<string | null>(null);
  const [isModalOpen, setIsModalOpen] = useState(false);

  const saveMutation = useMutation({
    mutationFn: async (payload: InstanceFormState) => {
      const requestBody = {
        ...payload,
        assigned_user_id: payload.assigned_user_id,
      };
      if (payload.id) {
        return api.updateInstance(payload.id, requestBody);
      }
      return api.createInstance(requestBody);
    },
    onSuccess: () => {
      setForm(emptyState);
      setError(null);
      setIsModalOpen(false);
      void queryClient.invalidateQueries({ queryKey: ["admin-instances"] });
    },
    onError: (mutationError) => {
      setError(
        mutationError instanceof Error ? mutationError.message : "Save failed",
      );
    },
  });

  const assignMutation = useMutation({
    mutationFn: ({ id, userId }: { id: number; userId: number | null }) =>
      api.assignInstance(id, userId),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["admin-instances"] });
    },
  });

  const startEdit = (instance: Instance) => {
    setForm({
      id: instance.id,
      name: instance.name,
      slug: instance.slug,
      description: instance.description ?? "",
      upstream_host: instance.upstream_host,
      upstream_port: instance.upstream_port,
      upstream_user: instance.upstream_user,
      auth_method: instance.auth_method,
      auth_password: instance.auth_password ?? "",
      auth_key_inline: instance.auth_key_inline ?? "",
      auth_passphrase: instance.auth_passphrase ?? "",
      assigned_user_id: instance.assigned_user_id ?? null,
      enabled: instance.enabled,
    });
    setError(null);
    setIsModalOpen(true);
  };

  return (
    <div className="space-y-6">
      <PageHeader
        eyebrow="Administration"
        title="Instances"
        description="Define upstream SSH targets, keep their auth material in SQLite, and assign each instance to exactly one user."
        actions={
          <Button
            onClick={() => {
              setForm(emptyState);
              setError(null);
              setIsModalOpen(true);
            }}
          >
            <Plus className="h-4 w-4" />
            Create Instance
          </Button>
        }
      />

      <Dialog
        open={isModalOpen}
        onOpenChange={(open) => {
          setIsModalOpen(open);
          if (open) {
            return;
          }
          setIsModalOpen(false);
          setForm(emptyState);
          setError(null);
        }}
      >
        <DialogContent className="sm:max-w-4xl">
          <DialogHeader>
            <DialogTitle>
              {form.id ? "Edit instance" : "Create instance"}
            </DialogTitle>
            <DialogDescription>
              Manage upstream targets, routing slug, and assignment in one
              place.
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-4">
            <div className="grid gap-4 sm:grid-cols-2">
              <div className="space-y-2">
                <Label htmlFor={nameId}>Name</Label>
                <Input
                  id={nameId}
                  value={form.name}
                  onChange={(event) =>
                    setForm((current) => ({
                      ...current,
                      name: event.target.value,
                    }))
                  }
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor={slugId}>Slug</Label>
                <Input
                  id={slugId}
                  value={form.slug}
                  onChange={(event) =>
                    setForm((current) => ({
                      ...current,
                      slug: event.target.value,
                    }))
                  }
                  placeholder="leave blank to derive from name"
                />
              </div>
            </div>

            <div className="space-y-2">
              <Label htmlFor={descriptionId}>Description</Label>
              <Textarea
                id={descriptionId}
                value={form.description}
                onChange={(event) =>
                  setForm((current) => ({
                    ...current,
                    description: event.target.value,
                  }))
                }
              />
            </div>

            <div className="grid gap-4 sm:grid-cols-2">
              <div className="space-y-2">
                <Label htmlFor={hostId}>Upstream host</Label>
                <Input
                  id={hostId}
                  value={form.upstream_host}
                  onChange={(event) =>
                    setForm((current) => ({
                      ...current,
                      upstream_host: event.target.value,
                    }))
                  }
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor={portId}>Port</Label>
                <Input
                  id={portId}
                  type="number"
                  value={form.upstream_port}
                  onChange={(event) =>
                    setForm((current) => ({
                      ...current,
                      upstream_port: Number(event.target.value) || 22,
                    }))
                  }
                />
              </div>
            </div>

            <div className="grid gap-4 sm:grid-cols-2">
              <div className="space-y-2">
                <Label htmlFor={upstreamUserId}>Upstream user</Label>
                <Input
                  id={upstreamUserId}
                  value={form.upstream_user}
                  onChange={(event) =>
                    setForm((current) => ({
                      ...current,
                      upstream_user: event.target.value,
                    }))
                  }
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor={assignedUserId}>Assigned user</Label>
                <Select
                  id={assignedUserId}
                  value={form.assigned_user_id?.toString() ?? ""}
                  onChange={(event) =>
                    setForm((current) => ({
                      ...current,
                      assigned_user_id: event.target.value
                        ? Number(event.target.value)
                        : null,
                    }))
                  }
                >
                  <option value="">Unassigned</option>
                  {usersQuery.data?.map((user) => (
                    <option key={user.id} value={user.id}>
                      {user.display_name || user.email}
                    </option>
                  ))}
                </Select>
              </div>
            </div>

            <div className="space-y-2">
              <Label htmlFor={authMethodId}>Auth method</Label>
              <Select
                id={authMethodId}
                value={form.auth_method}
                onChange={(event) =>
                  setForm((current) => ({
                    ...current,
                    auth_method: event.target
                      .value as InstanceFormState["auth_method"],
                  }))
                }
              >
                <option value="key">Private key</option>
                <option value="password">Password</option>
                <option value="none">None</option>
              </Select>
            </div>

            {form.auth_method === "password" ? (
              <div className="space-y-2">
                <Label htmlFor={authPasswordId}>Upstream password</Label>
                <Input
                  id={authPasswordId}
                  type="password"
                  value={form.auth_password}
                  onChange={(event) =>
                    setForm((current) => ({
                      ...current,
                      auth_password: event.target.value,
                    }))
                  }
                />
              </div>
            ) : null}

            {form.auth_method === "key" ? (
              <div className="space-y-4">
                <div className="space-y-2">
                  <Label htmlFor={authKeyId}>Private key</Label>
                  <Textarea
                    id={authKeyId}
                    value={form.auth_key_inline}
                    onChange={(event) =>
                      setForm((current) => ({
                        ...current,
                        auth_key_inline: event.target.value,
                      }))
                    }
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor={authPassphraseId}>Passphrase</Label>
                  <Input
                    id={authPassphraseId}
                    type="password"
                    value={form.auth_passphrase}
                    onChange={(event) =>
                      setForm((current) => ({
                        ...current,
                        auth_passphrase: event.target.value,
                      }))
                    }
                  />
                </div>
              </div>
            ) : null}

            <label className="flex items-center gap-3 rounded-xl border border-border bg-background px-4 py-3 text-sm">
              <input
                type="checkbox"
                checked={form.enabled}
                onChange={(event) =>
                  setForm((current) => ({
                    ...current,
                    enabled: event.target.checked,
                  }))
                }
              />
              Instance enabled
            </label>

            {error ? <p className="text-sm text-destructive">{error}</p> : null}

            <DialogFooter>
              <Button
                variant="outline"
                onClick={() => {
                  setIsModalOpen(false);
                  setForm(emptyState);
                  setError(null);
                }}
              >
                Cancel
              </Button>
              <Button
                onClick={() => saveMutation.mutate(form)}
                disabled={saveMutation.isPending}
              >
                {form.id ? "Save changes" : "Create instance"}
              </Button>
            </DialogFooter>
          </div>
        </DialogContent>
      </Dialog>

      <div>
        <Card>
          <CardContent className="p-0">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Instance</TableHead>
                  <TableHead>Upstream</TableHead>
                  <TableHead>Assignment</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead className="text-right">Action</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {instancesQuery.data?.map((instance) => (
                  <TableRow key={instance.id}>
                    <TableCell>
                      <div>
                        <p className="font-medium">{instance.name}</p>
                        <p className="text-xs text-muted-foreground">
                          {instance.slug}
                        </p>
                      </div>
                    </TableCell>
                    <TableCell className="text-sm">
                      {instance.upstream_user}@{instance.upstream_host}:
                      {instance.upstream_port}
                    </TableCell>
                    <TableCell>
                      <div className="flex items-center gap-2">
                        <Badge
                          tone={instance.assigned_user ? "success" : "muted"}
                        >
                          {instance.assigned_user?.display_name ||
                            instance.assigned_user?.email ||
                            "unassigned"}
                        </Badge>
                        <Button
                          variant="ghost"
                          onClick={() =>
                            assignMutation.mutate({
                              id: instance.id,
                              userId: null,
                            })
                          }
                          disabled={
                            !instance.assigned_user_id ||
                            assignMutation.isPending
                          }
                        >
                          Clear
                        </Button>
                      </div>
                    </TableCell>
                    <TableCell>
                      <Badge tone={instance.enabled ? "success" : "muted"}>
                        {instance.enabled ? "enabled" : "disabled"}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-right">
                      <Button
                        variant="ghost"
                        onClick={() => startEdit(instance)}
                      >
                        Edit
                      </Button>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
