# Websocket upgrade for Rango ZSmartex

## Messages

### Subscribe to a stream list

```
{"event":"subscribe","streams":["eurusd.trades","eurusd.ob-inc"]}
```

### Unsubscribe to one or several streams

```
{"event":"unsubscribe","streams":["eurusd.trades"]}
```

```
{"event":"subscribe","streams":["btcusd.trades","ethusd.ob-inc","ethusd.trades","xrpusd.ob-inc","xrpusd.trades","usdtusd.ob-inc","usdtusd.trades"]}
```

## Credits
- [Rango ZSmartex](https://github.com/zsmartex/rango)
- [1M Go Websockets](https://github.com/eranyanay/1m-go-websockets)