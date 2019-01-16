# README

## How to use it? [more](https://github.com/FlowerWrong/tun2socks/wiki)

```
# install golang 1.9+, because of sync.Map
go get -u -v github.com/FlowerWrong/tun2socks
cd tun2socks
go get ./...

cp config.example.ini config.ini
# edit it for free
sudo go run cmd/main.go -h
sudo go run cmd/main.go -v
sudo go run cmd/main.go -c=config.ini
```

## Note

* For darwin and linux, you should use [netstack smart branch](https://github.com/FlowerWrong/netstack/tree/smart).
* For windows user, you should use [netstack master branch](https://github.com/FlowerWrong/netstack).

## Support operating system.

* [x] macOS
* [x] linux
* [x] Windows Vista and above support with [tap-windows6](https://github.com/OpenVPN/tap-windows6), [download link](http://build.openvpn.net/downloads/releases/latest/), please use [netstack master branch](https://github.com/FlowerWrong/netstack).
* [x] Raspberry Pi support
* [x] android support with root

## Hot reload config with `USR2` signal. Not support windows.

Support `route`, `udp.proxy`, `proxy`, `pattern` and `rule`, see [config.example.ini](https://github.com/FlowerWrong/tun2socks/blob/master/config.example.ini).

```bash
sudo kill -s USR2 $PID
```

NOTE: `go run` not support kill command signal.

## As a static library

See [c api wiki](https://github.com/FlowerWrong/tun2socks/wiki/c-api).

Windows build need to install [git](https://git-scm.com/download) + [tdm-gcc](http://tdm-gcc.tdragon.net/download).

## TODO

* [ ] gui
* [ ] ipv6 support

## Thanks

* thanks [xjdrew/kone](https://github.com/xjdrew/kone) for dns fake mode
* thanks [yinghuocho/gotun2socks](https://github.com/yinghuocho/gotun2socks) for udp <-> socks5 tunnel
* thanks [songgao/water](https://github.com/songgao/water) for tun io on many platform
* thanks [google/netstack](https://github.com/google/netstack) for tcp/ip stack
* thanks [google/gopacket](https://github.com/google/gopacket) for pack/unpack ip package
* thanks [miekg/dns](https://github.com/miekg/dns) for dns parser and server
