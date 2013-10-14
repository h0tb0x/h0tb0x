DROP TABLE Object;
DROP TABLE Friend;
DROP TABLE TopicFriend;
DROP TABLE Blob;
DROP TABLE Advert;

CREATE UNIQUE INDEX IDX_Rendezvous_fp ON Rendezvous (fingerprint);
