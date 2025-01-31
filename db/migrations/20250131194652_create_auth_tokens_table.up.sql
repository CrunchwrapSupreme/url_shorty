CREATE TABLE auth_tokens (
       id INTEGER PRIMARY KEY ASC,
       token VARCHAR(255),
       owner_id INTEGER DEFAULT NULL,
       FOREIGN KEY(owner_id) REFERENCES tenants(id)
);
