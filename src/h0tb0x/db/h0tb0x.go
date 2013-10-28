package db

func init() {
	schemas["h0tb0x"] = &Schema{
		name: "h0tb0x",
		latest: `
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
`,
		migrations: []string{

			`
CREATE TABLE IF NOT EXISTS Object(
	topic TEXT NOT NULL,
	type CHAR NOT NULL,
	key TEXT NOT NULL,
	value BLOB NULL,
	seqno INTEGER NOT NULL,
	priority INT NOT NULL DEFAULT(0),
	author TEXT NOT NULL,
	signature BLOB NULL,
	PRIMARY KEY(topic, type, author, key)
);

CREATE TABLE IF NOT EXISTS Friend(
	id INTEGER PRIMARY KEY NOT NULL,
	fingerprint BLOB NOT NULL,
	rendezvous TEXT NOT NULL,
	public_key BLOB NULL,
	host TEXT NOT NULL,  -- Filled with '$' when 'empty' to deal with borked DB handling in go
	port INTEGER NOT NULL DEFAULT (0)
);

CREATE TABLE IF NOT EXISTS TopicFriend(
	friend_id INTEGER NOT NULL,
	topic TEXT NOT NULL,
	desired INTEGER NOT NULL DEFAULT(0),  -- Do I want to talk to friend_id about topic?
	requested INTEGER NOT NULL DEFAULT(0),  -- Does friends_id want to talk to me about topic?
	acked_seqno INTEGER NOT NULL DEFAULT(0), -- Last seqno of data I'm sending which was acked by remote
	heard_seqno INTEGER NOT NULL DEFAULT(0), -- Last seqno I heard from the remote side	
	PRIMARY KEY(friend_id, topic)
);

CREATE TABLE IF NOT EXISTS Blob(
	key TEXT NOT NULL PRIMARY KEY,
	needs_download BOOL NOT NULL,
	data BLOB NOT NULL
);

-- Represents incoming adverts
CREATE TABLE IF NOT EXISTS Advert(
	key TEXT NOT NULL,
	friend_id INTEGER NOT NULL,
	topic TEXT NOT NULL,
	PRIMARY KEY(key, friend_id, topic)
);

-- Todo: Move to a seperate schema
-- Also, I should probably make this information private to friends via public key crypto
CREATE TABLE IF NOT EXISTS Rendezvous(
	-- Not in sig
	fingerprint TEXT NOT NULL PRIMARY KEY,
	public_key TEXT NOT NULL,
	-- In sig
	version int NOT NULL,
	host TEXT NOT NULL,
	port int NOT NULL,
	-- The sig
	signature TEXT NOT NULL
);
	
-- Most of the data for an advert is for *inbound* adverts
-- That is, what I last heard from each friend regarding the destination
-- But I also keep my local data in the same table, with -1 for source

--CREATE TABLE IF NOT EXISTS Advert(
--	dest_id INTEGER NOT NULL,
--	source_id INTEGER NOT NULL,  -- -1 if self
--	cost INTEGER NOT NULL, -- -1 if inf
--	downhill_id INTEGER NOT NULL, -- friend_id if source_id = -1, otherwise 0 (not self) or 1 (self) 
--	timestamp INTEGER NOT NULL,
--	desired INTEGER NOT NULL,
--	PRIMARY KEY(dest_id, source_id)
--);

CREATE UNIQUE INDEX IF NOT EXISTS IDX_Friend_fingerprint ON Friend (fingerprint);
CREATE UNIQUE INDEX IF NOT EXISTS IDX_Object_seqno ON Object (seqno);
CREATE UNIQUE INDEX IF NOT EXISTS IDX_Blob ON Blob (key);
CREATE INDEX IF NOT EXISTS IDX_TopicFriend_check ON TopicFriend (friend_id, desired, requested);
CREATE INDEX IF NOT EXISTS IDX_Blob_needs_download ON Blob (needs_download);
CREATE INDEX IF NOT EXISTS IDX_Object_topic ON Object (topic, type, key, priority);
`,
			`
DROP TABLE Rendezvous;
`,
			`
--
-- Add Topic
--
CREATE TABLE Topic(
	topic_id     INTEGER PRIMARY KEY NOT NULL,
	name         TEXT NOT NULL
);
CREATE UNIQUE INDEX IDX_Topic_name ON Topic (name);

INSERT INTO Topic (name)
SELECT DISTINCT topic FROM Object
UNION
SELECT DISTINCT topic FROM TopicFriend;

--
-- Alter Object
--
CREATE TABLE T_Object(
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
INSERT INTO T_Object(
	topic_id, type, key, value, seqno, priority, author, signature
) 
SELECT 
	t.topic_id, o.type, o.key, o.value, o.seqno, o.priority, o.author, o.signature
FROM
	Object o
	JOIN Topic t ON o.topic = t.name;

DROP INDEX IDX_Object_seqno;
DROP INDEX IDX_Object_topic;
DROP TABLE Object;

ALTER TABLE T_Object RENAME TO Object;
CREATE UNIQUE INDEX IDX_Object_seqno ON Object (seqno);
CREATE INDEX        IDX_Object_ttkp ON Object (topic_id, type, key, priority);

--
-- Alter TopicFriend
--
CREATE TABLE T_TopicFriend(
	friend_id    INTEGER NOT NULL,
	topic_id     INTEGER NOT NULL,
	desired      INTEGER NOT NULL DEFAULT(0),
	requested    INTEGER NOT NULL DEFAULT(0),
	acked_seqno  INTEGER NOT NULL DEFAULT(0),
	heard_seqno  INTEGER NOT NULL DEFAULT(0),
	PRIMARY KEY(friend_id, topic_id)
);

INSERT INTO T_TopicFriend(
	friend_id, topic_id, desired, requested, acked_seqno, heard_seqno
)
SELECT
	tf.friend_id, t.topic_id, tf.desired, tf.requested, tf.acked_seqno, tf.heard_seqno
FROM
	TopicFriend tf
	JOIN Topic t ON tf.topic = t.name;

DROP INDEX IDX_TopicFriend_check
DROP TABLE TopicFriend;

ALTER TABLE T_TopicFriend RENAME TO TopicFriend;
CREATE INDEX IDX_TopicFriend_fdr ON TopicFriend (friend_id, desired, requested);

--
-- Alter Advert
--
CREATE TABLE T_Advert(
	key          TEXT NOT NULL,
	friend_id    INTEGER NOT NULL,
	topic_id     INTEGER NOT NULL,
	PRIMARY KEY(key, friend_id, topic_id)
);

INSERT INTO T_Advert(
	key, friend_id, topic_id
)
SELECT
	a.key, a.friend_id, t.topic_id
FROM
	Advert a
	JOIN Topic t ON a.topic = t.name;

DROP TABLE Advert;
ALTER TABLE T_Advert RENAME TO Advert;

--
-- Alter Friend 
--
CREATE TABLE T_Friend(
	friend_id    INTEGER PRIMARY KEY NOT NULL,
	fingerprint  BLOB NOT NULL,
	rendezvous   TEXT NOT NULL,
	public_key   BLOB NULL,
	host         TEXT NOT NULL,  -- Filled with '$' when 'empty' to deal with borked DB handling in go
	port         INTEGER NOT NULL DEFAULT (0)
);

INSERT INTO T_Friend(
	key, friend_id, topic_id
)
SELECT
	a.key, a.friend_id, t.topic_id
FROM
	Friend a
	JOIN Topic t ON a.topic = t.name;

DROP INDEX IDX_Friend_fingerprint;
DROP TABLE Friend;

ALTER TABLE T_Friend RENAME TO Friend;
CREATE UNIQUE INDEX IDX_Friend_fingerprint ON Friend (fingerprint);


--
-- Add TopicGroup
--
CREATE TABLE TopicGroup(
	topic_group_id  INTEGER NOT NULL,
	topic_id        INTEGER NOT NULL,
	name            TEXT NOT NULL,
	PRIMARY KEY(topic_group_id, topic_id)
);
CREATE UNIQUE INDEX IDX_TopicGroup_name ON TopicGroup (name);
`,
		},
	}
}
