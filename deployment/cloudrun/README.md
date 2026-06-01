# Cloud Run Configuration

`env/` stores non-secret runtime configuration per environment.

`secrets/` maps application environment variables to Secret Manager secret versions. The `__ENV__` placeholder is replaced by the deployment scripts with `staging` or `production`.

`local.env.example` is the source template for local development. Copy the values you need into `configs/.env.config` because the current app already loads that file in local and development modes.
