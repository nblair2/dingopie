# dingopie :wolf: :cake:

> [D1N0P13](https://github.com/nblair2/d1n0p13) in Go... and better

```
   |\_/|     ▌▘        ▘       ) (
  /     \   ▛▌▌▛▌▛▌▛▌▛▌▌█▌    ) ( )
 /_ ~ ~ _\  ▙▌▌▌▌▙▌▙▌▙▌▌▙▖  .:::::::.
    \@/          ▄▌  ▌     ~\_______/~
```
## Modes

### Forge

In forge mode, dingopie simply crafts its own DNP3 messages and sends the data in the DNP3 Application Objects. This traffic will be legitimate protocol-conforming DNP3, but is very noticeable. It will originate on a port and host that are not usually communicating using DNP3, and traffic inspection will likely show unusual usage, both in the amount of data transferred and the DNP3 characteristics. The advantage of forge mode is that it can be configured to run at higher speeds and can run between any two devices, in any direction.

### Filter

:exclamation: NOTE :exclamation: DNP3 filter mode is not implemented. There is some spaghetti code that got started to intercept packets on linux.

In filter mode, dingopie 'rides on top of' an existing DNP3 channel. It will intercept packets as they leave / enter a host, add / remove the additional data, and then allow the packet to continue on to the legitimate SCADA program. This will show as an increase to the overall size of the packets being sent between devices, but will take place over an existing DNP3 connection and is much less likely to be noticed. The disadvantage of filter mode is that its speed is constrained by the channel that it is using.

## Status

### vBeta - forge (est 30 July 2025)

* [ ] add encryption
* [ ] clean up padded bytes on receipt
* [ ] figure out close of connection from client side
* [ ] add jitter to size of data sent (not all exact same size)

### v0.5 - filter linux

### v0.9 - filter (both)