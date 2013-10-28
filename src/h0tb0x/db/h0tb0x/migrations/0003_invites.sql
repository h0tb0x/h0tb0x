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
