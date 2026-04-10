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
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Textarea } from "@/components/ui/textarea";
import { api, type SSHKey } from "@/lib/api";

type SSHKeyFormState = {
  id: number | null;
  name: string;
  public_key: string;
  is_active: boolean;
};

const emptyState: SSHKeyFormState = {
  id: null,
  name: "",
  public_key: "",
  is_active: true,
};

export default function SSHKeysPage() {
  const nameId = useId();
  const publicKeyId = useId();
  const queryClient = useQueryClient();
  const keysQuery = useQuery({
    queryKey: ["ssh-keys"],
    queryFn: api.listSSHKeys,
  });

  const [form, setForm] = useState<SSHKeyFormState>(emptyState);
  const [error, setError] = useState<string | null>(null);
  const [isModalOpen, setIsModalOpen] = useState(false);

  const saveMutation = useMutation({
    mutationFn: async (payload: SSHKeyFormState) => {
      if (payload.id) {
        return api.updateSSHKey(payload.id, payload);
      }
      return api.createSSHKey(payload);
    },
    onSuccess: () => {
      setForm(emptyState);
      setError(null);
      setIsModalOpen(false);
      void queryClient.invalidateQueries({ queryKey: ["ssh-keys"] });
    },
    onError: (mutationError) => {
      setError(
        mutationError instanceof Error ? mutationError.message : "Save failed",
      );
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (id: number) => api.deleteSSHKey(id),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["ssh-keys"] });
      if (form.id) {
        setForm(emptyState);
      }
      setIsModalOpen(false);
    },
  });

  const setEditing = (key: SSHKey) => {
    setForm({
      id: key.id,
      name: key.name,
      public_key: key.public_key,
      is_active: key.is_active,
    });
    setError(null);
    setIsModalOpen(true);
  };

  return (
    <div className="space-y-6">
      <PageHeader
        eyebrow="Identity"
        title="SSH Keys"
        description="Publish the keys the proxy should trust for your account. Fingerprints are unique globally, so a key belongs to exactly one user."
        actions={
          <Button
            onClick={() => {
              setForm(emptyState);
              setError(null);
              setIsModalOpen(true);
            }}
          >
            <Plus className="h-4 w-4" />
            Add Key
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
        <DialogContent className="sm:max-w-2xl">
          <DialogHeader>
            <DialogTitle>
              {form.id ? "Edit SSH key" : "Add SSH key"}
            </DialogTitle>
            <DialogDescription>
              Publish a new SSH public key or update an existing one.
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-4">
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
                placeholder="Laptop, workstation, deploy key"
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor={publicKeyId}>Public key</Label>
              <Textarea
                id={publicKeyId}
                value={form.public_key}
                onChange={(event) =>
                  setForm((current) => ({
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
                checked={form.is_active}
                onChange={(event) =>
                  setForm((current) => ({
                    ...current,
                    is_active: event.target.checked,
                  }))
                }
              />
              Keep this key active for login
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
                {form.id ? "Save changes" : "Add key"}
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
                  <TableHead>Name</TableHead>
                  <TableHead>Algorithm</TableHead>
                  <TableHead>Fingerprint</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead className="text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {keysQuery.data?.map((key) => (
                  <TableRow key={key.id}>
                    <TableCell>
                      <div>
                        <p className="font-medium">{key.name}</p>
                        <p className="text-xs text-muted-foreground">
                          {key.comment}
                        </p>
                      </div>
                    </TableCell>
                    <TableCell>{key.algorithm}</TableCell>
                    <TableCell className="font-mono text-xs">
                      {key.fingerprint}
                    </TableCell>
                    <TableCell>
                      <Badge tone={key.is_active ? "success" : "muted"}>
                        {key.is_active ? "active" : "inactive"}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-right">
                      <div className="flex justify-end gap-2">
                        <Button variant="ghost" onClick={() => setEditing(key)}>
                          Edit
                        </Button>
                        <Button
                          variant="destructive"
                          onClick={() => deleteMutation.mutate(key.id)}
                          disabled={deleteMutation.isPending}
                        >
                          Delete
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
            {!keysQuery.data?.length ? (
              <p className="p-6 text-sm text-muted-foreground">
                No SSH keys uploaded yet.
              </p>
            ) : null}
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
