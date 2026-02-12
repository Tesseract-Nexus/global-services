-- Migration 007: Rebrand email templates from Tesserix to mark8ly
-- Updates all existing notification_templates in the DB

-- Update HTML templates: replace branding text and links
UPDATE notification_templates
SET html_template = REPLACE(
      REPLACE(
        REPLACE(html_template, 'Tesserix', 'mark8ly'),
        'tesserix.app', 'mark8ly.com'
      ),
      'Tesseract Hub', 'mark8ly'
    ),
    body_template = REPLACE(
      REPLACE(
        REPLACE(body_template, 'Tesserix', 'mark8ly'),
        'tesserix.app', 'mark8ly.com'
      ),
      'Tesseract Hub', 'mark8ly'
    ),
    updated_at = NOW()
WHERE html_template LIKE '%Tesserix%'
   OR html_template LIKE '%tesserix.app%'
   OR html_template LIKE '%Tesseract Hub%'
   OR body_template LIKE '%Tesserix%'
   OR body_template LIKE '%tesserix.app%'
   OR body_template LIKE '%Tesseract Hub%';

-- Update subject lines too
UPDATE notification_templates
SET subject = REPLACE(
      REPLACE(
        REPLACE(subject, 'Tesserix', 'mark8ly'),
        'tesserix.app', 'mark8ly.com'
      ),
      'Tesseract Hub', 'mark8ly'
    ),
    updated_at = NOW()
WHERE subject LIKE '%Tesserix%'
   OR subject LIKE '%tesserix.app%'
   OR subject LIKE '%Tesseract Hub%';
