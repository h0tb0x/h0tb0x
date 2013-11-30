-- TODO: make this information private to friends via public key crypto
CREATE TABLE Rendezvous(
	-- Not in sig
	fingerprint     TEXT    NOT NULL PRIMARY KEY,
	public_key      TEXT    NOT NULL,
	-- In sig
	version         INTEGER NOT NULL,
	host            TEXT    NOT NULL,
	port            INTEGER NOT NULL,
	-- The sig
	signature       TEXT    NOT NULL
);

CREATE UNIQUE INDEX IDX_Rendezvous_fp ON Rendezvous (fingerprint);
