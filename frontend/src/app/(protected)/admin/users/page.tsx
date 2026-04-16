"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { KeyRound, Plus, Shield, Trash2 } from "lucide-react";
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
import { Switch } from "@/components/ui/switch";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Textarea } from "@/components/ui/textarea";
import { useAuth } from "@/contexts/auth-context";
import { api, type SSHKey, type User } from "@/lib/api";

type UserFormState = {
  id: number | null;
  email: string;
  display_name: string;
  role: "admin" | "user";
  is_active: boolean;
};

type SSHKeyFormState = {
  name: string;
  public_key: string;
  is_active: boolean;
};

const emptyUserState: UserFormState = {
  id: null,
  email: "",
  display_name: "",
  role: "user",
  is_active: true,
};

const emptyKeyState: SSHKeyFormState = {
  name: "",
  public_key: "",
  is_active: true,
};

export default function AdminUsersPage() {
  const emailId = useId();
  const displayNameId = useId();
  const roleId = useId();
  const keyNameId = useId();
  const keyPublicKeyId = useId();
  const { user: currentUser } = useAuth();
  const queryClient = useQueryClient();
  const usersQuery = useQuery({
    queryKey: ["admin-users"],
    queryFn: api.listUsers,
  });

  const [form, setForm] = useState<UserFormState>(emptyUserState);
  const [error, setError] = useState<string | null>(null);
  const [isModalOpen, setIsModalOpen] = useState(false);
  const [keysModalUser, setKeysModalUser] = useState<User | null>(null);
  const [keyForm, setKeyForm] = useState<SSHKeyFormState>(emptyKeyState);
  const [keyError, setKeyError] = useState<string | null>(null);
  const [deleteUserTarget, setDeleteUserTarget] = useState<User | null>(null);
  const [deleteKeyTarget, setDeleteKeyTarget] = useState<SSHKey | null>(null);

  const keysQuery = useQuery({
    queryKey: ["admin-user-keys", keysModalUser?.id],
    queryFn: async () => {
      if (!keysModalUser) {
        throw new Error("Select a user first");
      }
      return api.listAdminUserSSHKeys(keysModalUser.id);
    },
    enabled: keysModalUser !== null,
  });

  const saveMutation = useMutation({
    mutationFn: async (payload: UserFormState) => {
      if (payload.id) {
        return api.updateUser(payload.id, {
          display_name: payload.display_name,
          role: payload.role,
          is_active: payload.is_active,
        });
      }
      return api.createUser(payload);
    },
    onSuccess: () => {
      setForm(emptyUserState);
      setError(null);
      setIsModalOpen(false);
      void queryClient.invalidateQueries({ queryKey: ["admin-users"] });
    },
    onError: (mutationError) => {
      setError(
        mutationError instanceof Error ? mutationError.message : "Save failed",
      );
    },
  });

  const deleteUserMutation = useMutation({
    mutationFn: (userId: number) => api.deleteUser(userId),
    onSuccess: () => {
      setDeleteUserTarget(null);
      void queryClient.invalidateQueries({ queryKey: ["admin-users"] });
      void queryClient.invalidateQueries({ queryKey: ["admin-instances"] });
    },
  });

  const createKeyMutation = useMutation({
    mutationFn: () => {
      if (!keysModalUser) {
        throw new Error("Select a user first");
      }
      return api.createAdminUserSSHKey(keysModalUser.id, keyForm);
    },
    onSuccess: () => {
      setKeyForm(emptyKeyState);
      setKeyError(null);
      void queryClient.invalidateQueries({
        queryKey: ["admin-user-keys", keysModalUser?.id],
      });
    },
    onError: (mutationError) => {
      setKeyError(
        mutationError instanceof Error ? mutationError.message : "Save failed",
      );
    },
  });

  const deleteKeyMutation = useMutation({
    mutationFn: (keyId: number) => {
      if (!keysModalUser) {
        throw new Error("Select a user first");
      }
      return api.deleteAdminUserSSHKey(keysModalUser.id, keyId);
    },
    onSuccess: () => {
      setDeleteKeyTarget(null);
      void queryClient.invalidateQueries({
        queryKey: ["admin-user-keys", keysModalUser?.id],
      });
    },
  });

  const startEdit = (user: User) => {
    setForm({
      id: user.id,
      email: user.email,
      display_name: user.display_name,
      role: user.role,
      is_active: user.is_active,
    });
    setError(null);
    setIsModalOpen(true);
  };

  const openKeysModal = (user: User) => {
    setKeysModalUser(user);
    setKeyForm(emptyKeyState);
    setKeyError(null);
    setDeleteKeyTarget(null);
  };

  return (
    <div className="space-y-6">
      <PageHeader
        title="User Management"
        description="Create users, manage sign-in state, and control their published SSH keys."
        actions={
          <Button
            onClick={() => {
              setForm(emptyUserState);
              setError(null);
              setIsModalOpen(true);
            }}
          >
            <Plus className="h-4 w-4" />
            Create User
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
          setForm(emptyUserState);
          setError(null);
        }}
      >
        <DialogContent className="sm:max-w-lg">
          <DialogHeader>
            <DialogTitle>{form.id ? "Edit user" : "Create user"}</DialogTitle>
            <DialogDescription>
              Add a user account, update role access, or disable sign-in.
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor={emailId}>Email</Label>
              <Input
                id={emailId}
                value={form.email}
                disabled={!!form.id}
                onChange={(event) =>
                  setForm((current) => ({
                    ...current,
                    email: event.target.value,
                  }))
                }
                placeholder="name@example.com"
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor={displayNameId}>Display name</Label>
              <Input
                id={displayNameId}
                value={form.display_name}
                onChange={(event) =>
                  setForm((current) => ({
                    ...current,
                    display_name: event.target.value,
                  }))
                }
                placeholder="Optional friendly name"
              />
            </div>

            <div className="grid gap-4 sm:grid-cols-[minmax(0,1fr)_220px] sm:items-end">
              <div className="space-y-2">
                <Label htmlFor={roleId}>Role</Label>
                <Select
                  id={roleId}
                  value={form.role}
                  onChange={(event) =>
                    setForm((current) => ({
                      ...current,
                      role: event.target.value as UserFormState["role"],
                    }))
                  }
                >
                  <option value="user">User</option>
                  <option value="admin">Admin</option>
                </Select>
              </div>

              <div className="rounded-xl border border-border bg-muted/30 px-4 py-3">
                <div className="flex items-center justify-between gap-3">
                  <div className="space-y-1">
                    <p className="text-sm font-medium">Active account</p>
                    <p className="text-xs text-muted-foreground">
                      Allow this user to sign in with OIDC.
                    </p>
                  </div>
                  <Switch
                    checked={form.is_active}
                    onCheckedChange={(checked) =>
                      setForm((current) => ({
                        ...current,
                        is_active: checked,
                      }))
                    }
                    aria-label="Toggle active account"
                  />
                </div>
              </div>
            </div>

            {error ? <p className="text-sm text-destructive">{error}</p> : null}

            <DialogFooter>
              <Button
                variant="outline"
                onClick={() => {
                  setIsModalOpen(false);
                  setForm(emptyUserState);
                  setError(null);
                }}
              >
                Cancel
              </Button>
              <Button
                onClick={() => saveMutation.mutate(form)}
                disabled={saveMutation.isPending}
              >
                {form.id ? "Save changes" : "Create user"}
              </Button>
            </DialogFooter>
          </div>
        </DialogContent>
      </Dialog>

      <Dialog
        open={keysModalUser !== null}
        onOpenChange={(open) => {
          if (!open) {
            setKeysModalUser(null);
            setKeyForm(emptyKeyState);
            setKeyError(null);
            setDeleteKeyTarget(null);
          }
        }}
      >
        <DialogContent className="sm:max-w-3xl">
          <DialogHeader>
            <DialogTitle>Manage user keys</DialogTitle>
            <DialogDescription>
              {keysModalUser
                ? `Add or remove SSH keys for ${
                    keysModalUser.display_name || keysModalUser.email
                  }.`
                : "Add or remove SSH keys for this user."}
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-6 overflow-x-hidden">
            <div className="space-y-4 rounded-2xl border border-border p-4">
              <div className="space-y-2">
                <Label htmlFor={keyNameId}>Key name</Label>
                <Input
                  id={keyNameId}
                  value={keyForm.name}
                  onChange={(event) =>
                    setKeyForm((current) => ({
                      ...current,
                      name: event.target.value,
                    }))
                  }
                  placeholder="Laptop, workstation, deploy key"
                />
              </div>

              <div className="space-y-2">
                <Label htmlFor={keyPublicKeyId}>Public key</Label>
                <Textarea
                  id={keyPublicKeyId}
                  value={keyForm.public_key}
                  onChange={(event) =>
                    setKeyForm((current) => ({
                      ...current,
                      public_key: event.target.value,
                    }))
                  }
                  placeholder="ssh-ed25519 AAAAC3..."
                />
              </div>

              <label className="flex items-center gap-3 rounded-xl border border-border bg-background px-4 py-3 text-sm">
                <input
                  type="checkbox"
                  checked={keyForm.is_active}
                  onChange={(event) =>
                    setKeyForm((current) => ({
                      ...current,
                      is_active: event.target.checked,
                    }))
                  }
                />
                Keep this key active for login
              </label>

              {keyError ? (
                <p className="text-sm text-destructive">{keyError}</p>
              ) : null}

              <div className="flex justify-end">
                <Button
                  onClick={() => createKeyMutation.mutate()}
                  disabled={createKeyMutation.isPending || !keysModalUser}
                >
                  Add key
                </Button>
              </div>
            </div>

            <div className="space-y-3 rounded-2xl border border-border p-3 sm:p-4">
              {keysQuery.data?.length ? (
                keysQuery.data.map((key) => (
                  <div
                    key={key.id}
                    className="space-y-4 rounded-2xl border border-border bg-muted/20 p-4"
                  >
                    <div className="flex items-start justify-between gap-3">
                      <div className="min-w-0 space-y-1">
                        <p className="font-medium break-words">{key.name}</p>
                        <p className="break-words text-xs text-muted-foreground">
                          {key.comment || "No comment"}
                        </p>
                      </div>
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => setDeleteKeyTarget(key)}
                        aria-label={`Delete ${key.name}`}
                      >
                        <Trash2 className="h-4 w-4" />
                      </Button>
                    </div>

                    <div className="grid gap-4 sm:grid-cols-[minmax(0,140px)_minmax(0,1fr)_auto]">
                      <div className="min-w-0 space-y-1">
                        <p className="text-xs font-semibold uppercase tracking-[0.16em] text-muted-foreground">
                          Algorithm
                        </p>
                        <p className="break-words text-sm">{key.algorithm}</p>
                      </div>

                      <div className="min-w-0 space-y-1">
                        <p className="text-xs font-semibold uppercase tracking-[0.16em] text-muted-foreground">
                          Fingerprint
                        </p>
                        <p className="break-all font-mono text-xs leading-5 text-foreground/90">
                          {key.fingerprint}
                        </p>
                      </div>

                      <div className="min-w-0 space-y-1">
                        <p className="text-xs font-semibold uppercase tracking-[0.16em] text-muted-foreground">
                          Status
                        </p>
                        <Badge tone={key.is_active ? "success" : "muted"}>
                          {key.is_active ? "active" : "inactive"}
                        </Badge>
                      </div>
                    </div>
                  </div>
                ))
              ) : (
                <p className="p-2 text-sm text-muted-foreground">
                  No SSH keys uploaded for this user yet.
                </p>
              )}
            </div>
          </div>
        </DialogContent>
      </Dialog>

      <ConfirmDialog
        open={deleteUserTarget !== null}
        title="Delete user?"
        description={
          deleteUserTarget
            ? `This will permanently delete ${
                deleteUserTarget.display_name || deleteUserTarget.email
              } and remove all of their SSH keys.`
            : ""
        }
        confirmLabel="Delete user"
        isPending={deleteUserMutation.isPending}
        onOpenChange={(open) => {
          if (!open) {
            setDeleteUserTarget(null);
          }
        }}
        onConfirm={() => {
          if (deleteUserTarget) {
            deleteUserMutation.mutate(deleteUserTarget.id);
          }
        }}
      />

      <ConfirmDialog
        open={deleteKeyTarget !== null}
        title="Delete SSH key?"
        description={
          deleteKeyTarget
            ? `This will remove ${deleteKeyTarget.name} from this user.`
            : ""
        }
        confirmLabel="Delete key"
        isPending={deleteKeyMutation.isPending}
        onOpenChange={(open) => {
          if (!open) {
            setDeleteKeyTarget(null);
          }
        }}
        onConfirm={() => {
          if (deleteKeyTarget) {
            deleteKeyMutation.mutate(deleteKeyTarget.id);
          }
        }}
      />

      <div>
        <Card>
          <CardContent className="p-0">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>User</TableHead>
                  <TableHead>Email</TableHead>
                  <TableHead>Role</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead className="w-40 text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {usersQuery.data?.map((user) => {
                  const isCurrentUser = user.email === currentUser?.email;
                  return (
                    <TableRow key={user.id}>
                      <TableCell>
                        <div className="flex items-center gap-3">
                          <Shield className="h-4 w-4 shrink-0" />
                          <div className="flex items-center gap-2">
                            <p className="font-medium">
                              {user.display_name || user.email.split("@")[0]}
                            </p>
                            {isCurrentUser ? (
                              <span className="rounded-full border border-border px-2 py-0.5 text-xs">
                                You
                              </span>
                            ) : null}
                          </div>
                        </div>
                      </TableCell>
                      <TableCell>
                        <span className="text-muted-foreground">
                          {user.email}
                        </span>
                      </TableCell>
                      <TableCell>
                        <Badge
                          tone={user.role === "admin" ? "success" : "muted"}
                        >
                          {user.role}
                        </Badge>
                      </TableCell>
                      <TableCell>
                        <Badge tone={user.is_active ? "success" : "muted"}>
                          {user.is_active ? "active" : "disabled"}
                        </Badge>
                      </TableCell>
                      <TableCell className="text-right">
                        <div className="flex justify-end gap-2">
                          <Button
                            variant="ghost"
                            onClick={() => openKeysModal(user)}
                            aria-label={`Manage keys for ${user.email}`}
                          >
                            <KeyRound className="h-4 w-4" />
                          </Button>
                          <Button
                            variant="ghost"
                            onClick={() => startEdit(user)}
                            aria-label={`Edit ${user.email}`}
                          >
                            Edit
                          </Button>
                          <Button
                            variant="ghost"
                            onClick={() => setDeleteUserTarget(user)}
                            aria-label={`Delete ${user.email}`}
                            disabled={isCurrentUser}
                          >
                            <Trash2 className="h-4 w-4" />
                          </Button>
                        </div>
                      </TableCell>
                    </TableRow>
                  );
                })}
              </TableBody>
            </Table>
            {!usersQuery.data?.length ? (
              <div className="p-6 text-sm text-muted-foreground">
                No users yet.
              </div>
            ) : null}
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
