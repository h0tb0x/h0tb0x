CREATE TABLE Object(
	topic_id     INTEGER NOT NULL,
	type         CHAR NOT NULL,
	author       TEXT NOT NULL,
	key          TEXT NOT NULL,
	value        BLOB NULL,
	seqno        INTEGER NOT NULL,
	priority     INTEGER NOT NULL DEFAULT(0),
	signature    BLOB NULL,
	PRIMARY KEY(topic_id, type, author, key)
);
CREATE UNIQUE INDEX IDX_Object_seqno ON Object (seqno);
CREATE INDEX        IDX_Object_ttkp ON Object (topic_id, type, key, priority);

CREATE TABLE Friend(
	friend_id    INTEGER PRIMARY KEY NOT NULL,
	fingerprint  BLOB NOT NULL,
	rendezvous   TEXT NOT NULL,
	public_key   BLOB NULL,
	host         TEXT NOT NULL,  -- Filled with '$' when 'empty' to deal with borked DB handling in go
	port         INTEGER NOT NULL DEFAULT (0)
);
CREATE UNIQUE INDEX IDX_Friend_fingerprint ON Friend (fingerprint);

CREATE TABLE TopicFriend(
	friend_id    INTEGER NOT NULL,
	topic_id     INTEGER NOT NULL,
	desired      INTEGER NOT NULL DEFAULT(0), -- Do I want to talk to friend_id about topic?
	requested    INTEGER NOT NULL DEFAULT(0), -- Does friends_id want to talk to me about topic?
	acked_seqno  INTEGER NOT NULL DEFAULT(0), -- Last seqno of data I'm sending which was acked by remote
	heard_seqno  INTEGER NOT NULL DEFAULT(0), -- Last seqno I heard from the remote side	
	PRIMARY KEY(friend_id, topic_id)
);
CREATE INDEX IDX_TopicFriend_fdr ON TopicFriend (friend_id, desired, requested);

CREATE TABLE Blob(
	key            TEXT NOT NULL PRIMARY KEY,
	needs_download BOOL NOT NULL,
	data           BLOB NOT NULL
);
CREATE UNIQUE INDEX IDX_Blob ON Blob (key);
CREATE INDEX IDX_Blob_needs_download ON Blob (needs_download);

-- Represents incoming adverts
CREATE TABLE Advert(
	key          TEXT NOT NULL,
	friend_id    INTEGER NOT NULL,
	topic_id     INTEGER NOT NULL,
	PRIMARY KEY(key, friend_id, topic_id)
);

-- Most of the data for an advert is for *inbound* adverts
-- That is, what I last heard from each friend regarding the destination
-- But I also keep my local data in the same table, with -1 for source

--CREATE TABLE Advert(
--	dest_id INTEGER NOT NULL,
--	source_id INTEGER NOT NULL,  -- -1 if self
--	cost INTEGER NOT NULL, -- -1 if inf
--	downhill_id INTEGER NOT NULL, -- friend_id if source_id = -1, otherwise 0 (not self) or 1 (self) 
--	timestamp INTEGER NOT NULL,
--	desired INTEGER NOT NULL,
--	PRIMARY KEY(dest_id, source_id)
--);

CREATE TABLE Topic(
	topic_id     INTEGER PRIMARY KEY NOT NULL,
	name         TEXT NOT NULL
);
CREATE UNIQUE INDEX IDX_Topic_name ON Topic (name);

CREATE TABLE TopicGroup(
	topic_group_id  INTEGER NOT NULL,
	topic_id        INTEGER NOT NULL,
	name            TEXT NOT NULL,
	PRIMARY KEY(topic_group_id, topic_id)
);
CREATE UNIQUE INDEX IDX_TopicGroup_name ON TopicGroup (name);

-- CREATE TABLE Attribute(
-- 	name         TEXT NOT NULL,
-- 	value        TEXT NOT NULL,
-- 	oid          INTEGER NOT NULL,
-- 	PRIMARY KEY(name, value, oid)
-- );
-- CREATE INDEX IDX_Attribute_value ON Attribute (value);
-- CREATE INDEX IDX_Attribute_object_id ON Attribute (oid);
