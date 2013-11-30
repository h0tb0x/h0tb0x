-- rename Object -> Record
ALTER TABLE Object RENAME TO Record;

-- rename indices:
--     IDX_Object_seqno        -> IDX_Record_01
DROP INDEX IDX_Object_seqno;
CREATE UNIQUE INDEX IDX_Record_01 ON Record (seqno);
--     IDX_Object_topic        -> IDX_Record_02
DROP INDEX IDX_Object_topic;
CREATE INDEX        IDX_Record_02 ON Record (topic, type, key, priority);
--     IDX_Friend_fingerprint  -> IDX_Friend_01
DROP INDEX IDX_Friend_fingerprint;
CREATE UNIQUE INDEX IDX_Friend_01 ON Friend (fingerprint);
--     IDX_TopicFriend_check   -> IDX_TopicFriend_01
DROP INDEX IDX_TopicFriend_check;
CREATE INDEX        IDX_TopicFriend_01 ON TopicFriend (friend_id, desired, requested);
--     IDX_Blob                -> IDX_Blob_01
DROP INDEX IDX_Blob;
CREATE UNIQUE INDEX IDX_Blob_01 ON Blob (key);
--     IDX_Blob_needs_download -> IDX_Blob_02
DROP INDEX IDX_Blob_needs_download;
CREATE INDEX        IDX_Blob_02 ON Blob (needs_download);

-- add Attribute
-- add Attribute indices
