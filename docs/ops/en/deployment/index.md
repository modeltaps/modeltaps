---
title: "Deployment"
layout: doc
outline: deep
lastUpdated: true
---

# Deployment

::: warning Use a dedicated database
Modeltaps has its own database schema. Give it a dedicated database and do not share tables with other services.
:::

## Configuration

Modeltaps supports two configuration methods:

1. Environment variables
2. A configuration file (`config.yaml`)

::: tip Precedence
Environment variables > configuration file
:::

### Required configuration

- `USER_TOKEN_SECRET`: required. The secret used to sign user tokens; it must be longer than 32 characters. Do not change it once set, or previously issued user tokens become invalid.
- `SESSION_SECRET`: recommended. Keeps users signed in; without it, everyone has to sign in again after each restart.

Generate a distinct strong random value for each:

```bash
openssl rand -base64 48 | tr -d '\n'; echo
```

::: danger Never use the example values from this documentation
Every secret, password and database connection string on this page is a **placeholder** (written as `<...>`) that illustrates the format only and **must not be used in production**. Put real values in a `.env` file on the host (and run `chmod 600` on it) or in a secret manager. Do not write them into command-line arguments or `docker-compose.yml`, and never commit them to version control.
:::

## Signing in to the private registry

Modeltaps images are published to the private registry `ghcr.io/modeltaps/modeltaps`. You **must sign in before pulling**, otherwise the `docker pull` / `docker run` / `docker-compose up` commands below fail with an authorization error.

::: tip Access control
This registry has access control enabled and only authorized accounts can pull. If the pull still fails after signing in, ask your delivery contact to grant that account read-only access to the image.
:::

1. **Prepare an access token**

   Use an access token with the `read:packages` scope (read-only package pulls) and no additional permissions.

   ::: warning GHCR only accepts classic PATs
   GitHub Container Registry only accepts **classic** personal access tokens. Fine-grained tokens cannot carry Packages permissions, so signing in with one keeps failing with `unauthorized`.
   :::

2. **Sign in (via stdin)**

   ```shell
   echo "$CR_PAT" | docker login ghcr.io -u <USERNAME> --password-stdin
   ```

   ::: warning Do not leak the token
   Pass the token to `docker login` through an environment variable or standard input. **Do not** put it in command-line arguments, scripts or plaintext files, or it ends up in shell history and the process list. `$CR_PAT` in the example should come from a securely configured local environment variable, and `<USERNAME>` should be replaced with the real account name.
   :::

3. **Verify the pull**

   ```shell
   docker pull ghcr.io/modeltaps/modeltaps:latest
   ```

   A successful pull confirms that the sign-in worked.

   ::: warning Credentials belong to the OS user that signed in
   `docker login` stores its credentials in the `~/.docker/config.json` of the **operating-system user that ran it**, and users do not share them. Sign in as the **same user that actually runs `docker compose`**: if you drive Compose with `sudo`, run `sudo docker login ghcr.io` as well (the credentials land in root's `/root/.docker/config.json`), otherwise the pull still fails with `unauthorized`.
   :::

## Docker deployment

### Prerequisites

1. Create the data directory:

```bash
# create the main data directory
sudo mkdir -p /data/modeltaps
cd /data/modeltaps
```

::: tip Tighten the data directory permissions
This directory will hold the SQLite database, secret files and logs. The application creates `modeltaps.db` with mode `0644`, so if the directory itself is open to other users, anyone on the host can read the database. After writing `.env` / `config.yaml` and before starting the container, hand the directory to the in-container user (UID 10001) and restrict it to its owner:

```bash
sudo chown -R 10001:10001 /data/modeltaps
sudo chmod 700 /data/modeltaps
```

See [Tightening directory permissions](#tightening-directory-permissions) for the full steps.
:::

2. Make sure Docker is installed and running:

```bash
# check Docker status
sudo systemctl status docker
# start Docker if it is not running
sudo systemctl start docker
```

::: warning Note

- The first `3000` in `-p 3000:3000` is the host port and can be changed as needed.
- Data and logs are stored in `/data/modeltaps` on the host. Make sure the directory exists and is writable, or change it to a suitable path. It contains the SQLite database and log files, which are sensitive, so set its owner and permissions as described in [Tightening directory permissions](#tightening-directory-permissions).
- If the container fails to start, add `--privileged=true`.
- Under high concurrency you **must** set `SQL_DSN`.
  :::

### Deploying with environment variables

See [Environment Variables](./env.md) for the full reference.

The recommended approach is to write secrets into a `.env` file on the host and inject it with `--env-file`, so secrets never reach shell history or the process list:

```bash
# /data/modeltaps/.env — the angle-bracket values below are placeholders; replace them with values you generate yourself
TZ=UTC
USER_TOKEN_SECRET=<random value from openssl rand -base64 48>
SESSION_SECRET=<a separately generated random value>
```

Then restrict the file so only its owner can read and write it:

```bash
chmod 600 /data/modeltaps/.env
```

#### Using SQLite

```shell
docker run -d -p 3000:3000 \
  --name modeltaps \
  --restart always \
  --env-file /data/modeltaps/.env \
  -v /data/modeltaps:/data \
  ghcr.io/modeltaps/modeltaps:latest
```

#### Using MySQL

Append `SQL_DSN` to `.env`; the container start command is identical to the SQLite one:

```bash
SQL_DSN=<DB_USER>:<DB_PASSWORD>@tcp(<DB_HOST>:3306)/modeltaps
```

#### Using PostgreSQL

```bash
SQL_DSN=postgres://<DB_USER>:<DB_PASSWORD>@<DB_HOST>:5432/modeltaps
```

::: warning Database account
Create a dedicated database account for Modeltaps with a strong password; do not reuse superuser accounts such as `root`.
:::

Once deployed, open `http://localhost:3000`. After the first sign-in, [set the server address](#setting-the-server-address) first.

### Deploying with a configuration file

1. Prepare the configuration template. `config.example.yaml` ships in the root of the delivery package; copy it into the data directory and rename it to `config.yaml`:

```bash
cd /data/modeltaps
cp <delivery package directory>/config.example.yaml ./config.yaml
```

2. Adjust the file as needed. Common keys include:

```yaml
# required settings (angle-bracket values are placeholders; replace them with strong random values you generate, and never use them in production)
user_token_secret: "<random value from openssl rand -base64 48>" # user token secret
session_secret: "<a separately generated random value>" # session secret

# database configuration
sql_dsn: "<DB_USER>:<DB_PASSWORD>@tcp(<DB_HOST>:3306)/modeltaps" # MySQL example
```

::: warning The configuration file stores secrets in plaintext
`config.yaml` keeps secrets in plaintext. Run `chmod 600 config.yaml` to restrict access and make sure it is never committed to version control. If you would rather keep secrets off disk, use the `.env` and environment variable approach from the previous section.
:::

3. Run the container

```shell
docker run -d -p 3000:3000 \
  --name modeltaps \
  --restart always \
  -e TZ=UTC \
  -v /data/modeltaps:/data \
  ghcr.io/modeltaps/modeltaps:latest
```

## Docker Compose deployment

::: warning Sign in to the private registry first
Private images require [signing in to the private registry](#signing-in-to-the-private-registry) first, otherwise `docker-compose up` fails to pull the image.
:::

### Prerequisites

1. Create the required directory structure:

```bash
# create the main directory
sudo mkdir -p /data/modeltaps
cd /data/modeltaps
# create the subdirectory
mkdir data
```

::: tip Tighten the data directory permissions
The `data` subdirectory is mounted as `/data` in the container and will hold the SQLite database, configuration file and logs. The application creates `modeltaps.db` with mode `0644`, so if the directory is open to other users, anyone on the host can read the database. **Before the first start**, hand it to the in-container user (UID 10001) and restrict it to its owner:

```bash
sudo chown -R 10001:10001 data
sudo chmod 700 data
```

See [Tightening directory permissions](#tightening-directory-permissions) for the full steps.
:::

2. Prepare the compose file and the environment variable template. `docker-compose.yml` and `.env.example` live in the root of the delivery package; copy them into the current directory and save the template as `.env`:

```bash
cp <delivery package directory>/docker-compose.yml ./
cp <delivery package directory>/.env.example ./.env
```

3. Edit `.env` to fill in the secrets. **Do not** edit the secrets directly in `docker-compose.yml`.

All secrets in `docker-compose.yml` are written as `${VAR:?...}` and are read automatically from the `.env` file in the same directory; if any of them is missing, `docker-compose up` exits with an error. At minimum you must provide `SESSION_SECRET`, `USER_TOKEN_SECRET`, `MYSQL_PASSWORD` and `MYSQL_ROOT_PASSWORD`. Generate the values with:

```bash
# session and token secrets
openssl rand -base64 48 | tr -d '\n'; echo
# database passwords
openssl rand -base64 24 | tr -d '\n'; echo
```

Then restrict the file permissions:

```bash
chmod 600 .env
```

::: warning Never commit .env
`.env` contains plaintext secrets. Make sure it is ignored by `.gitignore` and never distribute it with the deployment package.
:::

To use a configuration file instead, run the command below and remove the `SQL_DSN` / `REDIS_CONN_STRING` / `SESSION_SECRET` / `USER_TOKEN_SECRET` parameters from `docker-compose.yml`:

```shell
# copy the application configuration template
cp <delivery package directory>/config.example.yaml ./data/config.yaml
```

### Starting the services

```shell
docker-compose up -d
```

Check the deployment status with:

```shell
docker-compose ps
```

Make sure every service started successfully and shows the 'Up' state.

Once deployed, open `http://localhost:3000`. After the first sign-in, [set the server address](#setting-the-server-address) first.

### Optional: run the documentation site locally

`docker-compose.yml` also contains a `docs` service that is **not started** by default; it lets you self-host this documentation site on an intranet or offline. It is guarded by a profile, so a plain `docker-compose up -d` is unaffected:

```shell
docker compose --profile docs up -d
```

Once started, open `http://localhost:3001` (`/llms.txt` and `/llms-full.txt` are served as well).

::: warning Requires the full delivery package
This service is built locally (the build context is the `docs/` directory of the delivery package) rather than pulled as an image, so it only works on machines that have the complete delivery package; it cannot be started from `docker-compose.yml` alone. The first build needs internet access to install the documentation site's dependencies.
:::

To stop only the documentation site while keeping the gateway running:

```shell
docker compose --profile docs stop docs
```

## Manual deployment

Besides the container image, the delivery package includes a `modeltaps` executable for your platform that can run directly on the host.

1. **Place the executable**: copy the `modeltaps` binary from the delivery package into the installation directory (for example `/opt/modeltaps`).

2. **Run the application**: make it executable and start it:

   ```shell
   chmod u+x modeltaps
   ./modeltaps --port 3000 --log-dir ./logs
   ```

3. **Open the application**: browse to `http://localhost:3000` and sign in. The initial account is `root`; its initial password depends on the configuration at first startup:

   - `ROOT_PASSWORD` is set (8–64 characters): that value becomes the initial root password. The program fails to start if the length is out of range.
   - `ROOT_PASSWORD` is not set: the program generates a 16-character strong random password with `crypto/rand` and prints it **exactly once, in the first-run log**, in the form:

     ```text
     no user exists, create a root user for you: username is root, initial root password: <random password>
     ```

   Local development convention: set `ROOT_PASSWORD=root` together with `ROOT_PASSWORD_ALLOW_INSECURE=true` so you can sign in with `root/root` after the first startup. That switch is for local development only and must never be enabled in production.

   ::: warning Initial password
   This only applies when the database has no users yet. Change the root password immediately after your first sign-in and clean up the startup log containing the initial password (see [Tightening directory permissions](#tightening-directory-permissions)).
   :::

Runtime behaviour and configuration keys are the same as for the container deployment: you can inject settings through `.env` / environment variables, or point at a configuration file with `--config ./config.yaml` (see [CLI Options](./cli.md)). In production, tighten directory and file permissions as described in [Bare-metal (manual) deployment](#bare-metal-manual-deployment).

## Setting the server address

The server address defaults to `http://localhost:3000`. Whenever the site is reached through another domain, IP address or port (including through a reverse proxy), change it right after the first sign-in:

1. Sign in as root or an administrator, open **System Settings → Site** and find the **Server Settings** card.
2. In **Server Address**, enter the full address users use to reach the site, including the scheme and any non-default port, for example `https://yourdomain.com`.
3. Click **Save**.

The backend builds every external link from this address. While it keeps the default, the following all point to `localhost` and do not work for users:

- the Base URL and sample commands shown on the **API Key** page;
- links in emails such as password resets;
- OAuth / OIDC sign-in callbacks and payment callbacks;
- the site domain that passkeys (WebAuthn) are bound to.

While the default is still in place, the card shows a warning and suggests the address the browser is currently using.

::: warning Set the address before registering passkeys
Passkeys are bound to the domain of the server address at registration time, and changing the domain later invalidates them. Set the server address and restart the service once, then register root's passkeys as described in [Administrator break-glass access](#administrator-break-glass-access).
:::

## Tightening directory permissions

Both the log directory and the data directory hold sensitive content on disk, and the default permissions are **not** enough to prevent other users on the same machine from reading them. Tighten them manually after deployment.

### Why this matters

**1. The log directory records the initial root password**

On first startup (when the database has no users yet), if `ROOT_PASSWORD` is not set, the program generates a 16-character random password and **prints it once in plaintext**:

```text
no user exists, create a root user for you: username is root, initial root password: <random password>
```

This line is written to two places:

- **The log file**: at `LOG_DIR` / `LOGS_FILENAME` (defaults `./logs` / `modeltaps.log`; see [Environment Variables](./env.md) and [CLI Options](./cli.md)). The container image uses `/data` as its working directory, so by default it lands in `/data/logs/modeltaps.log`, that is `logs/modeltaps.log` under the host mount point.
- **The process's standard error**: the output visible through `docker logs modeltaps` or the systemd journal.

Two details about the current permissions are worth noting:

- If the log directory does not exist, the program creates it, requesting mode `0777` (the effective value depends on umask, usually `0755`), so **other users on the same machine can enter and read it**.
- Log files created by lumberjack use `0600`, but rotation **inherits the permissions of an existing file** — if you manually created a world-readable `modeltaps.log` beforehand, those loose permissions are carried forward indefinitely.

**2. The data directory holds the database and plaintext configuration**

The mount point (`/data/modeltaps` in the examples) typically contains:

- `modeltaps.db`: the SQLite database (`SQLITE_PATH`, relative to the working directory by default, so `/data/modeltaps.db`), containing user password hashes, user tokens, channel API keys and more; the application creates it with mode `0644`, so other users on the host can read it by default;
- `config.yaml`: plaintext `user_token_secret`, `session_secret`, `sql_dsn` and similar;
- `.env`: plaintext secrets;
- `logs/`: the log files mentioned above.

::: tip Uploaded images are not written to local disk
Image storage uploads (`STORAGE_*`) always go straight to the remote service (sm.ms / imgur / Alibaba Cloud OSS / S3 protocol). Modeltaps **never** creates a local upload directory, so no permissions are needed for uploaded content — what needs protecting is the `config.yaml` / `.env` holding those storage credentials. See [Image Storage](./storage.md) for details.
:::

### Docker / Docker Compose deployment

The image runs as the fixed non-root user `modeltaps` (UID and GID both `10001`). The owner of the host mount directory must match, otherwise the container has no write access:

```bash
# align ownership with modeltaps(10001) inside the container
sudo chown -R 10001:10001 /data/modeltaps

# mount root and log directory: owner-only read/write/execute
sudo chmod 700 /data/modeltaps
sudo chmod 700 /data/modeltaps/logs

# sensitive files: owner-only read/write (skip entries whose file does not exist)
sudo chmod 600 /data/modeltaps/.env \
               /data/modeltaps/config.yaml \
               /data/modeltaps/modeltaps.db \
               /data/modeltaps/logs/modeltaps.log
```

With the directory set to `700`, ordinary operations accounts on the host need `sudo` to view these files — which is exactly the intent.

### Bare-metal (manual) deployment

Do not run the binary as root. Create a dedicated system user with no login shell and give it ownership of the installation directory:

```bash
# create the dedicated system user
sudo useradd --system --home-dir /opt/modeltaps --shell /usr/sbin/nologin modeltaps

# hand ownership to the dedicated user; only the owner can enter the directory
sudo chown -R modeltaps:modeltaps /opt/modeltaps
sudo chmod 750 /opt/modeltaps
sudo chmod 700 /opt/modeltaps/logs

# sensitive files: owner-only read/write
sudo chmod 600 /opt/modeltaps/config.yaml \
               /opt/modeltaps/modeltaps.db \
               /opt/modeltaps/logs/modeltaps.log
```

Start it as that user (`--log-dir` points explicitly at the tightened directory so the program does not create one with a loose umask):

```bash
sudo -u modeltaps /opt/modeltaps/modeltaps --port 3000 --log-dir /opt/modeltaps/logs
```

::: tip Create the directory before starting
Create the `logs` directory with the commands above and set it to `700` first, so the program never takes the "auto-create with `0777`" path.
:::

### Cleaning up a recorded initial password

After changing the root password, the initial one is still stored in plaintext in the logs and must be cleaned up:

```bash
# 1. check whether the line exists in the logs
sudo grep -l "initial root password" /data/modeltaps/logs/*.log

# 2. delete the log files containing it (including rotated history files)
sudo rm -f /data/modeltaps/logs/modeltaps.log /data/modeltaps/logs/modeltaps-*.log
```

The container's standard output is stored separately in Docker's own log file, which the `rm` above does not clear. To check and clean it:

```bash
# check
docker logs modeltaps 2>&1 | grep "initial root password"
# clean: recreate the container (data lives in the mounted volume and is not lost)
docker rm -f modeltaps && docker run -d ...   # recreate with your original start command
```

::: tip A cleaner approach: set ROOT_PASSWORD before the first start
Set `ROOT_PASSWORD` in `.env` **before the first startup** (8–64 characters; the program fails to start if the length is out of range). The program then never generates a random password, and the log only contains `password is set via ROOT_PASSWORD env` without the password itself. See the `ROOT_PASSWORD` entry under [Environment Variables](./env.md#variable-reference).
:::

## Administrator break-glass access

Once the site's account system is switched to an external **identity provider**, regular users can only sign in through that provider. So that nobody is locked out of the admin backend when the provider becomes unavailable (key rotation, domain change, shutdown, misconfiguration), Modeltaps keeps one login path for the root account that does not depend on the identity provider.

- **Break-glass entry**: in external identity provider mode, `/login/admin` shows an administrator sign-in form (password or passkey), and only the root account gets through; any other account submitting that form is rejected. The entry keeps every check that applies to password login (account lockout, failure counting, human verification), so the security level is unchanged. The old address `/login?local=1` redirects here permanently.
- **It can be switched off**: Admin → System Settings → Sign-in → Admin emergency sign-in (`AdminLoginEnabled`, on by default) disables the entry; once it is off, `/login/admin` and root's local password and passkeys stop working and the identity provider is the only way in, so make sure the provider itself has a disaster-recovery plan first.
- **Before you switch**: **before** switching to the external identity provider, give root a strong password (16 characters or more, kept in a password manager) and register at least one passkey under Settings → Sign-in & security → Sign-in methods → Passkeys as a second path that does not depend on the identity provider.
- **Rotate at any time**: in external mode root can still change that password and manage passkeys under Settings → Sign-in & security → Admin emergency sign-in; every other account cannot set or change a local password in this state.
- **`ROOT_PASSWORD` is only an initial value**: the variable takes effect only when the database is first created (no users exist yet). Changing or removing it afterwards does not affect the existing root password, so it is not a break-glass mechanism. See the `ROOT_PASSWORD` entry under [Environment Variables](./env.md#variable-reference).

## Multi-node deployment

### Prerequisites

1. Make sure every server has the required components installed:

- Docker, or the components needed for a manual deployment
- Redis (if you want caching)
- A MySQL client (if you use a remote MySQL)

2. Network configuration:

- Make sure every server can reach the primary database
- If you use Redis, make sure the Redis server is reachable
- Review the firewall rules between servers

Then:

1. Set the same `SESSION_SECRET` on every server.
2. `SQL_DSN` is mandatory: use MySQL rather than SQLite, and point every server at the same database.
3. Every slave server must set `NODE_TYPE` to `slave`; when unset, a server defaults to master.
4. With `SYNC_FREQUENCY` set, servers periodically sync configuration from the database. When using a remote database, setting this and enabling Redis is recommended for both master and slave nodes.
5. Slave servers may optionally set `FRONTEND_BASE_URL` to redirect page requests to the master server.
6. Install Redis on each slave server **separately** and set `REDIS_CONN_STRING`; this achieves zero database access while the cache is valid and reduces latency.
7. If the master server also has high database latency, enable Redis there too and set `SYNC_FREQUENCY` so configuration is synced from the database periodically.
