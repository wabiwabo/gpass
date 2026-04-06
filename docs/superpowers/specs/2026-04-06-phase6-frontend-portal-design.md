# Phase 6: Frontend Portal — Design Specification

**Version:** 1.0  
**Date:** 2026-04-06  
**Status:** Draft  
**Depends on:** Phases 1-5 (all backend services)

---

## 1. Overview

Phase 6 adds the citizen-facing web portal (Next.js + TypeScript) and admin dashboard. The citizen portal provides identity management, consent control, corporate roles, and document signing. The admin dashboard provides platform monitoring, developer app management, and system configuration.

### 1.1 Scope

**In scope:**
- Citizen Portal: login, identity dashboard, consent management, corporate entity view, document signing
- Admin Dashboard: platform stats, developer app overview, system health
- Shared UI component library (Tailwind CSS + shadcn/ui)
- BFF integration (all API calls through BFF, never direct to services)
- Responsive design (mobile-first)
- Turborepo monorepo setup for apps/web and apps/admin

**Out of scope (deferred):**
- Flutter mobile app → Phase 2 roadmap
- Embeddable UI components (`<GarudaPassLogin />`) → Phase 3 roadmap
- Interactive API documentation → Phase 7
- Multi-language i18n → future

### 1.2 Key Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Framework | Next.js 15 (App Router) | SSR for SEO, RSC for performance, dominant in ID market |
| Styling | Tailwind CSS + shadcn/ui | Accessible, composable, no runtime CSS overhead |
| State | React Server Components + minimal client state | RSC eliminates most client state; forms use server actions |
| Auth | Server-side session via BFF cookies | No tokens in browser; secure HttpOnly cookies |
| Monorepo | Turborepo | Already in go.work monorepo; Turborepo handles JS/TS build graph |
| Package manager | pnpm | Fast, strict, workspace-native |

---

## 2. Architecture

### 2.1 App Structure

```
apps/
├── web/           # Citizen portal (Next.js)
│   ├── app/
│   │   ├── (auth)/           # Auth pages (login, register, callback)
│   │   ├── (dashboard)/      # Authenticated pages
│   │   │   ├── identity/     # Identity profile, verification status
│   │   │   ├── consent/      # Consent management
│   │   │   ├── corporate/    # Corporate entity & roles
│   │   │   ├── signing/      # Document signing
│   │   │   └── settings/     # Account settings
│   │   ├── layout.tsx
│   │   └── page.tsx          # Landing page
│   ├── components/           # App-specific components
│   ├── lib/                  # BFF API client, utilities
│   └── next.config.ts
│
├── admin/         # Admin dashboard (Next.js)
│   ├── app/
│   │   ├── (auth)/
│   │   └── (dashboard)/
│   │       ├── overview/     # Platform stats
│   │       ├── developers/   # Developer app management
│   │       ├── entities/     # Corporate entity list
│   │       └── system/       # Health, config
│   ├── components/
│   ├── lib/
│   └── next.config.ts
│
packages/
├── ui/            # Shared UI components (shadcn/ui based)
│   ├── components/
│   │   ├── button.tsx
│   │   ├── card.tsx
│   │   ├── input.tsx
│   │   ├── table.tsx
│   │   ├── badge.tsx
│   │   ├── dialog.tsx
│   │   ├── dropdown-menu.tsx
│   │   ├── sidebar.tsx
│   │   └── toast.tsx
│   ├── lib/
│   │   └── utils.ts          # cn() helper
│   ├── package.json
│   └── tsconfig.json
│
├── config/        # Shared configs (tsconfig, eslint, tailwind)
│   ├── tailwind.config.ts
│   ├── tsconfig.base.json
│   └── eslint.config.js
│
turbo.json
pnpm-workspace.yaml
```

### 2.2 API Flow

```
Browser → Next.js (SSR/RSC) → BFF (Go, port 4000) → Internal Services
                              ↑
                    HttpOnly session cookie
                    (no tokens in browser)
```

All data fetching happens server-side via RSC or server actions. The BFF handles OAuth2/OIDC, session management, and CSRF protection. The frontend never calls internal services directly.

---

## 3. Pages

### 3.1 Citizen Portal (apps/web)

| Route | Page | Data Source |
|-------|------|-------------|
| `/` | Landing page | Static |
| `/login` | Login (redirect to Keycloak via BFF) | BFF /auth/login |
| `/register` | Registration start | BFF → Identity |
| `/auth/callback` | OAuth callback handler | BFF /auth/callback |
| `/dashboard` | Identity dashboard | BFF → Identity + GarudaInfo |
| `/dashboard/identity` | Identity profile & verification | BFF → Identity |
| `/dashboard/consent` | Active consents, grant/revoke | BFF → GarudaInfo |
| `/dashboard/corporate` | Corporate entities & roles | BFF → GarudaCorp |
| `/dashboard/signing` | Document signing flow | BFF → GarudaSign |
| `/dashboard/settings` | Account settings | BFF → Identity |

### 3.2 Admin Dashboard (apps/admin)

| Route | Page | Data Source |
|-------|------|-------------|
| `/` | Platform overview (stats cards) | BFF → Portal |
| `/developers` | Developer app list | BFF → Portal |
| `/developers/:id` | App detail + usage | BFF → Portal |
| `/entities` | Corporate entity list | BFF → GarudaCorp |
| `/system` | System health (service status) | BFF → Health endpoints |

---

## 4. Shared UI Components (packages/ui)

Built on shadcn/ui primitives with GarudaPass branding:

| Component | Purpose |
|-----------|---------|
| Button | Primary, secondary, destructive, outline variants |
| Card | Content container with header/description/footer |
| Input | Text input with label, error state, helper text |
| Table | Data table with sort headers |
| Badge | Status indicators (ACTIVE, REVOKED, PENDING) |
| Dialog | Modal dialogs for confirmations |
| DropdownMenu | Action menus |
| Sidebar | Navigation sidebar with collapsible sections |
| Toast | Notifications (success, error, info) |
| StatusBadge | Color-coded status: green (ACTIVE), red (REVOKED), yellow (PENDING) |

---

## 5. BFF API Client

Server-side fetch wrapper (`apps/web/lib/api.ts`):

```typescript
// All calls are server-side only (RSC or server actions)
// Cookie is forwarded from the incoming request to BFF
async function bffFetch<T>(path: string, options?: RequestInit): Promise<T>

// Typed API functions
async function getSession(): Promise<Session | null>
async function getIdentityProfile(): Promise<IdentityProfile>
async function getConsents(): Promise<Consent[]>
async function getCorporateEntities(): Promise<Entity[]>
async function getSigningRequests(): Promise<SigningRequest[]>
```

---

## 6. Implementation Approach

Since the frontend is a separate technology stack (TypeScript/Next.js vs Go), we implement it as a Turborepo workspace alongside the existing Go services.

### 6.1 Turborepo Setup

```json
// turbo.json
{
  "$schema": "https://turbo.build/schema.json",
  "tasks": {
    "build": { "dependsOn": ["^build"], "outputs": [".next/**", "dist/**"] },
    "dev": { "cache": false, "persistent": true },
    "lint": { "dependsOn": ["^build"] },
    "type-check": { "dependsOn": ["^build"] }
  }
}
```

### 6.2 Workspace Config

```yaml
# pnpm-workspace.yaml
packages:
  - "apps/web"
  - "apps/admin"
  - "packages/*"
```

---

## 7. Configuration

```bash
# apps/web/.env.local
NEXT_PUBLIC_APP_NAME=GarudaPass
BFF_URL=http://localhost:4000           # Server-side only
NEXT_PUBLIC_BFF_URL=http://localhost:4000  # Client-side (only for auth redirects)
```

---

## 8. File Count Estimate

| Directory | Files | Purpose |
|-----------|-------|---------|
| apps/web/ | ~25 | Pages, components, lib, config |
| apps/admin/ | ~15 | Pages, components, lib, config |
| packages/ui/ | ~15 | Shared components |
| packages/config/ | ~5 | Shared configs |
| Root | ~5 | turbo.json, pnpm-workspace, package.json |
| **Total** | **~65** | |

---

## 9. Docker Compose (dev mode)

```yaml
  web:
    build:
      context: .
      dockerfile: apps/web/Dockerfile
    ports:
      - "3000:3000"
    environment:
      BFF_URL: http://bff:4000
    depends_on:
      - bff
    networks:
      - gpass-network

  admin:
    build:
      context: .
      dockerfile: apps/admin/Dockerfile
    ports:
      - "3001:3001"
    environment:
      BFF_URL: http://bff:4000
    depends_on:
      - bff
    networks:
      - gpass-network
```
