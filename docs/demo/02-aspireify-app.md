# Part 2: Aspireify the existing app

Use this prompt after completing and demonstrating
[Part 1](01-build-app.md). It discovers the app built under `src/`; no particular
language, framework, or application domain is assumed.

The repository starts without an AppHost or bundled Aspire skills. This step
introduces Aspire to the working Docker Compose app.

## Prompt

```text
Aspireify the existing application under src/ for the second half of a live demo.
The app already works with Docker Compose. Preserve its features and user
workflow; this step changes orchestration, configuration, and observability,
not the product.

Read the repository instructions and use any available Aspire skills; do not
assume they are installed. Inspect src/, its README, Dockerfiles, and compose.yaml
to discover the services, dependencies, ports, configuration, health checks, and
persistent data.
Inspect any existing AppHost before choosing how to integrate the app.

Briefly explain the proposed resource graph and the Compose-to-Aspire mapping,
then implement it:
- Use `aspire init` to add an AppHost to the existing app before wiring its
  resources. If an AppHost already exists from a previous attempt, inspect and
  reuse it instead of creating a duplicate.
- Keep application code under src/. Choose a supported AppHost language that
  fits the app and repository conventions.
- Use supported Aspire integrations for the app's actual frameworks and
  infrastructure. Discover available integrations and consult current Aspire
  docs/API references rather than guessing package names or APIs.
- Model services and dependencies explicitly. Use resource references, service
  discovery, injected configuration, and readiness dependencies instead of
  copying Compose's fixed host ports or hardcoding container addresses.
- Ensure browser-facing API routing works too: internal resource names are not
  browser-resolvable URLs. Preserve the frontend's existing proxy/routing model
  where practical and discover actual endpoints from Aspire.
- Preserve database initialization and repeatable seeding. Configure persistent
  storage for the Aspire database and explain whether it uses a separate local
  dataset from Compose. Do not delete, overwrite, or silently migrate existing
  Compose data.
- Add appropriate health checks and OpenTelemetry instrumentation. For .NET
  services, use ServiceDefaults where appropriate; for other runtimes, use their
  supported instrumentation. Make app logs and a representative request trace
  visible in the Aspire dashboard without replacing application behavior.
- Keep secrets out of source control and use Aspire's supported local secret
  configuration. Do not edit generated Aspire modules manually.
- Retain the original Docker Compose workflow as a working baseline. Share
  application code between both launch paths, using configuration rather than
  duplicated implementations. Do not add cloud deployment or unrelated features.

Before starting Aspire, identify the demo's Compose stack and stop only its
containers non-destructively if needed to avoid conflicts or competing writers.
Do not remove its volumes or stop unrelated containers.

Validate through the Aspire lifecycle tools/CLI and any available skills: start
the exact AppHost, wait for resources to become ready, discover the real URLs,
and exercise the same user workflow as in the Compose version. Check resource
health, logs, and traces in the dashboard, and run the relevant application
checks. Follow any available skills' validation/cleanup lifecycle; otherwise,
stop the AppHost non-destructively after validation and give the exact command
to start it for the presentation. Report blockers honestly rather than claiming
unverified success.

Update src/README.md with both startup paths, prerequisites, configuration,
data-storage differences, non-destructive shutdown, and a short demo sequence:
show the resource graph, open the app, perform the workflow, then inspect its
logs and request trace. Explain which manual Compose wiring Aspire now handles.

Finish with a concise summary of the changes, the exact Aspire launch command,
and the dashboard/app URLs discovered during validation, including whether they
are still running. Do not commit or push changes.
```
