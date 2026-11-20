---
title: "Auto Upgrade"
layout: doc
outline: deep
lastUpdated: true
---

# Automatic Upgrade

::: warning Sign in to the private registry first
The upgrade scripts below run `docker pull ghcr.io/modeltaps/modeltaps:latest` against a private image. The host running the script must first complete [Signing in to the private registry](./index.md#signing-in-to-the-private-registry), otherwise the pull fails with an authorization error.
:::

::: danger Upgrades cannot simply be rolled back
From the version that stores API keys as hashes, the database keeps only the hash of each key and the plaintext is cleared. This step cannot be undone: downgrading to an older version makes every API key invalid. Back up the database before upgrading, and restore that backup together with the old version if you need to roll back. The version that introduces management keys does the same with each account's access token: it is moved to a hash and the plaintext cleared, so after a downgrade no old access token works.
:::

### Upgrading from 0.5.0 to 0.6.0

The database is migrated by itself. Check these after starting the new version:

- **Supply routing** stays as it was on an installation that already has users. To order, share and pause channels by supplies (dragging and priorities on the providers page), turn on **Route by supplies** in **Platform settings › Gateway › Supply mode and cost**. Saving it first migrates the channels to supplies, so no channel model stops being served.
- **Prompt cache:** an installation that never saved the site-wide prompt cache switch turns it on. Set API keys used for one-off prompts, where a cache write costs 25% more, to Off.
- **Supply pause rules:** the switch is gone and every failed request goes through the pause rules. Channels are disabled automatically only by channel tests, no longer on live requests.
- **Request body retention** is set on the account or the organization, not on the key. What was kept stays kept.

### Upgrading from 0.4.0 to 0.5.0

Do these before starting the new version; the database is migrated by itself.

- **Docker Compose:** add `REDIS_PASSWORD` to `.env` (`openssl rand -hex 32`). The bundled Redis requires it and `docker compose up` stops without it.
- **Set `ENCRYPTION_KEY`** (`openssl rand -base64 32`) if it is not set yet. The first start moves image storage from the `storage:` section of `config.yaml` (or `STORAGE_*`) into the platform settings, and its secrets are stored encrypted only with a key. The section is not read afterwards.
- **Behind a reverse proxy that rewrites the Host header,** set the server address in the settings: cookie-authenticated changes must come from the site's own origin.
- **Clients of own routing policies** (personal and organization) must call them as `policy/<name>`; a bare name now means a managed policy or a model.
- **Scripts calling these endpoints with GET** must use POST: `/api/channel/test`, `/api/channel/test/:id`, `/api/channel/check/:id`, `/api/channel/update_balance`, `/api/channel/update_balance/:id`, `/api/user/logout`, `/api/user/token`, `/api/verification`, `/api/reset_password`.
- **Billing:** the "Never charge below the channel cost" switch is gone. Requests are charged the catalog price times the group ratio; to keep a margin, raise the default group's ratio.

A user who lost both their authenticator and their recovery codes can have two-step verification turned off by root with `DELETE /api/user/<id>/totp`.

### Using a control panel scheduled task

If you deploy through an operations panel such as aaPanel, you can add a scheduled task directly.

Add scheduled task -> Shell script -> paste the script below.

When deploying with Docker Compose, change the directory in the script to the one containing `docker-compose.yml`:

```
cd /www/wwwroot/
```

### Using Crontab

The script also works in Crontab, but make sure Crontab can locate the correct Docker binary — absolute paths are recommended.

Configure Crontab to run the script every 5 minutes:

```
*/5 * * * * /path/to/modeltaps-auto-update.sh >> /path/to/modeltaps-auto-update.log 2>&1
```

### Docker Compose script

```sh
# Pull the latest image and capture the output in a variable
output=$(docker pull ghcr.io/modeltaps/modeltaps:latest 2>&1)

# Check whether the pull succeeded
if [ $? -ne 0 ]; then
exit 1
fi

# Check whether the output contains the up-to-date marker
echo "$output" | grep -q "Image is up to date for ghcr.io/modeltaps/modeltaps:latest"

# Do nothing if the image is already up to date
if [ $? -eq 0 ]; then
exit 0
fi

echo "modeltaps update detected"

# Remove the old container
echo "removed: $(docker rm -f modeltaps)"

# Switch to the directory containing `docker-compose.yml` first
cd /www/wwwroot/

# Start the new container
echo "started: $(docker-compose up)"

# Print the update time and version
echo "updated at: $(date)"
echo "version: $(docker inspect ghcr.io/modeltaps/modeltaps:latest | grep 'org.opencontainers.image.version' | awk -F'"' '{print $4}')"

# Clean up unused images
docker images | grep 'ghcr.io/modeltaps/modeltaps' | grep -v 'latest' | awk '{print $3}' | xargs -r docker rmi > /dev/null 2>&1
echo "old images removed."
```

### Plain `docker run` script

```sh
# Pull the latest image and capture the output in a variable
output=$(docker pull ghcr.io/modeltaps/modeltaps:latest 2>&1)

# Check whether the pull succeeded
if [ $? -ne 0 ]; then
exit 1
fi

# Check whether the output contains the up-to-date marker
echo "$output" | grep -q "Image is up to date for ghcr.io/modeltaps/modeltaps:latest"

# Do nothing if the image is already up to date
if [ $? -eq 0 ]; then
exit 0
fi

echo "modeltaps update detected"

# Remove the old container
echo "removed: $(docker rm -f modeltaps)"

# Switch to the directory containing `docker-compose.yml` first
cd /www/wwwroot/

# Start the new container. This is the SQLite variant — adjust it to match your deployment.
echo "started: $(docker run -d -p 3000:3000 --name modeltaps --restart always -e TZ=UTC -v /home/ubuntu/data/modeltaps:/data ghcr.io/modeltaps/modeltaps:latest)"

# Print the update time and version
echo "updated at: $(date)"
echo "version: $(docker inspect ghcr.io/modeltaps/modeltaps:latest | grep 'org.opencontainers.image.version' | awk -F'"' '{print $4}')"

# Clean up unused images
docker images | grep 'ghcr.io/modeltaps/modeltaps' | grep -v 'latest' | awk '{print $3}' | xargs -r docker rmi > /dev/null 2>&1
echo "old images removed."
```

### Self-check: users.email must allow NULL

Accounts without an email store NULL in `users.email`, and the unique email constraint relies on that. If this column was ever changed to `NOT NULL` by hand, Modeltaps makes it nullable again on startup (keeping its type, character set and collation, and changing no data) and logs the SQL it ran.

If the database user has no `ALTER` privilege, the automatic fix fails and startup stops; the log shows the SQL to run. Run it as a user with `ALTER` privilege and start Modeltaps again.

To check whether the column allows NULL (`YES` means it is fine):

```sql
-- MySQL
SELECT IS_NULLABLE, COLUMN_TYPE FROM information_schema.COLUMNS
WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'users' AND COLUMN_NAME = 'email';

-- PostgreSQL
SELECT is_nullable FROM information_schema.columns
WHERE table_schema = current_schema() AND table_name = 'users' AND column_name = 'email';
```

To fix it by hand (on MySQL, replace `varchar(255)` with the `COLUMN_TYPE` returned above and keep the column's character set and collation):

```sql
-- MySQL
ALTER TABLE `users` MODIFY COLUMN `email` varchar(255) NULL;

-- PostgreSQL
ALTER TABLE users ALTER COLUMN email DROP NOT NULL;
```
