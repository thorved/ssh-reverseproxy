export type User = {
  id: number;
  email: string;
  display_name: string;
  role: "admin" | "user";
  is_active: boolean;
  last_login_at?: string | null;
};

export type Instance = {
  id: number;
  name: string;
  slug: string;
  description: string;
  assigned_user_ids: number[];
  assigned_users: User[];
  auth_public_key?: string;
  upstream_host: string;
  upstream_port: number;
  upstream_user: string;
  auth_method: "none" | "password" | "key";
  enabled: boolean;
  created_at: string;
  updated_at: string;
};

export type UserInstancesResponse = {
  instances: Instance[];
  ssh_host: string;
  ssh_port: number;
};

export type SSHKey = {
  id: number;
  user_id: number;
  name: string;
  public_key: string;
  fingerprint: string;
  algorithm: string;
  comment: string;
  is_active: boolean;
  created_at: string;
  updated_at: string;
};

export type SSHKeyPayload = {
  name: string;
  public_key: string;
  is_active: boolean;
};

function getApiBaseUrl() {
  return process.env.NEXT_PUBLIC_DEV_API_URL?.trim() ?? "";
}

export function getApiUrl(path: string) {
  return `${getApiBaseUrl()}${path}`;
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(getApiUrl(path), {
    ...init,
    credentials: "include",
    headers: {
      "Content-Type": "application/json",
      ...(init?.headers ?? {}),
    },
    cache: "no-store",
  });

  if (!response.ok) {
    const data = (await response.json().catch(() => null)) as {
      error?: string;
    } | null;
    throw new Error(data?.error ?? `Request failed with ${response.status}`);
  }

  if (response.status === 204) {
    return undefined as T;
  }

  return (await response.json()) as T;
}

export const api = {
  getMe: () => request<User>("/api/auth/me"),
  logout: () =>
    request<{ ok: boolean }>("/api/auth/logout", { method: "POST" }),
  listUsers: () => request<User[]>("/api/admin/users"),
  createUser: (payload: Record<string, unknown>) =>
    request<User>("/api/admin/users", {
      method: "POST",
      body: JSON.stringify(payload),
    }),
  updateUser: (id: number, payload: Record<string, unknown>) =>
    request<User>(`/api/admin/users/${id}`, {
      method: "PATCH",
      body: JSON.stringify(payload),
    }),
  deleteUser: (id: number) =>
    request<{ ok: boolean }>(`/api/admin/users/${id}`, {
      method: "DELETE",
    }),
  listAdminUserSSHKeys: (userId: number) =>
    request<SSHKey[]>(`/api/admin/users/${userId}/ssh-keys`),
  createAdminUserSSHKey: (userId: number, payload: SSHKeyPayload) =>
    request<SSHKey>(`/api/admin/users/${userId}/ssh-keys`, {
      method: "POST",
      body: JSON.stringify(payload),
    }),
  deleteAdminUserSSHKey: (userId: number, keyId: number) =>
    request<{ ok: boolean }>(`/api/admin/users/${userId}/ssh-keys/${keyId}`, {
      method: "DELETE",
    }),
  listAdminInstances: () => request<Instance[]>("/api/admin/instances"),
  createInstance: (payload: Record<string, unknown>) =>
    request<Instance>("/api/admin/instances", {
      method: "POST",
      body: JSON.stringify(payload),
    }),
  updateInstance: (id: number, payload: Record<string, unknown>) =>
    request<Instance>(`/api/admin/instances/${id}`, {
      method: "PATCH",
      body: JSON.stringify(payload),
    }),
  deleteInstance: (id: number) =>
    request<{ ok: boolean }>(`/api/admin/instances/${id}`, {
      method: "DELETE",
    }),
  listUserInstances: () =>
    request<UserInstancesResponse>("/api/user/instances"),
  listSSHKeys: () => request<SSHKey[]>("/api/user/ssh-keys"),
  createSSHKey: (payload: SSHKeyPayload) =>
    request<SSHKey>("/api/user/ssh-keys", {
      method: "POST",
      body: JSON.stringify(payload),
    }),
  updateSSHKey: (id: number, payload: SSHKeyPayload) =>
    request<SSHKey>(`/api/user/ssh-keys/${id}`, {
      method: "PATCH",
      body: JSON.stringify(payload),
    }),
  deleteSSHKey: (id: number) =>
    request<{ ok: boolean }>(`/api/user/ssh-keys/${id}`, {
      method: "DELETE",
    }),
};
