DHTRobot
=======

A kademila DHT implementation in go

## Install

~~~
CGO_LDFLAGS="-L{Path to libreadline & libhistory}" go build github.com/zhujun1980/dhtrobot
~~~

## Run

~~~
./dhtrobot
~~~

## Client mode

A debug tool for dhtrobot

~~~
./dhtrobot -client

DHTRobot 0.1.0, Type 'help' show help page
Local node ID: 8222e4b39b57cd88524d0ef54f9886beb2155789
[1] >>> connect c1 ip port

[2] >>> ping c1
--> Message T=0001, Y=q, V=, Q=ping, ID=8222e4b39b57cd88524d0ef54f9886beb2155789, SendNode(ID=, Addr=, Status=0)
--> Packet-Sent: bytes=56, waiting for response (5s timeout)
<-- Packet-received: bytes=47 from=127.0.0.1:58527
<-- Message T=0001, Y=r, V=, Q=ping, ID=b4cfafee49b5ce879da9e2feff23a8755209017f, SendNode(ID=b4cfafee49b5ce879da9e2feff23a8755209017f, Addr=127.0.0.1:58527, Status=0)

[3] >>> find c1 a3abdd68d8c3e2045fd8eab997e8bfa27f287e81
--> Message T=0004, Y=q, V=, Q=find_node, ID=8222e4b39b57cd88524d0ef54f9886beb2155789, Target=a3abdd68d8c3e2045fd8eab997e8bfa27f287e81, SendNode(ID=, Addr=, Status=0)
--> Packet-Sent: bytes=92, waiting for response (5s timeout)
<-- Packet-received: bytes=162 from=127.0.0.1:58527
<-- Message T=0004, Y=r, V=, Q=find_node, ID=b4cfafee49b5ce879da9e2feff23a8755209017f, Length=4, Nodes=[<0, ID=a3abdd68d8c3e2045fd8eab997e8bfa27f287e84, Addr=184.218.148.119:12745>; <1, ID=a72c7a80fcafd96cdcc7e6005bdceebd93ecbba6, Addr=162.234.63.121:6698>; <2, ID=a0dbc0e1075cdbe9ba94ebbca17c2b7d87fba8de, Addr=178.200.12.219:29313>; <3, ID=a4ffacda6fdfc4e50358dfedbe90efb8a5782f79, Addr=172.236.64.75:15107>], SendNode(ID=b4cfafee49b5ce879da9e2feff23a8755209017f, Addr=127.0.0.1:58527, Status=0)
4 nodes received
0 ID=a3abdd68d8c3e2045fd8eab997e8bfa27f287e84, Addr=184.218.148.119:12745, Status=0, Distance=3, Distance=158
1 ID=a72c7a80fcafd96cdcc7e6005bdceebd93ecbba6, Addr=162.234.63.121:6698, Status=0, Distance=155, Distance=158
2 ID=a0dbc0e1075cdbe9ba94ebbca17c2b7d87fba8de, Addr=178.200.12.219:29313, Status=0, Distance=154, Distance=158
3 ID=a4ffacda6fdfc4e50358dfedbe90efb8a5782f79, Addr=172.236.64.75:15107, Status=0, Distance=155, Distance=158

~~~


## References

1. <http://pdos.csail.mit.edu/~petar/papers/maymounkov-kademlia-lncs.pdf>
2. <http://www.bittorrent.org/beps/bep_0005.html>
3. <https://en.wikipedia.org/wiki/Kademlia>
4. <http://codemacro.com/2013/05/19/crawl-dht/>
5. <http://wenku.baidu.com/view/022f4552ad02de80d4d84062.html>
6. <http://wenku.baidu.com/view/ee91580216fc700abb68fcae.html>
7. <http://blog.csdn.net/tsingmei/article/details/2924368>
8. <http://blog.csdn.net/xxxxxx91116/article/details/8549454>
9. <http://www.bittorrent.org/beps/bep_0009.html>
