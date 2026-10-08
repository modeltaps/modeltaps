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
