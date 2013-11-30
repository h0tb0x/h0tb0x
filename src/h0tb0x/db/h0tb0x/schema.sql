CREATE TABLE Record(
	topic           TEXT    NOT NULL,
	type            CHAR    NOT NULL,
	key             TEXT    NOT NULL,
	value           BLOB    NULL,
	seqno           INTEGER NOT NULL,
	priority        INTEGER NOT NULL DEFAULT(0),
	author          TEXT    NOT NULL,
	signature       BLOB    NULL,
	PRIMARY KEY     (topic, type, author, key)
);
CREATE UNIQUE INDEX IDX_Record_01 ON Record (seqno);
CREATE INDEX        IDX_Record_02 ON Record (topic, type, key, priority);

CREATE TABLE Attribute(
	seqno           INTEGER NOT NULL,
	name            TEXT    NOT NULL,
	value           TEXT    NOT NULL,
	PRIMARY KEY     (seqno, name, value)
);

CREATE TABLE Friend(
	id              INTEGER NOT NULL PRIMARY KEY,
	fingerprint     BLOB    NOT NULL,
	rendezvous      TEXT    NOT NULL,
	public_key      BLOB    NULL,
	-- Filled with '$' when 'empty' to deal with borked DB handling in go
	host            TEXT    NOT NULL,
	port            INTEGER NOT NULL DEFAULT (0)
);
CREATE UNIQUE INDEX IDX_Friend_01 ON Friend (fingerprint);

CREATE TABLE TopicFriend(
	friend_id       INTEGER NOT NULL,
	topic           TEXT    NOT NULL,
	-- Do I want to talk to friend_id about topic?
	desired         INTEGER NOT NULL DEFAULT(0),
	-- Does friends_id want to talk to me about topic?
	requested       INTEGER NOT NULL DEFAULT(0),
	-- Last seqno of data I'm sending which was acked by remote
	acked_seqno     INTEGER NOT NULL DEFAULT(0), 
	-- Last seqno I heard from the remote side	
	heard_seqno     INTEGER NOT NULL DEFAULT(0), 
	PRIMARY KEY     (friend_id, topic)
);
CREATE INDEX        IDX_TopicFriend_01 ON TopicFriend (friend_id, desired, requested);

CREATE TABLE Blob(
	key             TEXT    NOT NULL PRIMARY KEY,
	needs_download  BOOL    NOT NULL,
	data            BLOB    NOT NULL
);
CREATE UNIQUE INDEX IDX_Blob_01 ON Blob (key);
CREATE INDEX        IDX_Blob_02 ON Blob (needs_download);

-- Represents incoming adverts
CREATE TABLE Advert(
	key             TEXT    NOT NULL,
	friend_id       INTEGER NOT NULL,
	topic           TEXT    NOT NULL,
	PRIMARY KEY     (key, friend_id, topic)
);
