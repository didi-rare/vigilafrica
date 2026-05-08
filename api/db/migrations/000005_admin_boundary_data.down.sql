-- Delete all admin boundaries for Nigeria and Ghana (development prototypes)
DELETE FROM admin_boundaries WHERE country_code IN ('NG', 'GH');
