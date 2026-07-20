# Platform Integration Plan: TitanOps

## Vision

We don't compete with observability platforms. We add autonomous capabilities that plug into whatever stack people already use. The more integrations, the better.

**Name:** TitanOps — Autonomous AiOps modules for Kubernetes

**Position:** "Your observability stack tells you what's wrong. TitanOps fixes it."

---

## The Portfolio

| Module | Domain | What It Does | Tech |
|--------|--------|-------------|------|
| **Earthworm** | Health | K8s cluster heartbeat monitoring via eBPF | Go + TypeScript + C |
| **Tlapix** | Security/Compliance | Autonomous TLS certificate lifecycle guardian | Rust + Aya |
| **eBeeControl** | Threat Detection | Autonomous deception engine (honeytokens) | TypeScript + Gemini |
| **Quack** | Performance | AI-powered container CPU scheduling | Go + sched_ext |

All four share the same architecture pattern:

```
eBPF (kernel observation) → AI (analysis/decision) → Autonomous Action
```

---

## Integration Strategy

We are NOT building:
- A metrics database (use VictoriaMetrics/Prometheus)
- A log aggregator (use Loki/Elastic)
- A tracing backend (use Tempo/Jaeger)
- A dashboard engine (use Grafana)
- An alerting system (use Alertmanager/PagerDuty)

We ARE building:
- Autonomous eBPF-powered capabilities that generate insights
- AI that makes decisions without humans
- Actions that execute at kernel speed
- A correlation layer that connects signals no single tool can see

---

## Target Integrations

| Backend | How We Integrate | Status |
|---------|-----------------|--------|
| Prometheus / VictoriaMetrics | `/metrics` endpoint (Prometheus exposition format) | Tlapix ✅, Earthworm ✅, eBeeControl ❌, Quack ❌ |
| Grafana | Pre-built dashboard JSON + optional plugin | Planned |
| Grafana Cloud | Remote write | Planned |
| Datadog | OTLP export | Tlapix ✅ |
| Splunk | HEC export | Quack ✅ |
| Dynatrace | API push | eBeeControl ✅ |
| PagerDuty / OpsGenie | Webhook alerts | Tlapix ✅ |
| Slack / Teams | Webhook alerts | Tlapix ✅ |
| Elastic / OpenSearch | OTLP or Filebeat | Planned |
| OpenTelemetry Collector | OTLP native | Tlapix ✅ |

---

## Phase 1: Make Each Module Independently Installable (2-3 weeks)

**Goal:** Anyone can `helm install` one module and get value in 5 minutes.

| Task | What | Why |
|------|------|-----|
| 1.1 | Each project has a working Helm chart | One command install |
| 1.2 | Each project exports Prometheus metrics at `/metrics` | Works with any stack |
| 1.3 | Each project ships a Grafana dashboard JSON | Instant visibility |
| 1.4 | Each project has a 3-minute demo video | People don't read, they watch |
| 1.5 | Each project has a `values.yaml` that works with zero config | Reduce friction to zero |

### Current Status

| Module | Helm Chart | Prometheus | Grafana Dashboard | Demo Video |
|--------|-----------|-----------|-------------------|-----------|
| Tlapix | ✅ | ✅ | ❌ | ❌ |
| Earthworm | ✅ (deploy/helm) | ✅ | ❌ | ❌ |
| eBeeControl | ✅ | ❌ (Dynatrace only) | ❌ | ❌ |
| Quack | ❌ (K8s manifests) | ❌ (Splunk HEC only) | ❌ | ❌ |

### Priority Fixes

1. **eBeeControl**: Add Prometheus metrics exporter alongside Dynatrace
2. **Quack**: Add Prometheus metrics exporter alongside Splunk HEC
3. **Quack**: Create proper Helm chart (currently raw manifests)
4. **All**: Create Grafana dashboard JSON files

---

## Phase 2: Unified Helm Chart + Shared Identity (1-2 weeks)

**Goal:** One chart installs all four modules (each toggleable).

```bash
helm install titanops titanops/titanops \
  --set tlapix.enabled=true \
  --set earthworm.enabled=true \
  --set ebeecontrol.enabled=false \
  --set quack.enabled=false
```

| Task | What |
|------|------|
| 2.1 | Create umbrella Helm chart that depends on the four sub-charts |
| 2.2 | Shared ServiceAccount + RBAC (one ClusterRole for all modules) |
| 2.3 | Shared ConfigMap for common settings (cluster name, OTLP endpoint) |
| 2.4 | One DaemonSet that loads all eBPF programs (instead of four) |
| 2.5 | Landing page / documentation site |

### Umbrella Chart Structure

```
helm/titanops/
├── Chart.yaml                    # Umbrella chart
├── values.yaml                   # Global + per-module config
├── charts/
│   ├── tlapix/                   # Sub-chart (from tlapix repo)
│   ├── earthworm/                # Sub-chart (from earthworm repo)
│   ├── ebeecontrol/              # Sub-chart (from ebeecontrol repo)
│   └── quack/                    # Sub-chart (from quack repo)
└── templates/
    ├── shared-rbac.yaml          # Unified RBAC
    ├── shared-configmap.yaml     # Common config
    └── correlation-service.yaml  # Phase 5 component
```

---

## Phase 3: Grafana Dashboard Pack (1 week)

**Goal:** Import or install dashboards that show all modules.

| Dashboard | Key Panels |
|-----------|-----------|
| **Overview** | Module health, events/sec, active alerts, AI decisions |
| **Certificates (Tlapix)** | Cert inventory, expiry timeline, shadow certs, anomalies |
| **Heartbeat (Earthworm)** | Node cardiogram, anomaly heatmap, prediction timeline |
| **Deception (eBeeControl)** | Honeytoken map, access events, threat classification |
| **Scheduling (Quack)** | Priority decisions, latency impact, model confidence |
| **Correlation** | Cross-module events on shared timeline |

### Distribution Options

1. Grafana dashboard JSON files in each repo's `grafana/` directory
2. Grafana plugin (more polished, discoverable in marketplace)
3. Published to Grafana.com dashboard library (free, searchable)

---

## Phase 4: Integration Adapters (2-3 weeks)

**Goal:** Work with whatever backend the customer already has.

Each module gets a unified export configuration:

```yaml
# In Helm values.yaml
export:
  prometheus:
    enabled: true
    port: 9090
  otlp:
    enabled: false
    endpoint: ""
  splunk:
    enabled: false
    hecUrl: ""
    hecToken: ""
  dynatrace:
    enabled: false
    apiUrl: ""
    apiToken: ""
  webhooks:
    - endpoint: "https://hooks.slack.com/..."
      events: ["critical", "high"]
```

The key: every module supports every backend. Customer picks what they have.

---

## Phase 5: Correlation Engine (3-4 weeks)

**Goal:** Connect signals across modules automatically. This is the paid differentiator.

### How It Works

```
Earthworm: "Node X heartbeat degraded"         (t=0)
Tlapix:    "Shadow cert appeared on Node X"     (t=+30s)
eBeeControl: "Honeytoken accessed from pod Y"   (t=+45s)
Quack:     "Pod Y consuming 95% CPU"            (t=+60s)

Correlation Engine:
  → Match by node_name="X" within 2-minute window
  → Generate: "Possible crypto-miner on Node X (confidence: 92%)"
  → Auto-action: Isolate pod Y, alert operator, forensic report
```

### Components

| Component | What |
|-----------|------|
| Shared event schema | All modules emit events with `node`, `pod`, `namespace`, `timestamp`, `severity`, `module` |
| Event bus | Lightweight in-cluster gRPC service or shared NATS/Redis stream |
| Correlation rules | "If A + B + C within N minutes → correlated incident" |
| AI layer | Model trained on correlated incidents to predict attack chains |
| Output | Correlated alerts with full narrative, sent to configured backends |

### Why This Is the Moat

No single observability tool can do this because:
- Datadog doesn't have eBPF-based deception
- Grafana doesn't have autonomous certificate renewal
- Splunk doesn't have kernel-level scheduling
- VictoriaMetrics doesn't have AI-driven action execution

The correlation of signals across these domains is unique. That's what you charge for.

---

## Phase 6: Go-to-Market

| Channel | Action | Timeline |
|---------|--------|----------|
| GitHub | All repos public, good READMEs, stars campaign | Now |
| Artifact Hub | Publish Helm charts (K8s community discovers them) | Phase 2 |
| Grafana Marketplace | Publish dashboard pack | Phase 3 |
| Blog posts | One per module + one platform overview | Phase 3-4 |
| Conference CFPs | KubeCon, eBPF Summit, GrafanaCon, Splunk .conf | Phase 4 |
| YouTube | 3-min demo per module + 10-min platform overview | Phase 3 |
| Hacker News | 47-day certificate mandate angle (Tlapix) | Phase 1 |
| CNCF Landscape | Apply for listing under Observability → AIOps | Phase 4 |
| Product Hunt | Launch the unified platform | Phase 5 |

---

## Business Model

| Tier | What | Price |
|------|------|-------|
| **Open Source** | All four modules, Helm charts, Grafana dashboards | Free |
| **Pro** | Correlation engine, managed AI models, priority support | $X/node/month |
| **Enterprise** | Custom integrations, SLA, dedicated support, training | Contact |

The open source modules drive adoption. The correlation engine drives revenue.

---

## Timeline Summary

| Phase | Duration | Outcome |
|-------|----------|---------|
| 1. Individual modules ready | 2-3 weeks | Each module installable and demo-able |
| 2. Unified chart | 1-2 weeks | One `helm install` for everything |
| 3. Grafana dashboards | 1 week | Visual proof of value |
| 4. Integration adapters | 2-3 weeks | Works with any backend |
| 5. Correlation engine | 3-4 weeks | The paid differentiator |
| 6. Go-to-market | Ongoing | Visibility and adoption |

**Total to MVP platform: ~10 weeks.**

---

## Architecture Decision: Keep Projects Separate

The four projects use different languages (Rust, Go, TypeScript). Don't rewrite them. Connect them via:

1. **Shared event schema** (protobuf or JSON with common fields)
2. **OTLP as the universal transport** (all modules emit, any backend consumes)
3. **Umbrella Helm chart** (installs sub-charts as dependencies)
4. **Correlation service** (new, small, reads from all four)

Each project continues to evolve independently. The platform is the orchestration layer on top.

---

## Key Principle

> "We don't replace your observability stack. We give it superpowers."

The customer keeps Grafana, keeps Prometheus, keeps their alerts. They add our modules and suddenly their cluster has autonomous certificate management, deception-based threat detection, AI scheduling, and heartbeat monitoring — all feeding into the tools they already trust.

---

## AI/ML Strategy: Local-First, Cloud-Optional

### The Problem Today

Each module uses a different AI approach, creating vendor lock-in and inconsistency:

| Module | Current AI/ML | Dependency |
|--------|--------------|-----------|
| Tlapix | ONNX Runtime (local) | None (correct approach) |
| eBeeControl | Gemini (Google Cloud) | Requires Google Cloud |
| Quack | Splunk AITK | Requires Splunk |
| Earthworm | Server-side rules | No real ML |

Four modules, three different AI strategies, two vendor lock-ins. That's not a platform.

### The Unified Architecture

TitanOps needs a single AI layer with a pluggable backend:

```
┌─────────────────────────────────────────────────────────────┐
│                    TitanOps AI Layer                          │
│                                                              │
│  ┌─────────────────────────────────────────────────────┐    │
│  │              AI Provider Interface                    │    │
│  │                                                      │    │
│  │  train(data) → model                                 │    │
│  │  predict(features) → score                           │    │
│  │  explain(decision) → reasoning                       │    │
│  └──────────┬──────────┬──────────┬──────────┬─────────┘    │
│             │          │          │          │               │
│        ┌────┴───┐ ┌────┴───┐ ┌────┴───┐ ┌───┴────┐         │
│        │ Local  │ │ Gemini │ │Bedrock │ │ Splunk │         │
│        │ (ONNX) │ │(Google)│ │ (AWS)  │ │ (AITK) │         │
│        └────────┘ └────────┘ └────────┘ └────────┘         │
│                                                              │
│  Default: Local ONNX (free, private, no dependency)          │
│  Optional: Cloud providers for enhanced capabilities         │
└─────────────────────────────────────────────────────────────┘
```

### The Three Tiers

| Tier | What | Cost | When to Use |
|------|------|------|-------------|
| **Tier 1: Local ONNX** | Isolation Forest, XGBoost, simple models | $0 | Default. Always works. No internet needed. |
| **Tier 2: Cloud ML** | Vertex AI, Bedrock, SageMaker | $$$ | When customer wants better models, has budget |
| **Tier 3: LLM** | Gemini, Claude, GPT | $$$$ | Explanation generation, incident reports, natural language queries |

### What Runs Where

| Function | Where It Runs | Why |
|----------|--------------|-----|
| Inference (scoring, prediction) | Always local | Latency-sensitive, must work offline |
| Decision making | Always local | Can't depend on cloud availability for actions |
| Action execution | Always local (BPF maps) | Kernel-speed, no network round-trip |
| Model training | Cloud (optional) | Offline, not in hot path, benefits from scale |
| Explanation generation | Cloud (optional) | Nice-to-have, not critical path |
| Complex reasoning/correlation | Local default, cloud optional | Latency matters for correlation |

### Module Migration Plan

| Module | Today | TitanOps Unified |
|--------|-------|-----------------|
| Tlapix | ONNX (local) | ✅ Keep as-is. This is the reference implementation. |
| eBeeControl | Gemini (required) | Refactor: Local rules + ONNX default, Gemini optional for explanations |
| Quack | Splunk AITK (required) | Refactor: Local scoring default, Splunk AITK optional for training |
| Earthworm | Rules only | Add: Local ONNX anomaly model (same as Tlapix approach) |

### Unified Configuration

```yaml
# titanops-values.yaml
ai:
  # Default provider (works offline, free, no vendor dependency)
  provider: local
  local:
    modelPath: /opt/titanops/models/
    # Each module has its own model file:
    # tlapix-anomaly.onnx
    # earthworm-anomaly.onnx
    # ebeecontrol-threat.onnx
    # quack-priority.onnx

  # Optional: Cloud provider for enhanced capabilities
  # Uncomment ONE of the following:

  # cloud:
  #   provider: gemini
  #   gemini:
  #     apiKey: ""
  #     model: "gemini-2.0-flash"
  #
  #   provider: bedrock
  #   bedrock:
  #     region: "us-east-1"
  #     modelId: "anthropic.claude-3-haiku"
  #
  #   provider: vertex
  #   vertex:
  #     project: "my-project"
  #     location: "us-central1"
  #
  #   provider: sagemaker
  #   sagemaker:
  #     region: "us-east-1"
  #     endpointName: "titanops-model"
  #
  #   provider: splunk
  #   splunk:
  #     searchUrl: "https://splunk:8089"
  #     modelName: "titanops_model"

  # What cloud is used for (never the hot path)
  cloudUsage:
    training: true          # Retrain models in the cloud
    explanations: true      # Generate human-readable explanations
    correlation: false      # Keep correlation local (latency-sensitive)
```

### Key Principles

1. **Local-first**: The product works out of the box with zero cloud dependencies. Install via Helm, it runs.
2. **Cloud-optional**: Customers who want better models, explanations, or training pipelines can plug in their preferred cloud AI.
3. **Vendor-neutral**: We don't pick the cloud for them. Gemini, Bedrock, Vertex, SageMaker, Splunk AITK — all supported.
4. **Hot path is always local**: Inference, decisions, and actions never depend on network calls. Cloud is only for offline training and optional enrichment.
5. **Graceful degradation**: If cloud is configured but unavailable, fall back to local models (same pattern Tlapix already implements).

### Why This Matters Strategically

- **No vendor lock-in for customers**: They choose their cloud. Or no cloud at all.
- **No vendor lock-in for us**: We're not dependent on Google, AWS, or Splunk for our core product to work.
- **Enterprise-friendly**: Large companies already have Bedrock/Vertex/SageMaker. They want TitanOps to use THEIR cloud AI, not bring its own.
- **Startup-friendly**: Small teams with no cloud budget get the full product for free (local ONNX).
- **Air-gap compatible**: Government, defense, regulated industries can run TitanOps without any internet connectivity.

### The Tlapix Model as Reference

Tlapix already implements this correctly:
- Local ONNX Runtime for inference ($0, no dependency)
- Rule-based fallback when model unavailable
- AI backend gated behind a feature flag
- Model trained offline, deployed as a file

This pattern scales to all four modules. Tlapix is the blueprint.

---

## UI Strategy: Product First, Integrations Second

### Priority Order

| Priority | What | Why |
|----------|------|-----|
| **1** | TitanOps React Dashboard | The product. Works standalone. Shows decisions, actions, reasoning. |
| **2** | Integration exports (Prometheus, OTLP, webhooks) | Data flows out to whatever they have. |
| **3** | Intelligence injection into existing platforms | Push AI reasoning INTO their Grafana/Datadog/Splunk — not just metrics. |

### Why Our Own Dashboard First

The TitanOps dashboard shows what no integration can:
- Autonomous decision chain (observation → analysis → action)
- Cross-module correlation timeline
- AI confidence scores and reasoning
- Human override controls
- "Why did TitanOps do X?" explainability

Grafana shows time-series charts. Datadog shows metrics. Our dashboard shows **decisions and actions** — that's the product.

### What the Dashboard Is NOT

- NOT a metrics dashboard (use Grafana/Datadog for that)
- NOT a log viewer (use Loki/Elastic for that)
- NOT a replacement for existing tools

### What the Dashboard IS

A **command center for autonomous operations**:
- Module health status (all four at a glance)
- Recent autonomous actions with reasoning
- Cross-module correlation timeline
- AI predictions and confidence
- Human override / approval controls
- Audit trail for compliance

### Integration as Intelligence Injection

Priority 3 isn't "push metrics to Grafana." It's "push INTELLIGENCE to their tools":

| Platform | What We Push | Value Added |
|----------|-------------|-------------|
| Grafana | Annotations on their panels showing AI decisions | "At 14:32, TitanOps renewed this cert because..." |
| Datadog | Custom events with full reasoning chain | Rich context no other data source provides |
| Splunk | Saved searches powered by our model outputs | AI-driven alerts, not just threshold alerts |
| PagerDuty | Rich alerts with decision chain + confidence | Responders know WHY before they look |

We're not sending numbers. We're sending **decisions with explanations** that no other data source can generate.


---

## Language Strategy: Consolidation Plan

### Current State

| Module | Language | Justification |
|--------|----------|---------------|
| Tlapix | Rust | eBPF via Aya, memory safety, kernel-level performance — no alternative |
| Earthworm | Go + TypeScript | Go for server/eBPF (cilium/ebpf), TypeScript for React visualizer |
| eBeeControl | TypeScript | Fast prototyping, Gemini SDK — but no strong justification for backend |
| Quack | Go | sched_ext integration, kernel proximity, client-go for K8s |

### The Problem with eBeeControl in TypeScript

eBeeControl is the odd one out. It's a backend agent that:
- Monitors kernel-level file access (Tetragon/eBPF)
- Deploys honeytokens into Kubernetes pods
- Makes autonomous threat classification decisions
- Responds by isolating pods and blocking IPs

None of these require TypeScript. The choice was made for prototyping speed, not architectural fit. In the TitanOps platform context, this creates:
- A different build system (npm vs go build)
- A different test framework (Vitest vs go test)
- No code sharing with Earthworm/Quack (which do similar K8s + eBPF work in Go)
- A different deployment pattern (Node.js runtime vs static binary)

### Decision: Rewrite eBeeControl Core in Go

**Priority: Important (Phase 2-3 of platform integration)**

**What to rewrite:**
- Agent orchestrator (currently TypeScript)
- Tetragon event processing (currently TypeScript)
- Threat classifier (currently TypeScript + Gemini)
- Response planner (currently TypeScript)
- Kubernetes interactions (currently TypeScript)

**What to keep or adapt:**
- The architecture and design (proven, well-tested)
- The 24 property-based correctness properties (reimplement in Go)
- The Gemini integration (as optional cloud backend, not required)

**Why Go:**
- 3 of 4 modules will be Go (Earthworm, Quack, eBeeControl) — shared libraries possible
- cilium/ebpf for Tetragon integration (same as Earthworm)
- client-go for Kubernetes (same as Quack)
- Static binary deployment (same as all other modules)
- onnxruntime-go for local AI inference (unified AI layer)
- Consistent CI/CD (go test, go build, single Dockerfile pattern)

**What this gives TitanOps:**
- Rust for kernel-critical eBPF (Tlapix) — justified, stays
- Go for all platform services (Earthworm, eBeeControl, Quack, correlation engine, API gateway)
- TypeScript for UI only (TitanOps React Dashboard, Earthworm visualizer)

### Target Architecture After Consolidation

```
TitanOps Platform
├── Kernel Layer (Rust)
│   └── Tlapix eBPF programs (Aya) — stays Rust, no change
│
├── Platform Layer (Go)
│   ├── Earthworm agent + server
│   ├── eBeeControl agent (REWRITTEN from TypeScript)
│   ├── Quack scheduler service
│   ├── Correlation engine (NEW)
│   ├── TitanOps API gateway (NEW)
│   └── Shared libraries:
│       ├── titanops-ai (ONNX inference, pluggable cloud backends)
│       ├── titanops-k8s (common K8s client patterns)
│       ├── titanops-ebpf (common eBPF event handling)
│       └── titanops-export (Prometheus, OTLP, webhooks)
│
└── UI Layer (TypeScript/React)
    ├── TitanOps Dashboard (NEW)
    └── Earthworm Visualizer (existing)
```

### Shared Go Libraries (Enabled by Consolidation)

Once eBeeControl is in Go, these shared libraries become possible:

| Library | What It Provides | Used By |
|---------|-----------------|---------|
| `titanops-ai` | ONNX inference, model loading, cloud backend interface | All modules |
| `titanops-k8s` | K8s client, secret reading, pod operations | eBeeControl, Quack, Earthworm |
| `titanops-ebpf` | Event parsing, ring buffer reading, map operations | eBeeControl, Earthworm, Quack |
| `titanops-export` | Prometheus metrics, OTLP export, webhook dispatch | All modules |
| `titanops-config` | Unified config loading, validation | All modules |

This eliminates the duplication where each module reimplements the same patterns independently.

### Timeline

| Phase | Action |
|-------|--------|
| Now | Keep eBeeControl in TypeScript (it works, it's tested) |
| Phase 2 | Design the Go shared libraries based on common patterns across modules |
| Phase 3 | Rewrite eBeeControl core in Go, using shared libraries |
| Phase 4 | All Go modules share titanops-* libraries |


---

## Repository Strategy: Hybrid Multi-Repo

### Decision: Keep Modules Separate, Add a Platform Repo

Don't merge repos. Don't copy/paste. Each module keeps its identity, history, and independence. A new `titanops/` repo holds the platform layer (shared libraries, correlation engine, dashboard, umbrella Helm chart).

### Repository Structure

```
github.com/mercadoalex/
├── titanops/              (NEW — platform core)
│   ├── shared/            # Shared Go libraries
│   │   ├── titanops-ai/   #   ONNX inference, pluggable cloud backends
│   │   ├── titanops-k8s/  #   Common K8s client patterns
│   │   ├── titanops-ebpf/ #   Common eBPF event handling
│   │   ├── titanops-export/ # Prometheus, OTLP, webhooks
│   │   └── titanops-config/ # Unified config loading
│   ├── correlation/       # Cross-module correlation engine
│   ├── gateway/           # API gateway for the dashboard
│   ├── dashboard/         # TitanOps React UI
│   ├── helm/              # Umbrella Helm chart
│   ├── docs/              # Platform-level documentation
│   └── go.mod             # Go module: github.com/mercadoalex/titanops
│
├── tlapix/                (EXISTING — stays independent)
│   └── imports: github.com/mercadoalex/titanops/shared/titanops-export
│
├── earthworm/             (EXISTING — stays independent)
│   └── imports: github.com/mercadoalex/titanops/shared/titanops-k8s
│
├── ebeecontrol/           (EXISTING → rewrite in Go, then imports shared libs)
│   └── imports: github.com/mercadoalex/titanops/shared/titanops-ai
│
└── quack/                 (EXISTING — stays independent)
    └── imports: github.com/mercadoalex/titanops/shared/titanops-k8s
```

### Why Hybrid (Not Monorepo)

| Concern | Monorepo | Hybrid (our choice) |
|---------|----------|-------------------|
| Module discoverability | Hidden inside one repo | Each module has its own GitHub page, stars, SEO |
| Independent releases | Hard (everything versioned together) | Easy (each module has its own tags) |
| Install just one module | User gets the whole monorepo | User installs only what they need |
| Git history | Lost or complex (subtree merge) | Preserved naturally |
| CI speed | Slow (runs everything on every change) | Fast (each repo has focused CI) |
| Shared code | Easy (local imports) | Clean (Go modules with semver) |
| Cross-repo changes | Atomic (one PR) | Coordinated (multiple PRs, but rare) |

### How Modules Import Shared Libraries

Go modules make this clean:

```go
// In earthworm/server/main.go
import (
    "github.com/mercadoalex/titanops/shared/titanops-export"
    "github.com/mercadoalex/titanops/shared/titanops-k8s"
)
```

```go
// In quack/main.go
import (
    "github.com/mercadoalex/titanops/shared/titanops-ai"
    "github.com/mercadoalex/titanops/shared/titanops-config"
)
```

For Tlapix (Rust), shared integration happens via:
- The Prometheus metrics format (already implemented)
- OTLP export (already implemented)
- The umbrella Helm chart (references Tlapix's chart as a dependency)

### How the Umbrella Helm Chart Works

The TitanOps Helm chart doesn't contain module code. It references each module's chart as a dependency:

```yaml
# titanops/helm/titanops/Chart.yaml
apiVersion: v2
name: titanops
description: Autonomous AiOps modules for Kubernetes
version: 0.1.0

dependencies:
  - name: tlapix
    version: ">=0.1.0"
    repository: "https://mercadoalex.github.io/tlapix/charts"
    condition: tlapix.enabled

  - name: earthworm
    version: ">=0.1.0"
    repository: "https://mercadoalex.github.io/earthworm/charts"
    condition: earthworm.enabled

  - name: ebeecontrol
    version: ">=0.1.0"
    repository: "https://mercadoalex.github.io/ebeecontrol/charts"
    condition: ebeecontrol.enabled

  - name: quack
    version: ">=0.1.0"
    repository: "https://mercadoalex.github.io/quack/charts"
    condition: quack.enabled
```

Each module publishes its own Helm chart to its own GitHub Pages. The umbrella chart pulls them together. No code duplication.

### Installation Paths

| User Wants | How They Install |
|-----------|-----------------|
| Just Tlapix | `helm install tlapix mercadoalex/tlapix` |
| Just Quack | `helm install quack mercadoalex/quack` |
| Full TitanOps platform | `helm install titanops mercadoalex/titanops --set tlapix.enabled=true --set quack.enabled=true` |
| Platform + correlation | `helm install titanops mercadoalex/titanops --set correlation.enabled=true` |

### Versioning Strategy

| Component | Versioning | Release Cadence |
|-----------|-----------|-----------------|
| Each module (tlapix, earthworm, etc.) | Independent semver | When ready |
| Shared libraries (titanops/shared/) | Semver via Go modules | When API changes |
| Umbrella Helm chart | Own semver | When compatibility matrix updates |
| TitanOps Dashboard | Own semver | When UI changes |

### Rules to Avoid Chaos

1. **Shared libraries NEVER import modules** — dependency flows one way only (modules → shared)
2. **Breaking changes in shared libs require a major version bump** — modules pin to compatible versions
3. **Each module's CI tests against the latest shared libs** — catch incompatibilities early
4. **The umbrella chart declares a compatibility matrix** — "tlapix >=0.1.0, earthworm >=0.2.0, etc."
5. **Platform-level features (correlation, dashboard) live in titanops/ repo only** — not scattered across modules


---

## Versioning: Semantic Versioning (Semver)

All TitanOps components follow [Semantic Versioning](https://semver.org/).

### Format: `MAJOR.MINOR.PATCH` → `v1.2.3`

| Part | When to Bump | Meaning | Example |
|------|-------------|---------|---------|
| **PATCH** (1.2.**3**) | Bug fix, no API change | Safe to upgrade blindly | Fixed a log message, patched a race condition |
| **MINOR** (1.**2**.3) | New feature, backwards compatible | Safe to upgrade, new stuff available | Added a new endpoint, new config option |
| **MAJOR** (**1**.2.3) | Breaking change | Requires code changes by consumers | Renamed a function, removed a field, changed behavior |

### The Contract

> "You can upgrade safely within the same major version."

- `v0.2.1` → `v0.2.5`: always safe (patches only)
- `v0.2.1` → `v0.3.0`: safe but check release notes (new features, possible deprecations)
- `v0.2.1` → `v1.0.0`: breaking — read migration guide before upgrading

### How It Works in Practice (Go Modules)

Shared libraries in `titanops/` are versioned via git tags:

```bash
# After making a bug fix to titanops/shared/titanops-ai:
git tag shared/v0.2.2
git push --tags

# Modules update their dependency:
cd ../earthworm
go get github.com/mercadoalex/titanops/shared@v0.2.2
```

No package registry needed. No npm publish. Just git tags. Go's module proxy (`proxy.golang.org`) caches it automatically and makes it available to anyone running `go get`.

### Rules to Avoid Technical Debt

1. **Never release v1.0.0 until the API is stable** — stay on v0.x.x during development (breaking changes are expected in v0.x)
2. **Every breaking change bumps MAJOR** — no exceptions, no "small breaking changes"
3. **Deprecate before removing** — mark as deprecated in v0.3.0, remove in v0.4.0 (give consumers time)
4. **Tag every release** — no "just use main branch" — pinned versions prevent surprise breakage
5. **CHANGELOG.md in every repo** — document what changed and why, every release
6. **CI tests against pinned versions** — modules test against the version they declare, not latest

### Version Status of TitanOps Components

| Component | Current Version | Stability |
|-----------|----------------|-----------|
| Tlapix | v0.1.0 | API unstable (pre-v1, expect changes) |
| Earthworm | v0.1.0 | API unstable |
| eBeeControl | v0.1.0 | API unstable (will be rewritten in Go) |
| Quack | v0.1.0 | API unstable |
| TitanOps shared libs | Not yet released | Design phase |
| TitanOps Dashboard | Not yet started | — |
| Umbrella Helm chart | v0.1.0 | Tracks module compatibility |

All components start at `v0.1.0`. The `v0.x` prefix signals: "this is under active development, APIs may change." When the platform stabilizes, individual components graduate to `v1.0.0` independently.

---

## Infrastructure Strategy: One Shared EKS Cluster

### Decision: Single Cluster for All Modules

All four TitanOps modules run on one shared EKS cluster. Separate clusters per module are rejected because:

- The Correlation Engine needs sub-second access to events from all modules — cross-cluster communication adds latency and complexity
- eBPF probes from Earthworm, Tlapix, eBeeControl, and Quack all need to run on the same kernel (same node) to observe the same workloads
- One DaemonSet loading all eBPF programs is cheaper than four separate DaemonSets across four clusters
- The umbrella Helm chart is designed for exactly this — one `helm install` into one cluster
- EKS control plane cost: $0.10/hour × 1 cluster vs × 4 clusters = 75% savings on control plane alone

### Cost Comparison

| Approach | Control Plane | Nodes | Networking | Complexity |
|----------|--------------|-------|------------|------------|
| 4 separate clusters | $292/mo | 4+ nodes minimum (1 per cluster) | Cross-cluster VPC peering for correlation | High |
| 1 shared cluster | $73/mo | 1-2 nodes (all modules share) | In-cluster gRPC, zero extra networking | Low |

### Terraform Location

Infrastructure lives in the `titanops` platform repo — not in individual module repos:

```
github.com/mercadoalex/titanops/
├── infra/
│   ├── terraform/
│   │   ├── environments/
│   │   │   ├── dev/
│   │   │   │   ├── main.tf
│   │   │   │   ├── variables.tf
│   │   │   │   ├── outputs.tf
│   │   │   │   └── terraform.tfvars
│   │   │   ├── staging/
│   │   │   └── prod/
│   │   ├── modules/
│   │   │   ├── eks/              # EKS cluster + node groups
│   │   │   ├── vpc/              # VPC, subnets, security groups
│   │   │   ├── iam/              # Shared IAM roles for all modules
│   │   │   ├── ecr/              # Container registries (one per module)
│   │   │   ├── s3/               # Model storage (ONNX files)
│   │   │   └── monitoring/       # CloudWatch, Prometheus remote write
│   │   └── backend.tf            # S3 + DynamoDB state backend
│   └── scripts/
│       ├── bootstrap.sh          # First-time setup
│       └── destroy.sh            # Teardown
```

**Why in `titanops/` and not in each module repo:**
- Infrastructure is platform-level — it serves all modules equally
- One state file, one plan, one apply — no coordination across repos
- Module repos stay focused on application code
- Matches the principle: "platform-level concerns live in titanops/"

### Cost Optimization Strategies

| Strategy | Savings | How |
|----------|---------|-----|
| Spot instances for non-prod | 60-70% on compute | Node group with spot + on-demand fallback |
| Karpenter autoscaling | Scale to zero when idle | Nodes spin down when no workload |
| ARM instances (Graviton) | 20% cheaper than x86 | All Go binaries cross-compile trivially |
| Fargate for Dashboard | No idle node cost | Dashboard pod runs on-demand only |
| Single NAT Gateway | $32/mo saved per extra | One AZ for dev, multi-AZ for prod |

### Environment Sizing

| Environment | Nodes | Instance Type | Estimated Cost |
|-------------|-------|---------------|----------------|
| Dev | 1 | t3.medium (spot) | ~$100-150/mo |
| Staging | 2 | t3.large (spot + on-demand) | ~$200-300/mo |
| Prod | 3+ | m6g.large (Graviton, multi-AZ) | ~$400-600/mo |

### Key Principle

> "One cluster, one Helm install, one state file. Modules share compute, not just code."

Individual module repos contain zero infrastructure code. All Terraform, all cluster config, all shared IAM — it lives in `titanops/infra/`. When you `terraform apply`, you get one EKS cluster ready for `helm install titanops`.


---

## Dashboard Strategy: Two Layers, Two Audiences

### Decision: Grafana JSON + Custom TitanOps Dashboard

TitanOps needs two dashboard layers. They are not competing — they serve different audiences and show fundamentally different things.

### Layer 1: Grafana Dashboard JSON (Distribution Channel)

**What it is:** A static JSON file users import into their existing Grafana instance.

**What it shows:** Time-series metrics — heartbeat gaps, CPU/memory/latency trends, event throughput, anomaly rates, prediction accuracy (TP/FP). This is commodity observability data that ops teams expect to see alongside their other Kubernetes metrics.

**Why it exists:**
- ~90% of Kubernetes teams already run Grafana
- Zero runtime dependency — it's a file in the repo, not a Grafana plugin
- Adoption signal: "It just works with my stack"
- Table stakes for Phase 1 credibility
- If users can't see Earthworm data in Grafana, they assume we don't integrate

**What it is NOT:**
- Not a product differentiator
- Not a vendor lock-in (users can ignore it entirely)
- Not a replacement for the TitanOps dashboard

**Distribution options (in priority order):**

| Option | Effort | Value | When |
|--------|--------|-------|------|
| JSON file in repo (`grafana/`) | 1-2 days | Users can import immediately | Phase 1 |
| Published to Grafana.com library | 1 hour (just upload) | Free discoverability, SEO | Phase 1 |
| Grafana plugin | 2-3 weeks | Deep integration, custom panels | Only if demand proves it |

**Decision:** Ship JSON files in each module repo. Do NOT build a Grafana plugin unless user demand is overwhelming. The plugin path is maintenance-heavy and pulls engineering effort away from the real product.

### Layer 2: TitanOps Custom Dashboard (The Product)

**What it is:** A React-based command center that shows what no integration can.

**What it shows:**
- Autonomous decision chains (observation → analysis → action → outcome)
- AI confidence scores with reasoning explanations
- Cross-module correlation timelines (Earthworm + Tlapix + eBeeControl linked)
- Human override controls ("undo this action", "approve this recommendation")
- "Why did TitanOps do X?" explainability
- Eval runner results and model accuracy over time
- Module health at a glance

**Why this is the moat:**

Grafana shows time-series charts. Datadog shows metrics. The TitanOps dashboard shows **decisions and actions**. No existing tool can render:
- A decision chain: "Earthworm detected latency spike → AI scored 0.88 → cordoned node → outcome: true positive"
- Cross-module correlation: "Heartbeat degraded + shadow cert appeared + honeytoken accessed = crypto-miner (92% confidence)"
- Approval workflows: "TitanOps recommends pod restart. Approve / Reject / Modify"

**This is what you charge for in Pro/Enterprise tiers.**

### How They Relate

```
┌─────────────────────────────────────────────────────────────────┐
│                        User's Stack                               │
│                                                                   │
│  ┌──────────────────────┐     ┌──────────────────────────────┐  │
│  │   Grafana (existing)  │     │  TitanOps Dashboard (ours)    │  │
│  │                        │     │                                │  │
│  │  • Metrics over time   │     │  • Decisions & reasoning       │  │
│  │  • Alert panels        │     │  • Cross-module correlation    │  │
│  │  • Standard K8s data   │     │  • Human override controls     │  │
│  │                        │     │  • Eval/accuracy tracking      │  │
│  │  (import our JSON)     │     │  • Audit trail                 │  │
│  └──────────┬─────────────┘     └──────────────┬─────────────────┘  │
│             │                                   │                    │
│             │ Prometheus scrape                  │ REST/WS API        │
│             ▼                                   ▼                    │
│  ┌────────────────────────────────────────────────────────────────┐ │
│  │                   TitanOps Modules                              │ │
│  │   /metrics (Prometheus)    +    /api/* (decisions, actions)     │ │
│  └────────────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────────┘
```

### Strategic Principle

> "Grafana shows what happened. TitanOps shows what it did about it — and why."

The Grafana JSON is a marketing asset disguised as a config file. The custom dashboard is the product. Do both, in this order:

1. **Grafana JSON** (Phase 1) — low effort, high adoption signal, ships as a file
2. **TitanOps Dashboard** (Phase 2-3) — the differentiator, the revenue driver

### Updated Phase 1 Dashboard Task

| Task | What | Effort | Status |
|------|------|--------|--------|
| 1.3a | Grafana dashboard JSON per module (import-ready) | 1-2 days per module | Not started |
| 1.3b | Publish to Grafana.com dashboard library | 1 hour per module | Blocked on 1.3a |
| 1.3c | TitanOps Dashboard design (wireframes, API contract) | 1 week | Not started |
| 1.3d | TitanOps Dashboard MVP (decisions + actions view) | 2-3 weeks | Not started |

### What NOT to Build

- ❌ Grafana plugin (maintenance burden, pulls focus from product)
- ❌ Datadog/Splunk custom dashboards (they consume our OTLP/HEC exports natively)
- ❌ A general-purpose metrics viewer (use Grafana for that)
- ❌ Dashboard as the primary install requirement (it's optional, modules work headless)
