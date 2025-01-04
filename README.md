# Sdump

An opensource HTTP request bin built over SSH

Sdump is a HTTP request bin built over SSH. I usually have to google for some
3rd party server or download a binary to inspect certain requests.

Inspect, test and debug any request or webhook from your terminal

![sdump TUI](assets/sdump.png)

## Why?

There is always a new flavor of an online HTTP request bin i have to use every other
month. Or sometimes, people have to download Ngrok to get a request bin
functionality i.e people have to download or use external services to
get a request bin.

I spend an awful amount of time in the terminal and it makes sense i should
be able to spin up and use a request bin on my terminal. Hence this
project sdump

### How to use public hosted version

```sh
ssh -t -p 2222 ssh.sdump.app
```

#### You can also proxy to your local server

```sh
ssh -p 2222 -t ssh.sdump.app http 4200
```

> URLs are persisted by default as long as you connect with the same ssh key

## Getting started

- `sdump http`: starts the HTTP server.
- `sdump ssh`: starts the SSH server
- `sdump delete-http`: deletes/prunes old ingested requests. This can be a form
  of a cron job that runs every few days or so

### Configuration file

Here is a full config file for all possible values:

```yaml
## log level
log: debug

cron:
  ## how often should the `delete-http` command run
  ttl: "48h"
  ## Do soft deletes or actually wipe them off the database
  soft_deletes: false

tui:
  ## the color_scheme to use for the request body
  # see https://github.com/alecthomas/chroma/tree/master/styles
  color_scheme: monokai

ssh:
  ## port to run ssh server on
  port: 2222
  ## allow_list is a list of public keys that can connect to the ssh server
  # this is useful if you were running a private instance for a few coworkers
  # or friends
  allow_list:
    - ./.ssh/id_rsa.pub
    - /Users/lanreadelowo/.ssh/id_rsa.pub

  ## keys for the ssh server
  identities:
    - "id_ed25519"

http:
  ## port to run http server on
  port: 4200
  ## what domain name you want to use?
  domain: http://localhost:4200
  ## rate limiting clients
  rate_limit:
    ## limit the number of ingested requests from a specific client
    requests_per_minute: 60
  ## database configuration. postgres essentially
  database:
    ## database dsn
    dsn: postgres://sdump:sdump@localhost:3432/sdump?sslmode=disable
    ## should we log sql queries? In prod, no but in local mode,
    ## you probably want to
    log_queries: true

  #  limit the size of jSON request body that can be sent to endpoints
  max_request_body_size: 500

  ## Opentelemetry and tracing config
  otel:
    ## does OTEL endpoint have tls enabled?
    use_tls: true
    ## custom name you want to use to identify the service
    service_name: SDUMP
    ## OTEL Endpoint
    endpoint: http://localhost:4200
    ## Should we trace all http and DB requests
    is_enabled: false

  ## Prometheus configuration
  prometheus:
    ## protect your /metrics endpoint with basic auth
    ## if provided, password must also be provided too
    username: sdump
    ## basic auth password for your /metrics
    password: sdump

    ## enable /metrics endpoint and metrics collection?
    is_enabled: true
```

### Developers' note

Use `ssh-keygen -f .ssh/id_rsa` to generate a test ssh key

### Deployment to your own server?

I have added a [guide](./deploy/README.md) here on how I have
deployed the public version
