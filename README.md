DHTROBOT
=======
使用Go实现的BT分布式哈希表爬虫

###DHT
DHT，分布式哈希表，它是一种去中心化的key-value存储系统。它没有中央服务器，所有的数据都分散的存储在网络中的各个节点(Node)上。在文件分享领域，它主要用来保存网络上各个节点的连接信息和资源位置信息。目前几乎所有的P2P工具都支持DHT，避免对于Tracker服务器的依赖。

###Kademlia算法
DHT有很多实现方案，比如Chord, Pastry和Kademlia。其中在P2P领域中应用最广泛的是Kademlia。大部分工具都是基于Kademlia实现的DHT。

Kademlia中支持DHT协议的网络应用程序称为`Node`。每个Node有个Node ID，它是个160 bits的串，其实就是一个SHA1的哈希值。Kademlia使用Node ID之间的异或值（XOR）来确定节点间的*距离*。注意这个距离不是说两个节点间的物理距离，也不是它们之间的IP跳数，而仅仅是逻辑上的距离。Kademlia协议本质上使得每个节点仅可能的保存自己*邻居*的信息。

Node ID有160 bits，所以有2<sup>160</sup>个地址。Kademlia把地址分为160个桶，对于0 <= i < 160来说，每个桶保存与当前Node距离是 [2<sup>i</sup>, 2<sup>i+1</sup>)范围内的节点信息。这样在查询某个节点时能够很快定位到它所在的区间。注意真正的要查询的节点未必在网络上，但是只要能查到它附近的节点，就能获得对应的信息，因为Kademlia使节点对于自己的邻居最熟悉。

具体细节请参考下面的引用

###DHTROBOT
DHTROBOT就是实现Kademlia协议，让自己成为Node加入DHT网络，从上面分析请求并记录在mysql中。接受到Announce_Peer消息保存下来

###Why Go?
DHT的爬虫有很多人已经实现过，有点用erlang，有的用python。我写这个程序目的是想学习Go。它非常好用，编译很快，库比较完善，尤其是goroutine。

###依赖
    go get github.com/zeebo/bencode

###ChangeLog

_**[2014-12-11]**_  Use sqlite save search result, install method:

~~~
> go get github.com/mattn/go-sqlite3
> sqlite3 dhtrobot.db 
create tables.....
~~~

###References
1. <http://pdos.csail.mit.edu/~petar/papers/maymounkov-kademlia-lncs.pdf>
2. <http://www.bittorrent.org/beps/bep_0005.html>
3. <https://en.wikipedia.org/wiki/Kademlia>
4. <http://codemacro.com/2013/05/19/crawl-dht/>
5. <http://wenku.baidu.com/view/022f4552ad02de80d4d84062.html>
6. <http://wenku.baidu.com/view/ee91580216fc700abb68fcae.html>
7. <http://blog.csdn.net/tsingmei/article/details/2924368>
8. <http://blog.csdn.net/xxxxxx91116/article/details/8549454>
9. <http://www.bittorrent.org/beps/bep_0009.html>
