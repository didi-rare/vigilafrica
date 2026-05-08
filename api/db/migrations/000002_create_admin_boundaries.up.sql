CREATE TABLE admin_boundaries (
    id            SERIAL           PRIMARY KEY,
    country_code  TEXT             NOT NULL,
    country_name  TEXT             NOT NULL,
    adm_level     INTEGER          NOT NULL CHECK (adm_level IN (0, 1, 2)),
    adm_name      TEXT             NOT NULL,
    geom          geometry(MultiPolygon, 4326) NOT NULL
);

-- Performance indexes
CREATE INDEX idx_boundaries_country    ON admin_boundaries(country_code);
CREATE INDEX idx_boundaries_adm_level  ON admin_boundaries(adm_level);
CREATE INDEX idx_boundaries_adm_name   ON admin_boundaries(adm_name);
CREATE INDEX idx_boundaries_geom       ON admin_boundaries USING GIST(geom);
