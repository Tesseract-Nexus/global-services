package seeder

import (
	"location-service/internal/models"
)

func getTimezonesData() []models.Timezone {
	return []models.Timezone{
		// Americas
		{ID: "America/New_York", Name: "Eastern Time", Abbreviation: "EST/EDT", Offset: "-05:00", DST: true, Countries: `["US","CA"]`},
		{ID: "America/Chicago", Name: "Central Time", Abbreviation: "CST/CDT", Offset: "-06:00", DST: true, Countries: `["US","CA","MX"]`},
		{ID: "America/Denver", Name: "Mountain Time", Abbreviation: "MST/MDT", Offset: "-07:00", DST: true, Countries: `["US","CA","MX"]`},
		{ID: "America/Los_Angeles", Name: "Pacific Time", Abbreviation: "PST/PDT", Offset: "-08:00", DST: true, Countries: `["US","CA"]`},
		{ID: "America/Anchorage", Name: "Alaska Time", Abbreviation: "AKST/AKDT", Offset: "-09:00", DST: true, Countries: `["US"]`},
		{ID: "Pacific/Honolulu", Name: "Hawaii-Aleutian Time", Abbreviation: "HST", Offset: "-10:00", DST: false, Countries: `["US"]`},
		{ID: "America/Toronto", Name: "Toronto Time", Abbreviation: "EST/EDT", Offset: "-05:00", DST: true, Countries: `["CA"]`},
		{ID: "America/Vancouver", Name: "Vancouver Time", Abbreviation: "PST/PDT", Offset: "-08:00", DST: true, Countries: `["CA"]`},
		{ID: "America/Sao_Paulo", Name: "Brasília Time", Abbreviation: "BRT/BRST", Offset: "-03:00", DST: true, Countries: `["BR"]`},
		{ID: "America/Mexico_City", Name: "Central Standard Time", Abbreviation: "CST", Offset: "-06:00", DST: false, Countries: `["MX"]`},

		// Europe
		{ID: "Europe/London", Name: "Greenwich Mean Time", Abbreviation: "GMT/BST", Offset: "+00:00", DST: true, Countries: `["GB"]`},
		{ID: "Europe/Paris", Name: "Central European Time", Abbreviation: "CET/CEST", Offset: "+01:00", DST: true, Countries: `["FR","DE","IT","ES","NL"]`},
		{ID: "Europe/Berlin", Name: "Central European Time", Abbreviation: "CET/CEST", Offset: "+01:00", DST: true, Countries: `["DE"]`},
		{ID: "Europe/Madrid", Name: "Central European Time", Abbreviation: "CET/CEST", Offset: "+01:00", DST: true, Countries: `["ES"]`},
		{ID: "Europe/Rome", Name: "Central European Time", Abbreviation: "CET/CEST", Offset: "+01:00", DST: true, Countries: `["IT"]`},
		{ID: "Europe/Amsterdam", Name: "Central European Time", Abbreviation: "CET/CEST", Offset: "+01:00", DST: true, Countries: `["NL"]`},
		{ID: "Europe/Stockholm", Name: "Central European Time", Abbreviation: "CET/CEST", Offset: "+01:00", DST: true, Countries: `["SE"]`},

		// Asia
		{ID: "Asia/Dubai", Name: "Gulf Standard Time", Abbreviation: "GST", Offset: "+04:00", DST: false, Countries: `["AE"]`},
		{ID: "Asia/Kolkata", Name: "India Standard Time", Abbreviation: "IST", Offset: "+05:30", DST: false, Countries: `["IN"]`},
		{ID: "Asia/Singapore", Name: "Singapore Time", Abbreviation: "SGT", Offset: "+08:00", DST: false, Countries: `["SG"]`},
		{ID: "Asia/Tokyo", Name: "Japan Standard Time", Abbreviation: "JST", Offset: "+09:00", DST: false, Countries: `["JP"]`},
		{ID: "Asia/Seoul", Name: "Korea Standard Time", Abbreviation: "KST", Offset: "+09:00", DST: false, Countries: `["KR"]`},
		{ID: "Asia/Shanghai", Name: "China Standard Time", Abbreviation: "CST", Offset: "+08:00", DST: false, Countries: `["CN"]`},

		// Oceania
		{ID: "Australia/Sydney", Name: "Australian Eastern Time", Abbreviation: "AEST/AEDT", Offset: "+10:00", DST: true, Countries: `["AU"]`},
		{ID: "Australia/Melbourne", Name: "Australian Eastern Time", Abbreviation: "AEST/AEDT", Offset: "+10:00", DST: true, Countries: `["AU"]`},
		{ID: "Australia/Brisbane", Name: "Australian Eastern Standard Time", Abbreviation: "AEST", Offset: "+10:00", DST: false, Countries: `["AU"]`},
		{ID: "Australia/Perth", Name: "Australian Western Standard Time", Abbreviation: "AWST", Offset: "+08:00", DST: false, Countries: `["AU"]`},
		{ID: "Pacific/Auckland", Name: "New Zealand Time", Abbreviation: "NZST/NZDT", Offset: "+12:00", DST: true, Countries: `["NZ"]`},

		// Africa
		{ID: "Africa/Johannesburg", Name: "South Africa Standard Time", Abbreviation: "SAST", Offset: "+02:00", DST: false, Countries: `["ZA"]`},
	}
}
