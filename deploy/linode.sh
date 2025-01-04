#!/bin/bash

## fetch the token from Infisical
## safe to store here. token Only valid for about about a minute

# Check if required environment variables are set
if [ -z "$INFISICAL_CLIENT_ID" ]; then
	echo "Error: INFISICAL_CLIENT_ID is not set"
	exit 1
fi

if [ -z "$INFISICAL_CLIENT_SECRET" ]; then
	echo "Error: INFISICAL_CLIENT_SECRET is not set"
	exit 1
fi

mkdir -p /etc/infisical
export INFISICAL_TOKEN=$(infisical login --method=universal-auth --client-id="${INFISICAL_CLIENT_ID}" --client-secret="${INFISICAL_CLIENT_SECRET}" --silent --plain)

if [ -z "$INFISICAL_TOKEN" ]; then
	echo "Error: Failed to obtain INFISICAL_TOKEN"
	exit 1
fi

sudo echo "INFISICAL_TOKEN=$INFISICAL_TOKEN" >/etc/infisical/infisical.env

sudo chmod 600 /etc/infisical/infisical.env

## restart service. Service already reads this config file
sudo systemctl stop sdump-http
sudo mv /root/sdump /usr/local/bin/sdump
sudo systemctl restart sdump-http
sudo systemctl status sdump-http

export SDUMP_SSH_PORT=3333
export SDUMP_SSH_HOST=localhost
export SDUMP_SSH_IDENTITIES=/root/.ssh/id_rsa
export SDUMP_HTTP_DOMAIN=https://sdump.app

## kill all screen sesssions and restart
pkill screen
screen -wipe
screen -dmS ssh_server sdump ssh
