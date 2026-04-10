"use client";

import { ShieldCheck, SquareTerminal } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { getApiUrl } from "@/lib/api";

export default function LoginPage() {
  return (
    <main className="mx-auto flex min-h-screen max-w-6xl items-center px-4 py-10 sm:px-6 lg:px-8">
      <div className="grid w-full gap-6 lg:grid-cols-[1.25fr_0.85fr]">
        <section className="rounded-[2.5rem] border border-border/70 bg-card/75 p-8 shadow-[0_25px_80px_-40px_rgba(15,23,42,0.45)] sm:p-12">
          <div className="inline-flex items-center gap-2 rounded-full border border-border/80 bg-background/80 px-4 py-2 text-xs font-semibold uppercase tracking-[0.24em] text-muted-foreground">
            <SquareTerminal className="h-3.5 w-3.5" />
            SSH Control Plane
          </div>
          <div className="mt-8 space-y-5">
            <h1 className="max-w-3xl text-4xl font-semibold tracking-tight sm:text-5xl">
              Give every SSH hop a clear owner, a clear target, and a safer
              path.
            </h1>
            <p className="max-w-2xl text-base text-muted-foreground sm:text-lg">
              Sign in with your OIDC provider to manage assignments, publish SSH
              keys, and connect to approved instances using a simple slug-based
              SSH username.
            </p>
          </div>
          <div className="mt-10 grid gap-4 sm:grid-cols-2">
            <Card className="bg-background/80">
              <CardHeader>
                <CardTitle className="text-lg">Admin workflow</CardTitle>
                <CardDescription>
                  Create users, define instances, and assign ownership without
                  juggling raw mapping rows.
                </CardDescription>
              </CardHeader>
            </Card>
            <Card className="bg-background/80">
              <CardHeader>
                <CardTitle className="text-lg">User workflow</CardTitle>
                <CardDescription>
                  Publish your SSH keys once, see your instances, and connect
                  with{" "}
                  <code className="rounded bg-secondary px-1 py-0.5">
                    ssh instance-slug@proxy
                  </code>
                  .
                </CardDescription>
              </CardHeader>
            </Card>
          </div>
        </section>

        <Card className="self-center">
          <CardHeader>
            <div className="inline-flex h-12 w-12 items-center justify-center rounded-2xl bg-primary/10 text-primary">
              <ShieldCheck className="h-6 w-6" />
            </div>
            <CardTitle className="mt-4">Sign in with OIDC</CardTitle>
            <CardDescription>
              Authentication is OIDC-only. Your role and assignments are loaded
              after the provider callback completes.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <Button asChild className="w-full" size="lg">
              <a href={getApiUrl("/api/auth/oidc/login")}>
                Continue to identity provider
              </a>
            </Button>
            <p className="text-sm text-muted-foreground">
              The backend callback sets an HTTP-only session cookie, so the UI
              does not store access tokens in the browser.
            </p>
          </CardContent>
        </Card>
      </div>
    </main>
  );
}
