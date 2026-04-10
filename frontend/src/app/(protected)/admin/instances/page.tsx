"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Copy, KeyRound, Plus, Trash2, Users } from "lucide-react";
import { useId, useState } from "react";
import { PageHeader } from "@/components/layout/page-header";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
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
  auth_public_key: string;
  key_input_mode: "generate" | "paste";
  regenerate_auth_key?: boolean;
  assigned_user_ids: number[];
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
  auth_public_key: "",
  key_input_mode: "generate",
  regenerate_auth_key: false,
  assigned_user_ids: [],
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
  const [successMessage, setSuccessMessage] = useState<string | null>(null);
  const [isModalOpen, setIsModalOpen] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<Instance | null>(null);

  const saveMutation = useMutation({
    mutationFn: async (payload: InstanceFormState) => {
      if (
        payload.auth_method === "key" &&
        payload.key_input_mode === "paste" &&
        !payload.auth_key_inline.trim()
      ) {
        throw new Error(
          "Paste a private key or switch back to Generate for me.",
        );
      }

      const requestBody = {
        ...payload,
        auth_key_inline:
          payload.auth_method === "key" &&
          payload.key_input_mode === "generate" &&
          !payload.id
            ? ""
            : payload.auth_key_inline,
        auth_passphrase:
          payload.auth_method === "key" &&
          payload.key_input_mode === "generate" &&
          !payload.id
            ? ""
            : payload.auth_passphrase,
        assigned_user_ids: payload.assigned_user_ids,
        auth_public_key: undefined,
        key_input_mode: undefined,
      };
      if (payload.id) {
        return api.updateInstance(payload.id, requestBody);
      }
      return api.createInstance(requestBody);
    },
    onSuccess: (instance, payload) => {
      setError(null);
      void queryClient.invalidateQueries({ queryKey: ["admin-instances"] });
      if (payload.id && !payload.regenerate_auth_key) {
        setSuccessMessage(null);
        setForm(emptyState);
        setIsModalOpen(false);
        return;
      }

      setForm({
        id: instance.id,
        name: instance.name,
        slug: instance.slug,
        description: instance.description ?? "",
        upstream_host: instance.upstream_host,
        upstream_port: instance.upstream_port,
        upstream_user: instance.upstream_user,
        auth_method: instance.auth_method,
        auth_password: "",
        auth_key_inline: "",
        auth_passphrase: "",
        auth_public_key: instance.auth_public_key ?? "",
        key_input_mode: payload.key_input_mode,
        regenerate_auth_key: false,
        assigned_user_ids: instance.assigned_user_ids ?? [],
        enabled: instance.enabled,
      });
      setSuccessMessage(
        payload.regenerate_auth_key
          ? "A fresh key pair was generated. Copy the new public key below."
          : instance.auth_method === "key" && instance.auth_public_key
            ? "Instance created. Copy the public key below and add it to the upstream machine."
            : "Instance created.",
      );
    },
    onError: (mutationError) => {
      setError(
        mutationError instanceof Error ? mutationError.message : "Save failed",
      );
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (instanceId: number) => api.deleteInstance(instanceId),
    onSuccess: () => {
      setDeleteTarget(null);
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
      auth_password: "",
      auth_key_inline: "",
      auth_passphrase: "",
      auth_public_key: instance.auth_public_key ?? "",
      key_input_mode: "generate",
      regenerate_auth_key: false,
      assigned_user_ids: instance.assigned_user_ids ?? [],
      enabled: instance.enabled,
    });
    setError(null);
    setSuccessMessage(null);
    setIsModalOpen(true);
  };

  const toggleAssignedUser = (userId: number, checked: boolean) => {
    setForm((current) => ({
      ...current,
      assigned_user_ids: checked
        ? [...current.assigned_user_ids, userId]
        : current.assigned_user_ids.filter((value) => value !== userId),
    }));
  };

  return (
    <div className="space-y-6">
      <PageHeader
        eyebrow="Administration"
        title="Instances"
        description="Define upstream SSH targets, keep their auth material in SQLite, and assign each instance to multiple users when needed."
        actions={
          <Button
            onClick={() => {
              setForm(emptyState);
              setError(null);
              setSuccessMessage(null);
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
          setForm(emptyState);
          setError(null);
          setSuccessMessage(null);
        }}
      >
        <DialogContent className="sm:max-w-4xl">
          <DialogHeader>
            <DialogTitle>
              {form.id ? "Edit instance" : "Create instance"}
            </DialogTitle>
            <DialogDescription>
              Manage upstream targets, routing slug, and which users can connect
              to this instance.
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
                <Label>User access</Label>
                <div className="rounded-2xl border border-border bg-background/55 p-4">
                  <div className="mb-3 flex items-center gap-2 text-sm font-medium">
                    <Users className="h-4 w-4" />
                    Select one or more users
                  </div>
                  <div className="max-h-48 space-y-2 overflow-y-auto">
                    {usersQuery.data?.map((user) => {
                      const checked = form.assigned_user_ids.includes(user.id);
                      return (
                        <label
                          key={user.id}
                          className="flex items-center gap-3 rounded-xl border border-border px-3 py-2 text-sm"
                        >
                          <input
                            type="checkbox"
                            checked={checked}
                            onChange={(event) =>
                              toggleAssignedUser(user.id, event.target.checked)
                            }
                          />
                          <span className="font-medium">
                            {user.display_name || user.email}
                          </span>
                          <span className="text-xs text-muted-foreground">
                            {user.email}
                          </span>
                        </label>
                      );
                    })}
                    {!usersQuery.data?.length ? (
                      <p className="text-sm text-muted-foreground">
                        Create users first to assign access.
                      </p>
                    ) : null}
                  </div>
                </div>
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
                  <Label>Key source</Label>
                  <div className="flex flex-wrap gap-2">
                    <Button
                      type="button"
                      variant={
                        form.key_input_mode === "generate"
                          ? "default"
                          : "outline"
                      }
                      onClick={() =>
                        setForm((current) => ({
                          ...current,
                          key_input_mode: "generate",
                          regenerate_auth_key: false,
                          auth_key_inline: current.id
                            ? current.auth_key_inline
                            : "",
                          auth_passphrase: current.id
                            ? current.auth_passphrase
                            : "",
                        }))
                      }
                    >
                      <KeyRound className="h-4 w-4" />
                      Generate for me
                    </Button>
                    <Button
                      type="button"
                      variant={
                        form.key_input_mode === "paste" ? "default" : "outline"
                      }
                      onClick={() =>
                        setForm((current) => ({
                          ...current,
                          key_input_mode: "paste",
                          regenerate_auth_key: false,
                          auth_public_key:
                            current.key_input_mode === "paste"
                              ? current.auth_public_key
                              : "",
                        }))
                      }
                    >
                      Paste my private key
                    </Button>
                  </div>
                </div>

                {form.key_input_mode === "generate" ? (
                  <div className="space-y-3 rounded-2xl border border-border bg-muted/25 p-4">
                    <p className="text-sm text-muted-foreground">
                      {form.id
                        ? form.auth_public_key
                          ? "This instance is using a stored private key. The public key is shown below if you need to install it on another upstream machine."
                          : "This instance is using a stored private key, but the public key is not available in this view right now."
                        : "A fresh ed25519 private key will be generated when you create the instance. Copy the public key after save and add it to the upstream machine."}
                    </p>
                    {form.auth_public_key ? (
                      <div className="space-y-4">
                        {form.id ? (
                          <div className="flex justify-end">
                            <Button
                              type="button"
                              variant="outline"
                              onClick={() =>
                                saveMutation.mutate({
                                  ...form,
                                  regenerate_auth_key: true,
                                })
                              }
                              disabled={saveMutation.isPending}
                            >
                              <KeyRound className="h-4 w-4" />
                              Regenerate key
                            </Button>
                          </div>
                        ) : null}
                        <div className="space-y-2">
                          <Label>
                            Public key to install on the upstream machine
                          </Label>
                          <Textarea
                            value={form.auth_public_key}
                            readOnly
                            className="min-h-24 font-mono text-xs"
                          />
                          <div className="flex justify-end">
                            <Button
                              type="button"
                              variant="outline"
                              onClick={async () => {
                                await navigator.clipboard.writeText(
                                  form.auth_public_key,
                                );
                              }}
                            >
                              <Copy className="h-4 w-4" />
                              Copy public key
                            </Button>
                          </div>
                        </div>
                      </div>
                    ) : (
                      <div className="rounded-xl border border-dashed border-border bg-background/70 px-4 py-3 text-sm text-muted-foreground">
                        <div className="space-y-3">
                          <p>
                            {form.id
                              ? "No public key is available to show for this saved instance."
                              : "The public key will appear here right after you create the instance."}
                          </p>
                          {form.id ? (
                            <div className="flex justify-end">
                              <Button
                                type="button"
                                variant="outline"
                                onClick={() =>
                                  saveMutation.mutate({
                                    ...form,
                                    regenerate_auth_key: true,
                                  })
                                }
                                disabled={saveMutation.isPending}
                              >
                                <KeyRound className="h-4 w-4" />
                                Regenerate key
                              </Button>
                            </div>
                          ) : null}
                        </div>
                      </div>
                    )}
                  </div>
                ) : (
                  <div className="space-y-2">
                    <Label htmlFor={authKeyId}>Private key</Label>
                    <Textarea
                      id={authKeyId}
                      value={form.auth_key_inline}
                      onChange={(event) =>
                        setForm((current) => ({
                          ...current,
                          auth_key_inline: event.target.value,
                          auth_public_key: "",
                        }))
                      }
                    />
                  </div>
                )}
                {form.key_input_mode === "paste" ? (
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
                ) : null}
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

            {successMessage ? (
              <p className="text-sm text-primary">{successMessage}</p>
            ) : null}
            {error ? <p className="text-sm text-destructive">{error}</p> : null}

            <DialogFooter>
              <Button
                variant="outline"
                onClick={() => {
                  setIsModalOpen(false);
                  setForm(emptyState);
                  setError(null);
                  setSuccessMessage(null);
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

      <ConfirmDialog
        open={deleteTarget !== null}
        title="Delete instance?"
        description={
          deleteTarget
            ? `This will permanently delete ${deleteTarget.name} and remove every user assignment for it.`
            : ""
        }
        confirmLabel="Delete instance"
        isPending={deleteMutation.isPending}
        onOpenChange={(open) => {
          if (!open) {
            setDeleteTarget(null);
          }
        }}
        onConfirm={() => {
          if (deleteTarget) {
            deleteMutation.mutate(deleteTarget.id);
          }
        }}
      />

      <div>
        <Card>
          <CardContent className="p-0">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Instance</TableHead>
                  <TableHead>Upstream</TableHead>
                  <TableHead>Assigned users</TableHead>
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
                      <div className="flex flex-wrap gap-2">
                        {(instance.assigned_users ?? []).length ? (
                          (instance.assigned_users ?? []).map((user) => (
                            <Badge key={user.id} tone="success">
                              {user.display_name || user.email}
                            </Badge>
                          ))
                        ) : (
                          <Badge tone="muted">unassigned</Badge>
                        )}
                      </div>
                    </TableCell>
                    <TableCell>
                      <Badge tone={instance.enabled ? "success" : "muted"}>
                        {instance.enabled ? "enabled" : "disabled"}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-right">
                      <div className="flex justify-end gap-2">
                        <Button
                          variant="ghost"
                          onClick={() => startEdit(instance)}
                        >
                          Edit
                        </Button>
                        <Button
                          variant="ghost"
                          onClick={() => setDeleteTarget(instance)}
                          aria-label={`Delete ${instance.name}`}
                        >
                          <Trash2 className="h-4 w-4" />
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
            {!instancesQuery.data?.length ? (
              <div className="p-6 text-sm text-muted-foreground">
                No instances yet.
              </div>
            ) : null}
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
