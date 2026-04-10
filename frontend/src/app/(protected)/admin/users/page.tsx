"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Plus, Shield } from "lucide-react";
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
import { Switch } from "@/components/ui/switch";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { useAuth } from "@/contexts/auth-context";
import { api, type User } from "@/lib/api";

type UserFormState = {
  id: number | null;
  email: string;
  display_name: string;
  role: "admin" | "user";
  is_active: boolean;
};

const emptyState: UserFormState = {
  id: null,
  email: "",
  display_name: "",
  role: "user",
  is_active: true,
};

export default function AdminUsersPage() {
  const emailId = useId();
  const displayNameId = useId();
  const roleId = useId();
  const { user: currentUser } = useAuth();
  const queryClient = useQueryClient();
  const usersQuery = useQuery({
    queryKey: ["admin-users"],
    queryFn: api.listUsers,
  });

  const [form, setForm] = useState<UserFormState>(emptyState);
  const [error, setError] = useState<string | null>(null);
  const [isModalOpen, setIsModalOpen] = useState(false);

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
      setForm(emptyState);
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

  return (
    <div className="space-y-6">
      <PageHeader
        title="User Management"
        description="Create and manage users who can access the SSH reverse proxy"
        actions={
          <Button
            onClick={() => {
              setForm(emptyState);
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
          setIsModalOpen(false);
          setForm(emptyState);
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
                {form.id ? "Save changes" : "Create user"}
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
                  <TableHead>User</TableHead>
                  <TableHead>Email</TableHead>
                  <TableHead>Role</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead className="w-16 text-right"></TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {usersQuery.data?.map((user) => (
                  <TableRow key={user.id}>
                    <TableCell>
                      <div className="flex items-center gap-3">
                        <Shield className="h-4 w-4 shrink-0" />
                        <div className="flex items-center gap-2">
                          <p className="font-medium">
                            {user.display_name || user.email.split("@")[0]}
                          </p>
                          {user.email === currentUser?.email ? (
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
                      <Badge tone={user.role === "admin" ? "success" : "muted"}>
                        {user.role}
                      </Badge>
                    </TableCell>
                    <TableCell>
                      <Badge tone={user.is_active ? "success" : "muted"}>
                        {user.is_active ? "active" : "disabled"}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-right">
                      <Button
                        variant="ghost"
                        onClick={() => startEdit(user)}
                        aria-label="Edit user"
                      >
                        ...
                      </Button>
                    </TableCell>
                  </TableRow>
                ))}
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
