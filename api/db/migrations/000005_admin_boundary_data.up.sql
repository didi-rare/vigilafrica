-- Admin Boundary Data — Nigeria + Ghana
-- Fixes pre-existing gap: admin_boundaries table was created in 000002 but never seeded.
--
-- IMPORTANT: These are SIMPLIFIED rectangular boundary approximations for development
-- and prototype use. They correctly classify events that are clearly within a state
-- but will be imprecise near borders.
--
-- For production accuracy, replace with official HDX COD Admin Boundaries:
--   Nigeria: https://data.humdata.org/dataset/cod-ab-nga
--   Ghana:   https://data.humdata.org/dataset/cod-ab-gha
-- See: openspec/specs/vigilafrica/country-onboarding-template.md §1 for the full
-- process to generate production-quality migration SQL from HDX GeoJSON files.
--
-- All geometries use EPSG:4326 (WGS84). Format: MULTIPOLYGON(((lon lat, ...)))
-- Rectangle convention: SW → SE → NE → NW → SW (clockwise exterior ring).

-- ─── NIGERIA ─────────────────────────────────────────────────────────────────

DO $$ BEGIN
  IF NOT EXISTS (SELECT 1 FROM admin_boundaries WHERE country_code = 'NG') THEN

    -- ADM0: Nigeria national boundary (simplified)
    INSERT INTO admin_boundaries (country_code, country_name, adm_level, adm_name, geom) VALUES
    ('NG', 'Nigeria', 0, 'Nigeria',
     ST_GeomFromText('MULTIPOLYGON(((2.0 4.0, 15.1 4.0, 15.1 14.0, 2.0 14.0, 2.0 4.0)))', 4326));

    -- ADM1: 36 states + FCT (simplified bounding-box approximations)
    INSERT INTO admin_boundaries (country_code, country_name, adm_level, adm_name, geom) VALUES
    ('NG', 'Nigeria', 1, 'Lagos',
     ST_GeomFromText('MULTIPOLYGON(((3.1 6.3, 3.9 6.3, 3.9 6.7, 3.1 6.7, 3.1 6.3)))', 4326)),
    ('NG', 'Nigeria', 1, 'Ogun',
     ST_GeomFromText('MULTIPOLYGON(((2.8 6.7, 4.0 6.7, 4.0 7.8, 2.8 7.8, 2.8 6.7)))', 4326)),
    ('NG', 'Nigeria', 1, 'Oyo',
     ST_GeomFromText('MULTIPOLYGON(((3.0 7.3, 5.0 7.3, 5.0 9.2, 3.0 9.2, 3.0 7.3)))', 4326)),
    ('NG', 'Nigeria', 1, 'Osun',
     ST_GeomFromText('MULTIPOLYGON(((4.2 7.1, 5.0 7.1, 5.0 8.0, 4.2 8.0, 4.2 7.1)))', 4326)),
    ('NG', 'Nigeria', 1, 'Ondo',
     ST_GeomFromText('MULTIPOLYGON(((4.4 5.8, 6.1 5.8, 6.1 8.0, 4.4 8.0, 4.4 5.8)))', 4326)),
    ('NG', 'Nigeria', 1, 'Ekiti',
     ST_GeomFromText('MULTIPOLYGON(((4.9 7.4, 5.9 7.4, 5.9 8.1, 4.9 8.1, 4.9 7.4)))', 4326)),
    ('NG', 'Nigeria', 1, 'Edo',
     ST_GeomFromText('MULTIPOLYGON(((5.0 5.7, 6.8 5.7, 6.8 7.5, 5.0 7.5, 5.0 5.7)))', 4326)),
    ('NG', 'Nigeria', 1, 'Delta',
     ST_GeomFromText('MULTIPOLYGON(((5.1 4.9, 7.0 4.9, 7.0 6.6, 5.1 6.6, 5.1 4.9)))', 4326)),
    ('NG', 'Nigeria', 1, 'Rivers',
     ST_GeomFromText('MULTIPOLYGON(((6.5 4.5, 7.8 4.5, 7.8 5.8, 6.5 5.8, 6.5 4.5)))', 4326)),
    ('NG', 'Nigeria', 1, 'Bayelsa',
     ST_GeomFromText('MULTIPOLYGON(((5.7 4.4, 6.9 4.4, 6.9 5.3, 5.7 5.3, 5.7 4.4)))', 4326)),
    ('NG', 'Nigeria', 1, 'Cross River',
     ST_GeomFromText('MULTIPOLYGON(((7.7 4.4, 9.6 4.4, 9.6 7.0, 7.7 7.0, 7.7 4.4)))', 4326)),
    ('NG', 'Nigeria', 1, 'Akwa Ibom',
     ST_GeomFromText('MULTIPOLYGON(((7.3 4.4, 8.6 4.4, 8.6 5.5, 7.3 5.5, 7.3 4.4)))', 4326)),
    ('NG', 'Nigeria', 1, 'Anambra',
     ST_GeomFromText('MULTIPOLYGON(((6.5 5.7, 7.3 5.7, 7.3 6.7, 6.5 6.7, 6.5 5.7)))', 4326)),
    ('NG', 'Nigeria', 1, 'Imo',
     ST_GeomFromText('MULTIPOLYGON(((6.8 5.0, 7.7 5.0, 7.7 6.0, 6.8 6.0, 6.8 5.0)))', 4326)),
    ('NG', 'Nigeria', 1, 'Abia',
     ST_GeomFromText('MULTIPOLYGON(((7.1 4.8, 8.2 4.8, 8.2 6.1, 7.1 6.1, 7.1 4.8)))', 4326)),
    ('NG', 'Nigeria', 1, 'Ebonyi',
     ST_GeomFromText('MULTIPOLYGON(((7.8 5.7, 8.7 5.7, 8.7 6.8, 7.8 6.8, 7.8 5.7)))', 4326)),
    ('NG', 'Nigeria', 1, 'Enugu',
     ST_GeomFromText('MULTIPOLYGON(((7.0 6.0, 8.1 6.0, 8.1 7.3, 7.0 7.3, 7.0 6.0)))', 4326)),
    ('NG', 'Nigeria', 1, 'Kogi',
     ST_GeomFromText('MULTIPOLYGON(((5.9 7.0, 7.8 7.0, 7.8 8.8, 5.9 8.8, 5.9 7.0)))', 4326)),
    ('NG', 'Nigeria', 1, 'Benue',
     ST_GeomFromText('MULTIPOLYGON(((7.6 6.4, 10.1 6.4, 10.1 8.4, 7.6 8.4, 7.6 6.4)))', 4326)),
    ('NG', 'Nigeria', 1, 'Kwara',
     ST_GeomFromText('MULTIPOLYGON(((3.7 7.9, 6.3 7.9, 6.3 9.8, 3.7 9.8, 3.7 7.9)))', 4326)),
    ('NG', 'Nigeria', 1, 'Niger',
     ST_GeomFromText('MULTIPOLYGON(((3.6 8.2, 7.4 8.2, 7.4 11.7, 3.6 11.7, 3.6 8.2)))', 4326)),
    ('NG', 'Nigeria', 1, 'FCT',
     ST_GeomFromText('MULTIPOLYGON(((6.7 8.4, 7.6 8.4, 7.6 9.4, 6.7 9.4, 6.7 8.4)))', 4326)),
    ('NG', 'Nigeria', 1, 'Nasarawa',
     ST_GeomFromText('MULTIPOLYGON(((7.1 7.7, 9.4 7.7, 9.4 9.4, 7.1 9.4, 7.1 7.7)))', 4326)),
    ('NG', 'Nigeria', 1, 'Plateau',
     ST_GeomFromText('MULTIPOLYGON(((8.2 8.1, 10.6 8.1, 10.6 10.6, 8.2 10.6, 8.2 8.1)))', 4326)),
    ('NG', 'Nigeria', 1, 'Taraba',
     ST_GeomFromText('MULTIPOLYGON(((9.6 6.4, 12.6 6.4, 12.6 9.1, 9.6 9.1, 9.6 6.4)))', 4326)),
    ('NG', 'Nigeria', 1, 'Adamawa',
     ST_GeomFromText('MULTIPOLYGON(((11.4 7.4, 14.6 7.4, 14.6 11.1, 11.4 11.1, 11.4 7.4)))', 4326)),
    ('NG', 'Nigeria', 1, 'Bauchi',
     ST_GeomFromText('MULTIPOLYGON(((9.2 9.4, 11.6 9.4, 11.6 12.4, 9.2 12.4, 9.2 9.4)))', 4326)),
    ('NG', 'Nigeria', 1, 'Gombe',
     ST_GeomFromText('MULTIPOLYGON(((10.5 9.4, 12.1 9.4, 12.1 11.6, 10.5 11.6, 10.5 9.4)))', 4326)),
    ('NG', 'Nigeria', 1, 'Yobe',
     ST_GeomFromText('MULTIPOLYGON(((10.9 11.1, 14.6 11.1, 14.6 14.0, 10.9 14.0, 10.9 11.1)))', 4326)),
    ('NG', 'Nigeria', 1, 'Borno',
     ST_GeomFromText('MULTIPOLYGON(((11.4 9.9, 15.1 9.9, 15.1 14.0, 11.4 14.0, 11.4 9.9)))', 4326)),
    ('NG', 'Nigeria', 1, 'Sokoto',
     ST_GeomFromText('MULTIPOLYGON(((4.0 11.9, 6.6 11.9, 6.6 14.0, 4.0 14.0, 4.0 11.9)))', 4326)),
    ('NG', 'Nigeria', 1, 'Kebbi',
     ST_GeomFromText('MULTIPOLYGON(((3.7 10.4, 6.0 10.4, 6.0 13.1, 3.7 13.1, 3.7 10.4)))', 4326)),
    ('NG', 'Nigeria', 1, 'Zamfara',
     ST_GeomFromText('MULTIPOLYGON(((5.4 11.0, 8.4 11.0, 8.4 13.1, 5.4 13.1, 5.4 11.0)))', 4326)),
    ('NG', 'Nigeria', 1, 'Katsina',
     ST_GeomFromText('MULTIPOLYGON(((6.9 11.1, 9.3 11.1, 9.3 13.5, 6.9 13.5, 6.9 11.1)))', 4326)),
    ('NG', 'Nigeria', 1, 'Kano',
     ST_GeomFromText('MULTIPOLYGON(((7.4 10.4, 9.9 10.4, 9.9 12.6, 7.4 12.6, 7.4 10.4)))', 4326)),
    ('NG', 'Nigeria', 1, 'Jigawa',
     ST_GeomFromText('MULTIPOLYGON(((8.9 11.3, 10.6 11.3, 10.6 13.2, 8.9 13.2, 8.9 11.3)))', 4326)),
    ('NG', 'Nigeria', 1, 'Kaduna',
     ST_GeomFromText('MULTIPOLYGON(((6.6 8.9, 9.3 8.9, 9.3 11.7, 6.6 11.7, 6.6 8.9)))', 4326));

  END IF;
END $$;

-- ─── GHANA ───────────────────────────────────────────────────────────────────

DO $$ BEGIN
  IF NOT EXISTS (SELECT 1 FROM admin_boundaries WHERE country_code = 'GH') THEN

    -- ADM0: Ghana national boundary (simplified)
    INSERT INTO admin_boundaries (country_code, country_name, adm_level, adm_name, geom) VALUES
    ('GH', 'Ghana', 0, 'Ghana',
     ST_GeomFromText('MULTIPOLYGON(((-3.3 4.7, 1.2 4.7, 1.2 11.2, -3.3 11.2, -3.3 4.7)))', 4326));

    -- ADM1: 16 regions — simplified bounding-box approximations (post-2019 reorganisation)
    -- Replace with HDX COD data for production accuracy:
    --   https://data.humdata.org/dataset/cod-ab-gha
    INSERT INTO admin_boundaries (country_code, country_name, adm_level, adm_name, geom) VALUES
    ('GH', 'Ghana', 1, 'Greater Accra',
     ST_GeomFromText('MULTIPOLYGON(((-0.4 5.5, 0.5 5.5, 0.5 6.1, -0.4 6.1, -0.4 5.5)))', 4326)),
    ('GH', 'Ghana', 1, 'Central',
     ST_GeomFromText('MULTIPOLYGON(((-1.8 4.9, -0.1 4.9, -0.1 6.3, -1.8 6.3, -1.8 4.9)))', 4326)),
    ('GH', 'Ghana', 1, 'Western',
     ST_GeomFromText('MULTIPOLYGON(((-3.3 4.7, -1.7 4.7, -1.7 6.3, -3.3 6.3, -3.3 4.7)))', 4326)),
    ('GH', 'Ghana', 1, 'Western North',
     ST_GeomFromText('MULTIPOLYGON(((-3.2 6.2, -2.0 6.2, -2.0 8.1, -3.2 8.1, -3.2 6.2)))', 4326)),
    ('GH', 'Ghana', 1, 'Eastern',
     ST_GeomFromText('MULTIPOLYGON(((-0.8 6.0, 0.5 6.0, 0.5 7.5, -0.8 7.5, -0.8 6.0)))', 4326)),
    ('GH', 'Ghana', 1, 'Ashanti',
     ST_GeomFromText('MULTIPOLYGON(((-2.5 6.3, -0.7 6.3, -0.7 7.5, -2.5 7.5, -2.5 6.3)))', 4326)),
    ('GH', 'Ghana', 1, 'Ahafo',
     ST_GeomFromText('MULTIPOLYGON(((-2.9 6.5, -1.9 6.5, -1.9 7.8, -2.9 7.8, -2.9 6.5)))', 4326)),
    ('GH', 'Ghana', 1, 'Bono',
     ST_GeomFromText('MULTIPOLYGON(((-2.9 7.7, -1.5 7.7, -1.5 8.7, -2.9 8.7, -2.9 7.7)))', 4326)),
    ('GH', 'Ghana', 1, 'Bono East',
     ST_GeomFromText('MULTIPOLYGON(((-1.5 7.5, -0.2 7.5, -0.2 9.1, -1.5 9.1, -1.5 7.5)))', 4326)),
    ('GH', 'Ghana', 1, 'Volta',
     ST_GeomFromText('MULTIPOLYGON(((-0.2 5.8, 0.9 5.8, 0.9 8.8, -0.2 8.8, -0.2 5.8)))', 4326)),
    ('GH', 'Ghana', 1, 'Oti',
     ST_GeomFromText('MULTIPOLYGON(((-0.1 8.7, 0.8 8.7, 0.8 10.9, -0.1 10.9, -0.1 8.7)))', 4326)),
    ('GH', 'Ghana', 1, 'Northern',
     ST_GeomFromText('MULTIPOLYGON(((-2.0 9.0, 0.0 9.0, 0.0 10.7, -2.0 10.7, -2.0 9.0)))', 4326)),
    ('GH', 'Ghana', 1, 'Savannah',
     ST_GeomFromText('MULTIPOLYGON(((-2.6 8.6, -1.0 8.6, -1.0 11.2, -2.6 11.2, -2.6 8.6)))', 4326)),
    ('GH', 'Ghana', 1, 'North East',
     ST_GeomFromText('MULTIPOLYGON(((-0.3 9.5, 0.8 9.5, 0.8 10.9, -0.3 10.9, -0.3 9.5)))', 4326)),
    ('GH', 'Ghana', 1, 'Upper East',
     ST_GeomFromText('MULTIPOLYGON(((-0.6 10.5, 1.1 10.5, 1.1 11.2, -0.6 11.2, -0.6 10.5)))', 4326)),
    ('GH', 'Ghana', 1, 'Upper West',
     ST_GeomFromText('MULTIPOLYGON(((-2.9 9.8, -1.5 9.8, -1.5 11.2, -2.9 11.2, -2.9 9.8)))', 4326));

  END IF;
END $$;
