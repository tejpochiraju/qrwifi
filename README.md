# qrwifi

Share your Wi-Fi as a QR code and join a network from one, on Linux (NetworkManager).

```
qrwifi show                 # QR for the active network, drawn in the terminal
qrwifi show -s NAME         # a different saved network
qrwifi show -o wifi.png     # write a PNG instead
qrwifi connect photo.png    # join the network encoded in a QR image
qrwifi connect "WIFI:T:WPA;S:name;P:pass;;"
```

Requires `nmcli`. Build with `go build -o qrwifi .`.

`show` assumes the saved connection's profile name matches its SSID (the NetworkManager default).
