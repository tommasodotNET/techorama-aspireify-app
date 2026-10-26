# Part 1: Build the app without Aspire

Copy the prompt below into your coding agent, replacing the bracketed app brief.
Run this part first and demonstrate the working Docker Compose app before using
[Part 2](02-aspireify-app.md).

The repository starts without an AppHost or Aspire skills. Aspire is introduced
only in Part 2.

## Prompt

```text
Build a small, working application for a live coding demo. Put the code in the src folder.

App brief:
The app is a store for [blank] to sell [something].
There is a frontend written in [blank] with style [blank].
There ia a backend written in [blank] that talks a storage [blank].
You have a password protected admin portal where password is [blank].

Inspect the repository and follow its instructions before making changes. If the
brief leaves implementation details open, choose simple, mainstream defaults.
Keep the scope small enough to build and explain on stage: a frontend, an API,
and a database, with one complete user workflow and a little useful sample data.
Avoid authentication, external paid services, and additional infrastructure
unless the brief requires them.

Requirements:
- Put all demo application code, dependency manifests, Dockerfiles, and a
  compose.yaml under src/. Keep it independent of the repository root tooling.
- Do not add or use Aspire for this app: no AppHost, Aspire packages,
  ServiceDefaults, Aspire skills, Aspire-generated Compose, or Aspire startup
  commands.
- Make `docker compose -f src/compose.yaml up --build` from the repository root
  start the whole app, including its database, without manual setup steps.
- Use ordinary configuration/environment variables for service addresses and
  database connections. Use Compose service names between containers, not
  localhost. Make browser-to-API requests work from the documented frontend URL.
- Include health checks and startup readiness handling, automatic database
  initialization, repeatable sample-data seeding, and a named database volume.
- Keep configuration ready to move to a different orchestrator later, but do not
  implement that migration now. Do not commit real secrets; clearly label any
  local-only demo credentials and document required configuration.
- Build a simple, presentable UI with loading, empty, and error states. Persist
  real data through the API rather than simulating success in the frontend.
- Add focused tests for the main workflow and a src/README.md with prerequisites,
  configuration, startup, URLs, verification steps, and non-destructive shutdown.
  Clearly distinguish an optional destructive data reset from normal shutdown.

Implement the app, run the relevant checks, and start it with Docker Compose.
Verify the main workflow through the UI/API and confirm that data survives a
container restart. Do not claim checks passed if you could not run them; report
the exact blocker instead.

Finish with a concise explanation of the app, the frontend URL, and the commands
needed to demonstrate and stop it. Leave the verified app running for the demo.
Stop here: do not add Aspire, commit, or push changes.
```
