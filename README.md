# dingopie

> [D1N0P13](https://github.com/nblair2/d1n0p13) in Go... and better

```
        _ _                         _      
       | (_)                       (_)     
     __| |_ _ __   __ _  ___  _ __  _  ___ 
    / _  | | '_ \ / _  |/ _ \| '_ \| |/ _ \
   | (_| | | | | | (_| | (_) | |_) | |  __/
    \__,_|_|_| |_|\__, |\___/| .__/|_|\___|
                    _/ |     | |           
                   |___/     |_|           

              |\_/|           ) (
             /     \         ) ( )
            /_ ~ ~ _\      .:::::::.
               \@/        ~\_______/~
```

## Status

#### vAlpha

Simple outstation -> master channel riding on existing DNP3, ~10B/s

* [X] cli
* [X] NFQUEUE intercept
* [ ] parse DNP3 messages and add additional (need gopacket module)
* [ ] receiver (master)

#### vBravo

* [ ] send own DNP3 (not riding on existing connection)
* [ ] send master -> outstation

#### V1.0

* [ ] bidirectional communication
* [ ] interactive shell
* [ ] error checking and correction

## Idea

#### vAlpha

* intercept outgoing DNP3 messages at outstation
* if there is a DNP3 application response, set the reserverd IIN field(s), add a couple of points with our data
  * both fields set indicate start / end message
  * alternate fields set inbetween
* intercept incoming DNP3 messages at master
* if there is an application response and any of the reserved IIN are set, strip the points with our data before passing on.

#### vBravo

? use application codes? Spam it? other?