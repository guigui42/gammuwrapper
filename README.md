GammuWrapper : a simple way to send SMS locally (no internet connectivity required)

Tested with Huawei E169 
But should work with any Gammu compatible usb dongle, see https://wammu.eu/smsd/

Run directly inside docker ( ghcr.io/guigui42/gammuwrapper )

Can be used with Uptime Kuma (using custom Webhooks)

Send an SMS using a simple POST REST call :

http://gammudocker:8083/sendsms

{
    "phone_number" : "XXXXXXXXXXX",
    "message" : "test json 4242"
}

replace XXXXXXXXXXX with your phone number.

TODO : 
Better documentation
