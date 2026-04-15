# Deploy

Kubernetes manifests for acurve. Managed by Flux + bjw-s/app-template.

## Structure

```
deploy/
├── base/                       # Applied by Flux to the acurve namespace
│   ├── namespace.yaml
│   ├── postgres.yaml           # CNPG Cluster (single instance, 5 GiB)
│   ├── secrets.sops.yaml       # SOPS-encrypted secrets (DO NOT commit plaintext)
│   ├── helmrelease.yaml        # app-template HelmRelease (all four controllers)
│   └── kustomization.yaml
└── flux/                       # Snippets to add to cluster-config
    ├── helmrepository-bjw-s.yaml   # HelmRepository for bjw-s/helm-charts
    └── kustomization-acurve.yaml   # Flux Kustomization pointing at deploy/base
```

## First-time setup

### 1. Generate an age key and register it with Flux

```bash
# Install age if needed: https://github.com/FiloSottile/age#installation
age-keygen -o age.key
```

This prints the public key to stdout, e.g. `age1abc123...`.

```bash
# Store the PRIVATE key in the cluster so Flux can decrypt secrets at apply time
kubectl create secret generic sops-age \
  --namespace=flux-system \
  --from-file=age.agekey=age.key

# Keep age.key somewhere safe (password manager).
# NEVER commit it to the repo.
rm age.key  # or move to a safe location
```

### 2. Configure the repo to use that public key

Edit [`.sops.yaml`](../.sops.yaml) at the repo root — replace `REPLACE_WITH_AGE_PUBLIC_KEY`
with the public key printed by `age-keygen` above:

```yaml
creation_rules:
  - path_regex: deploy/base/secrets\.sops\.yaml$
    age: age1abc123...   # <-- your actual public key here
```

### 3. Fill in and encrypt secrets

Edit `deploy/base/secrets.sops.yaml`, replace all `CHANGE_ME` values with
real credentials, then encrypt in place:

```bash
sops --encrypt --in-place deploy/base/secrets.sops.yaml
```

Commit the encrypted file. Never commit the plaintext version.

### 4. Wire up Flux

Apply the bjw-s HelmRepository to the cluster:
```bash
kubectl apply -f deploy/flux/helmrepository-bjw-s.yaml
```

Add `deploy/flux/kustomization-acurve.yaml` to your cluster-config repo
(adjust `sourceRef.name` to whichever GitRepository points at this repo,
and confirm `sops-age` matches the secret name from step 1).

### 5. CNPG

CNPG operator must already be installed in the cluster. If not:
```bash
kubectl apply --server-side \
  -f https://raw.githubusercontent.com/cloudnative-pg/cloudnative-pg/main/releases/cnpg-latest.yaml
```

### 6. Push & watch

```bash
git push
flux reconcile kustomization acurve --with-source
flux get helmreleases -n acurve
kubectl get pods -n acurve
```

## Image tags

CI pushes `main-<sha>` tags on every merge to main.
Update the image tags in `helmrelease.yaml` after first push, or set up
[Flux image automation](https://fluxcd.io/flux/guides/image-update/) to do it automatically.

## Editing secrets after initial encryption

```bash
sops deploy/base/secrets.sops.yaml
# $EDITOR opens with decrypted content; save to re-encrypt
```
