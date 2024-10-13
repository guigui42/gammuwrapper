# GammuWrapper
## A simple way to send SMS locally (no internet connectivity required)

### Prerequisites
- USB Modem
- Docker installation

Tested with Huawei E169 
But should work with any [Gammu](https://wammu.eu/smsd/)  compatible usb modem 

### Docker install

```
docker pull ghcr.io/guigui42/gammuwrapper:latest
```
Use one of the example docker compose files

[compose example](docker_example/docker-compose.yml)

make sure to map your device (usb modem) to this one :
```
/dev/ttyUSB1
```
### How it works
- Built with Go
- Uses [Gammu](https://wammu.eu/smsd/) in the backend to manage and use the USB Modem
- Uses a queue to send SMS to avoid making the device busy and unusable
- Chi webserver to handle API call

### REST call Example
Send an SMS using a simple POST REST call :
```
http://gammudocker:8083/sendsms
```

```
{
    "phone_number" : "XXXXXXXXXXX",
    "message" : "My Text Message"
}
```

replace XXXXXXXXXXX with your phone number.

### Uptime Kuma
Can be used with Uptime Kuma as a Notification method (using custom Webhooks)
it looks something like that :

<img src="ttps://github.com/user-attachments/assets/094c0d02-ce5e-4f74-95ed-b42e7929ef18" width="80" />


Using this custom body :
```
{
"phone_number" : "0033XXXXXXXX",
"message":"Uptime Kuma Altert - {{ monitorJSON['name'] }} {{ msg }}"
}
```

### TODO 
- Better documentation
- Uptime Kuma instructions

> [!CAUTION]
> **Security notes**
> 
> There is no authentification / authorization.
> 
> It is made to run locally only (ie: not opened to the outside world) in a home setup for example and accessed by a third party running along side in Docker (like Uptime Kuma) to send notifications

