# README

## How to use it?

```
cp config.example.ini config.ini
# edit it for free
sudo go run cmd/main.go --config=config.ini

go build -o tun2socks
```

## Supported OS (tested)

* [x] OSX 10.13.1
* [x] arch linux

## Hot reload config

Supported `route`, `udp.proxy`, `proxy`, `pattern`, `rule`.

```bash
sudo kill -s USR2 $PID
```

NOTE: go run not support kill command signal.

## Known bugs

## TODO

* [ ] log
* [ ] windows support
* [ ] android support
* [ ] release

## Thanks

* thanks [xjdrew/kone](https://github.com/xjdrew/kone) for dns fake mode
* thanks [yinghuocho/gotun2socks](https://github.com/yinghuocho/gotun2socks) for udp <-> socks5 tunnel
* thanks [songgao/water](https://github.com/songgao/water) for tun io on many platform
* thanks [google/netstack](https://github.com/google/netstack) for tcp/ip stack
* thanks [google/gopacket](https://github.com/google/gopacket) for pack/unpack ip package
* thanks [miekg/dns](https://github.com/miekg/dns) for dns parser and server
