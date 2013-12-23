DROP TABLE IF exists `Nodes`;
DROP TABLE IF exists `Resources`;
DROP TABLE IF exists `Peers`;

create table `Nodes`
(
    nodeid VARCHAR(40) not null,
    routing TEXT,
    ctime TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    utime TIMESTAMP,
    primary key(nodeid)
) engine InnoDB;

create table `Resources`
(
    infohash VARCHAR(40) not null,
    info TEXT,
    ctime TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    primary key(infohash)
) engine InnoDB;

create table `Peers`
(
    id int(11) NOT NULL AUTO_INCREMENT,
    infohash VARCHAR(40) not null,
    peers TEXT,
    ctime TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    primary key(id),
    key(infohash)
) engine InnoDB;
