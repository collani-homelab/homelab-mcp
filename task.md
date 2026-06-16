# homelab-mcp Task Tracker

## Current Sprint

- [x] CI/CD: Verify `.github/workflows/deploy.yml` builds and pushes correctly end-to-end.
- [x] Platform: Validate `platform.json` has all required environment variables defined.
- [x] Cross-repo: Confirm `homelab-platform` sre-machine.json reflects the shared runner topology.

## Known Issues (found 2026-06-16 deploying the Phoenix provider)

- [ ] **Deploy workflow no longer runs** — reopens the CI/CD item above. Repo is now public (`collani-homelab/homelab-mcp`) and the org's only self-hosted runner group has `allows_public_repositories: false`, so jobs queue forever. Also `REGISTRY_URL`/`PLATFORM_REPO_PATH` repo variables are unset (likely lost in the `wcollani/homelab-mcp` → `collani-homelab/homelab-mcp` transfer). See ROADMAP.md "Known Issues" for details and the manual-deploy workaround.
- [ ] **`platform.json`'s `environment` list is stale and appears unused** — it lists vars that don't match actual `.env` names (e.g. `UNIFI_USER`/`UNIFI_PASS` vs. the real `UNIFI_API_USER`/`UNIFI_API_KEY`), is missing most providers added since Phase 4 (Grafana, Dagu, Phoenix, monitoring, context), and grepping both this repo and `homelab-platform` turns up no code that actually reads `platform.json`. Reopens the "Platform" item above — either wire it into the deploy/provisioning path or remove it.
