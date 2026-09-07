# GammuWrapper

## A simple way to send SMS locally (no internet connectivity required)

### Prerequisites

- USB Modem
- Docker installation

Tested with Huawei E169, but should work with any [Gammu](https://wammu.eu/smsd/) compatible USB modem.

### Docker install

```shell
docker pull ghcr.io/guigui42/gammuwrapper:latest
```

Use one of the example Docker Compose files:

- [Standalone example](docker_example/docker-compose.yml)
- [Uptime Kuma example](docker_example/docker-compose-uptimekuma.yml)

Map your USB modem to:

```text
/dev/ttyUSB1
```

The default send timeout is 45 seconds. Keep production values greater than 30 seconds so Gammu can complete its normal mobile-network wait. Override it with:

```yaml
environment:
  GAMMUSENDTIMEOUTSECONDS: "60"
```

### How it works

- Built with Go
- Uses [Gammu](https://wammu.eu/smsd/) to manage the USB modem
- Serializes modem access so only one Gammu operation runs at a time
- Waits for `gammu --sendsms` to finish before returning HTTP success
- Uses the Chi web server to handle API calls

### REST call Example

Send an SMS with a `POST` request to `http://gammudocker:8083/sendsms`:

```json
{
  "phone_number": "+15555550100",
  "message": "Test notification"
}
```

A `200` response means the Gammu subprocess completed successfully. Errors use non-2xx responses:

| Status | Meaning |
| --- | --- |
| `400` | Invalid JSON or missing required fields |
| `429` | Another modem operation is already running |
| `502` | Gammu exited unsuccessfully |
| `504` | Gammu exceeded `GAMMUSENDTIMEOUTSECONDS` |

### Diagnostics

The diagnostic endpoints never send an SMS:

| Endpoint | Check |
| --- | --- |
| `GET /health` | HTTP process is responding |
| `GET /health/modem` | Gammu can identify the modem device |
| `GET /health/network` | The modem reports home or roaming network registration |

The Docker health check uses `/health`, so a missing modem does not restart an otherwise healthy HTTP process. Monitor `/health/modem` and `/health/network` separately when you need device and mobile-network diagnostics.

### Uptime Kuma

GammuWrapper can be used as an Uptime Kuma custom webhook notification. Configure the webhook URL as:

```text
http://gammuwrapperuptime:8083/sendsms
```

Use this custom JSON body, replacing the placeholder with your destination number:

```json
{
  "phone_number": "+15555550100",
  "message": "Uptime Kuma alert - {{ monitorJSON['name'] }} {{ msg }}"
}
```

Uptime Kuma records success only after Gammu finishes sending. A busy modem, timeout, or Gammu failure returns a non-2xx status.

### TODO

- Better documentation
- Uptime Kuma instructions

> [!CAUTION]
> **Security notes**
>
> There is no authentication or authorization.
>
> Run this service only on a trusted local network. Do not expose it to the internet.
