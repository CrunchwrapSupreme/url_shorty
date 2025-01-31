CREATE TABLE tenants (
       id INTEGER PRIMARY KEY ASC,
       account_name VARCHAR(12) UNIQUE
);

CREATE TABLE url_mappings (
       id        INTEGER PRIMARY KEY ASC,
       short_slug CHAR(20) UNIQUE NOT NULL CHECK(length(short_slug) > 0),
       long_url  VARCHAR(255) NOT NULL CHECK(length(long_url) > 0),
       protocol  VARCHAR(5) DEFAULT 'https',
       owner_id  INTEGER,
       FOREIGN KEY(owner_id) REFERENCES tenants(id)
);

CREATE UNIQUE INDEX idx_mapped_short_slug ON url_mappings(short_slug);
CREATE INDEX idx_mapped_long_url ON url_mappings(long_url);
