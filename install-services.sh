#!/bin/bash
set -e

USERS="patrick_van_staveren matt"

echo "Installing sonde-alert@.service..."
sudo cp sonde-alert@.service /etc/systemd/system/
sudo systemctl daemon-reload

for user in $USERS; do
    echo "Enabling and starting sonde-alert@${user}..."
    sudo systemctl enable "sonde-alert@${user}"
    sudo systemctl start "sonde-alert@${user}"
done

echo ""
echo "Done! Checking status..."
for user in $USERS; do
    sudo systemctl status "sonde-alert@${user}" --no-pager || true
    echo ""
done
