PRAGMA user_version = 1;

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
