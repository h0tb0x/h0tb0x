CREATE TABLE Storage(
        key TEXT NOT NULL, -- The hash of the storage
        data BLOB NOT NULL,  -- Info about who has what
        needs_req BOOL NOT NULL, -- Advert data needs a request
        PRIMARY KEY(key)
);

CREATE INDEX IDX_Storage_needs_req ON Storage (needs_req);

