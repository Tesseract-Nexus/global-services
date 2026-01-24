package seeder

import (
	"location-service/internal/models"
)

func getTimezonesData() []models.Timezone {
	return []models.Timezone{
		// ========== Americas ==========
		// United States
		{ID: "America/New_York", Name: "Eastern Time", Abbreviation: "EST/EDT", Offset: "-05:00", DST: true, Countries: `["US"]`},
		{ID: "America/Chicago", Name: "Central Time", Abbreviation: "CST/CDT", Offset: "-06:00", DST: true, Countries: `["US"]`},
		{ID: "America/Denver", Name: "Mountain Time", Abbreviation: "MST/MDT", Offset: "-07:00", DST: true, Countries: `["US"]`},
		{ID: "America/Phoenix", Name: "Arizona Time", Abbreviation: "MST", Offset: "-07:00", DST: false, Countries: `["US"]`},
		{ID: "America/Los_Angeles", Name: "Pacific Time", Abbreviation: "PST/PDT", Offset: "-08:00", DST: true, Countries: `["US"]`},
		{ID: "America/Anchorage", Name: "Alaska Time", Abbreviation: "AKST/AKDT", Offset: "-09:00", DST: true, Countries: `["US"]`},
		{ID: "Pacific/Honolulu", Name: "Hawaii-Aleutian Time", Abbreviation: "HST", Offset: "-10:00", DST: false, Countries: `["US"]`},

		// Canada
		{ID: "America/Toronto", Name: "Toronto Time", Abbreviation: "EST/EDT", Offset: "-05:00", DST: true, Countries: `["CA"]`},
		{ID: "America/Vancouver", Name: "Vancouver Time", Abbreviation: "PST/PDT", Offset: "-08:00", DST: true, Countries: `["CA"]`},
		{ID: "America/Edmonton", Name: "Edmonton Time", Abbreviation: "MST/MDT", Offset: "-07:00", DST: true, Countries: `["CA"]`},
		{ID: "America/Winnipeg", Name: "Winnipeg Time", Abbreviation: "CST/CDT", Offset: "-06:00", DST: true, Countries: `["CA"]`},
		{ID: "America/Halifax", Name: "Atlantic Time", Abbreviation: "AST/ADT", Offset: "-04:00", DST: true, Countries: `["CA"]`},
		{ID: "America/St_Johns", Name: "Newfoundland Time", Abbreviation: "NST/NDT", Offset: "-03:30", DST: true, Countries: `["CA"]`},

		// Mexico
		{ID: "America/Mexico_City", Name: "Central Mexico Time", Abbreviation: "CST", Offset: "-06:00", DST: false, Countries: `["MX"]`},
		{ID: "America/Tijuana", Name: "Pacific Mexico Time", Abbreviation: "PST/PDT", Offset: "-08:00", DST: true, Countries: `["MX"]`},
		{ID: "America/Cancun", Name: "Eastern Mexico Time", Abbreviation: "EST", Offset: "-05:00", DST: false, Countries: `["MX"]`},

		// South America
		{ID: "America/Sao_Paulo", Name: "Brasília Time", Abbreviation: "BRT", Offset: "-03:00", DST: false, Countries: `["BR"]`},
		{ID: "America/Manaus", Name: "Amazon Time", Abbreviation: "AMT", Offset: "-04:00", DST: false, Countries: `["BR"]`},
		{ID: "America/Argentina/Buenos_Aires", Name: "Argentina Time", Abbreviation: "ART", Offset: "-03:00", DST: false, Countries: `["AR"]`},
		{ID: "America/Bogota", Name: "Colombia Time", Abbreviation: "COT", Offset: "-05:00", DST: false, Countries: `["CO"]`},
		{ID: "America/Lima", Name: "Peru Time", Abbreviation: "PET", Offset: "-05:00", DST: false, Countries: `["PE"]`},
		{ID: "America/Santiago", Name: "Chile Time", Abbreviation: "CLT/CLST", Offset: "-04:00", DST: true, Countries: `["CL"]`},
		{ID: "America/Caracas", Name: "Venezuela Time", Abbreviation: "VET", Offset: "-04:00", DST: false, Countries: `["VE"]`},
		{ID: "America/Guayaquil", Name: "Ecuador Time", Abbreviation: "ECT", Offset: "-05:00", DST: false, Countries: `["EC"]`},
		{ID: "America/La_Paz", Name: "Bolivia Time", Abbreviation: "BOT", Offset: "-04:00", DST: false, Countries: `["BO"]`},
		{ID: "America/Montevideo", Name: "Uruguay Time", Abbreviation: "UYT", Offset: "-03:00", DST: false, Countries: `["UY"]`},
		{ID: "America/Asuncion", Name: "Paraguay Time", Abbreviation: "PYT/PYST", Offset: "-04:00", DST: true, Countries: `["PY"]`},

		// Central America & Caribbean
		{ID: "America/Panama", Name: "Panama Time", Abbreviation: "EST", Offset: "-05:00", DST: false, Countries: `["PA","CR"]`},
		{ID: "America/Guatemala", Name: "Guatemala Time", Abbreviation: "CST", Offset: "-06:00", DST: false, Countries: `["GT","HN","SV","NI"]`},
		{ID: "America/Havana", Name: "Cuba Time", Abbreviation: "CST/CDT", Offset: "-05:00", DST: true, Countries: `["CU"]`},
		{ID: "America/Puerto_Rico", Name: "Atlantic Time", Abbreviation: "AST", Offset: "-04:00", DST: false, Countries: `["PR","VG","VI"]`},
		{ID: "America/Jamaica", Name: "Jamaica Time", Abbreviation: "EST", Offset: "-05:00", DST: false, Countries: `["JM"]`},

		// ========== Europe ==========
		// Western Europe
		{ID: "Europe/London", Name: "Greenwich Mean Time", Abbreviation: "GMT/BST", Offset: "+00:00", DST: true, Countries: `["GB","IE"]`},
		{ID: "Europe/Dublin", Name: "Irish Time", Abbreviation: "GMT/IST", Offset: "+00:00", DST: true, Countries: `["IE"]`},
		{ID: "Europe/Lisbon", Name: "Western European Time", Abbreviation: "WET/WEST", Offset: "+00:00", DST: true, Countries: `["PT"]`},

		// Central Europe
		{ID: "Europe/Paris", Name: "Central European Time", Abbreviation: "CET/CEST", Offset: "+01:00", DST: true, Countries: `["FR"]`},
		{ID: "Europe/Berlin", Name: "Central European Time", Abbreviation: "CET/CEST", Offset: "+01:00", DST: true, Countries: `["DE"]`},
		{ID: "Europe/Madrid", Name: "Central European Time", Abbreviation: "CET/CEST", Offset: "+01:00", DST: true, Countries: `["ES"]`},
		{ID: "Europe/Rome", Name: "Central European Time", Abbreviation: "CET/CEST", Offset: "+01:00", DST: true, Countries: `["IT"]`},
		{ID: "Europe/Amsterdam", Name: "Central European Time", Abbreviation: "CET/CEST", Offset: "+01:00", DST: true, Countries: `["NL"]`},
		{ID: "Europe/Brussels", Name: "Central European Time", Abbreviation: "CET/CEST", Offset: "+01:00", DST: true, Countries: `["BE"]`},
		{ID: "Europe/Zurich", Name: "Central European Time", Abbreviation: "CET/CEST", Offset: "+01:00", DST: true, Countries: `["CH"]`},
		{ID: "Europe/Vienna", Name: "Central European Time", Abbreviation: "CET/CEST", Offset: "+01:00", DST: true, Countries: `["AT"]`},
		{ID: "Europe/Stockholm", Name: "Central European Time", Abbreviation: "CET/CEST", Offset: "+01:00", DST: true, Countries: `["SE"]`},
		{ID: "Europe/Oslo", Name: "Central European Time", Abbreviation: "CET/CEST", Offset: "+01:00", DST: true, Countries: `["NO"]`},
		{ID: "Europe/Copenhagen", Name: "Central European Time", Abbreviation: "CET/CEST", Offset: "+01:00", DST: true, Countries: `["DK"]`},
		{ID: "Europe/Warsaw", Name: "Central European Time", Abbreviation: "CET/CEST", Offset: "+01:00", DST: true, Countries: `["PL"]`},
		{ID: "Europe/Prague", Name: "Central European Time", Abbreviation: "CET/CEST", Offset: "+01:00", DST: true, Countries: `["CZ"]`},
		{ID: "Europe/Budapest", Name: "Central European Time", Abbreviation: "CET/CEST", Offset: "+01:00", DST: true, Countries: `["HU"]`},

		// Eastern Europe
		{ID: "Europe/Helsinki", Name: "Eastern European Time", Abbreviation: "EET/EEST", Offset: "+02:00", DST: true, Countries: `["FI"]`},
		{ID: "Europe/Athens", Name: "Eastern European Time", Abbreviation: "EET/EEST", Offset: "+02:00", DST: true, Countries: `["GR"]`},
		{ID: "Europe/Bucharest", Name: "Eastern European Time", Abbreviation: "EET/EEST", Offset: "+02:00", DST: true, Countries: `["RO"]`},
		{ID: "Europe/Sofia", Name: "Eastern European Time", Abbreviation: "EET/EEST", Offset: "+02:00", DST: true, Countries: `["BG"]`},
		{ID: "Europe/Kiev", Name: "Eastern European Time", Abbreviation: "EET/EEST", Offset: "+02:00", DST: true, Countries: `["UA"]`},
		{ID: "Europe/Moscow", Name: "Moscow Time", Abbreviation: "MSK", Offset: "+03:00", DST: false, Countries: `["RU"]`},
		{ID: "Europe/Istanbul", Name: "Turkey Time", Abbreviation: "TRT", Offset: "+03:00", DST: false, Countries: `["TR"]`},

		// ========== Asia ==========
		// Middle East
		{ID: "Asia/Dubai", Name: "Gulf Standard Time", Abbreviation: "GST", Offset: "+04:00", DST: false, Countries: `["AE","OM"]`},
		{ID: "Asia/Riyadh", Name: "Arabia Standard Time", Abbreviation: "AST", Offset: "+03:00", DST: false, Countries: `["SA","KW","QA","BH"]`},
		{ID: "Asia/Jerusalem", Name: "Israel Time", Abbreviation: "IST/IDT", Offset: "+02:00", DST: true, Countries: `["IL"]`},
		{ID: "Asia/Tehran", Name: "Iran Time", Abbreviation: "IRST/IRDT", Offset: "+03:30", DST: true, Countries: `["IR"]`},
		{ID: "Asia/Baghdad", Name: "Arabia Standard Time", Abbreviation: "AST", Offset: "+03:00", DST: false, Countries: `["IQ"]`},
		{ID: "Asia/Amman", Name: "Eastern European Time", Abbreviation: "EET/EEST", Offset: "+02:00", DST: true, Countries: `["JO"]`},
		{ID: "Asia/Beirut", Name: "Eastern European Time", Abbreviation: "EET/EEST", Offset: "+02:00", DST: true, Countries: `["LB"]`},

		// South Asia
		{ID: "Asia/Kolkata", Name: "India Standard Time", Abbreviation: "IST", Offset: "+05:30", DST: false, Countries: `["IN"]`},
		{ID: "Asia/Kathmandu", Name: "Nepal Time", Abbreviation: "NPT", Offset: "+05:45", DST: false, Countries: `["NP"]`},
		{ID: "Asia/Dhaka", Name: "Bangladesh Standard Time", Abbreviation: "BST", Offset: "+06:00", DST: false, Countries: `["BD"]`},
		{ID: "Asia/Colombo", Name: "Sri Lanka Standard Time", Abbreviation: "SLST", Offset: "+05:30", DST: false, Countries: `["LK"]`},
		{ID: "Asia/Karachi", Name: "Pakistan Standard Time", Abbreviation: "PKT", Offset: "+05:00", DST: false, Countries: `["PK"]`},

		// Southeast Asia
		{ID: "Asia/Singapore", Name: "Singapore Time", Abbreviation: "SGT", Offset: "+08:00", DST: false, Countries: `["SG"]`},
		{ID: "Asia/Kuala_Lumpur", Name: "Malaysia Time", Abbreviation: "MYT", Offset: "+08:00", DST: false, Countries: `["MY"]`},
		{ID: "Asia/Bangkok", Name: "Indochina Time", Abbreviation: "ICT", Offset: "+07:00", DST: false, Countries: `["TH"]`},
		{ID: "Asia/Ho_Chi_Minh", Name: "Vietnam Time", Abbreviation: "ICT", Offset: "+07:00", DST: false, Countries: `["VN"]`},
		{ID: "Asia/Jakarta", Name: "Western Indonesia Time", Abbreviation: "WIB", Offset: "+07:00", DST: false, Countries: `["ID"]`},
		{ID: "Asia/Makassar", Name: "Central Indonesia Time", Abbreviation: "WITA", Offset: "+08:00", DST: false, Countries: `["ID"]`},
		{ID: "Asia/Jayapura", Name: "Eastern Indonesia Time", Abbreviation: "WIT", Offset: "+09:00", DST: false, Countries: `["ID"]`},
		{ID: "Asia/Manila", Name: "Philippine Time", Abbreviation: "PHT", Offset: "+08:00", DST: false, Countries: `["PH"]`},
		{ID: "Asia/Yangon", Name: "Myanmar Time", Abbreviation: "MMT", Offset: "+06:30", DST: false, Countries: `["MM"]`},
		{ID: "Asia/Phnom_Penh", Name: "Indochina Time", Abbreviation: "ICT", Offset: "+07:00", DST: false, Countries: `["KH"]`},

		// East Asia
		{ID: "Asia/Tokyo", Name: "Japan Standard Time", Abbreviation: "JST", Offset: "+09:00", DST: false, Countries: `["JP"]`},
		{ID: "Asia/Seoul", Name: "Korea Standard Time", Abbreviation: "KST", Offset: "+09:00", DST: false, Countries: `["KR"]`},
		{ID: "Asia/Shanghai", Name: "China Standard Time", Abbreviation: "CST", Offset: "+08:00", DST: false, Countries: `["CN"]`},
		{ID: "Asia/Hong_Kong", Name: "Hong Kong Time", Abbreviation: "HKT", Offset: "+08:00", DST: false, Countries: `["HK"]`},
		{ID: "Asia/Taipei", Name: "Taiwan Time", Abbreviation: "CST", Offset: "+08:00", DST: false, Countries: `["TW"]`},

		// Central Asia
		{ID: "Asia/Almaty", Name: "Alma-Ata Time", Abbreviation: "ALMT", Offset: "+06:00", DST: false, Countries: `["KZ"]`},
		{ID: "Asia/Tashkent", Name: "Uzbekistan Time", Abbreviation: "UZT", Offset: "+05:00", DST: false, Countries: `["UZ"]`},

		// ========== Oceania ==========
		{ID: "Australia/Sydney", Name: "Australian Eastern Time", Abbreviation: "AEST/AEDT", Offset: "+10:00", DST: true, Countries: `["AU"]`},
		{ID: "Australia/Melbourne", Name: "Australian Eastern Time", Abbreviation: "AEST/AEDT", Offset: "+10:00", DST: true, Countries: `["AU"]`},
		{ID: "Australia/Brisbane", Name: "Australian Eastern Standard Time", Abbreviation: "AEST", Offset: "+10:00", DST: false, Countries: `["AU"]`},
		{ID: "Australia/Perth", Name: "Australian Western Standard Time", Abbreviation: "AWST", Offset: "+08:00", DST: false, Countries: `["AU"]`},
		{ID: "Australia/Adelaide", Name: "Australian Central Time", Abbreviation: "ACST/ACDT", Offset: "+09:30", DST: true, Countries: `["AU"]`},
		{ID: "Australia/Darwin", Name: "Australian Central Standard Time", Abbreviation: "ACST", Offset: "+09:30", DST: false, Countries: `["AU"]`},
		{ID: "Pacific/Auckland", Name: "New Zealand Time", Abbreviation: "NZST/NZDT", Offset: "+12:00", DST: true, Countries: `["NZ"]`},
		{ID: "Pacific/Fiji", Name: "Fiji Time", Abbreviation: "FJT", Offset: "+12:00", DST: false, Countries: `["FJ"]`},
		{ID: "Pacific/Guam", Name: "Chamorro Standard Time", Abbreviation: "ChST", Offset: "+10:00", DST: false, Countries: `["GU"]`},

		// ========== Africa ==========
		{ID: "Africa/Johannesburg", Name: "South Africa Standard Time", Abbreviation: "SAST", Offset: "+02:00", DST: false, Countries: `["ZA"]`},
		{ID: "Africa/Cairo", Name: "Eastern European Time", Abbreviation: "EET", Offset: "+02:00", DST: false, Countries: `["EG"]`},
		{ID: "Africa/Lagos", Name: "West Africa Time", Abbreviation: "WAT", Offset: "+01:00", DST: false, Countries: `["NG","GH"]`},
		{ID: "Africa/Nairobi", Name: "East Africa Time", Abbreviation: "EAT", Offset: "+03:00", DST: false, Countries: `["KE","UG","TZ"]`},
		{ID: "Africa/Casablanca", Name: "Western European Time", Abbreviation: "WET/WEST", Offset: "+00:00", DST: true, Countries: `["MA"]`},

		// ========== UTC ==========
		{ID: "UTC", Name: "Coordinated Universal Time", Abbreviation: "UTC", Offset: "+00:00", DST: false, Countries: `[]`},
	}
}
