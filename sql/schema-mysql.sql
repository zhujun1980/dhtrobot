drop table if exists `Nodes`;
drop table if exists `Resources`;

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
    ctime TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    primary key(infohash)
) engine InnoDB;
