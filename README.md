# README

## How to use it?

```
cp config.example.ini config.ini
# edit it for free
sudo go run main.go --config=config.ini
```

## Supported OS (tested)

* [x] OSX 10.13.1
* [x] arch linux

## Known bugs

## TODO

* [ ] log
* [ ] windows support
* [ ] android support
* [ ] hot reload config
* [ ] release

## Thanks

* thanks [xjdrew/kone](https://github.com/xjdrew/kone) for dns fake mode
* thanks [yinghuocho/gotun2socks](https://github.com/yinghuocho/gotun2socks) for udp <-> socks5 tunnel
* thanks [songgao/water](https://github.com/songgao/water) for tun io on many platform
* thanks [google/netstack](https://github.com/google/netstack) for tcp/ip stack
* thanks [google/gopacket](https://github.com/google/gopacket) for pack/unpack ip package
* thanks [miekg/dns](https://github.com/miekg/dns) for dns parser and server
