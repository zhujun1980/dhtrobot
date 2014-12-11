create table Nodes
(
    nodeid text NOT NULL PRIMARY KEY,
    routing TEXT,
    ctime TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    utime TIMESTAMP
);

create table Resources
(
    infohash text NOT NULL PRIMARY KEY,
    info TEXT,
    ctime TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

create table Peers
(
    id integer PRIMARY KEY,
    infohash VARCHAR(40) not null,
    peers TEXT,
    ctime TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

