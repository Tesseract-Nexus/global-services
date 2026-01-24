package seeder

import (
	"location-service/internal/models"
)

func getStatesData() []models.State {
	// Helper function for pointer conversion
	float64Ptr := func(f float64) *float64 { return &f }

	var states []models.State

	// United States States
	usStates := []models.State{
		{ID: "US-AL", Name: "Alabama", Code: "AL", CountryID: "US", Type: "state", Latitude: float64Ptr(32.3182), Longitude: float64Ptr(-86.9023), Active: true},
		{ID: "US-AK", Name: "Alaska", Code: "AK", CountryID: "US", Type: "state", Latitude: float64Ptr(64.2008), Longitude: float64Ptr(-149.4937), Active: true},
		{ID: "US-AZ", Name: "Arizona", Code: "AZ", CountryID: "US", Type: "state", Latitude: float64Ptr(34.0489), Longitude: float64Ptr(-111.0937), Active: true},
		{ID: "US-AR", Name: "Arkansas", Code: "AR", CountryID: "US", Type: "state", Latitude: float64Ptr(35.2010), Longitude: float64Ptr(-91.8318), Active: true},
		{ID: "US-CA", Name: "California", Code: "CA", CountryID: "US", Type: "state", Latitude: float64Ptr(36.7783), Longitude: float64Ptr(-119.4179), Active: true},
		{ID: "US-CO", Name: "Colorado", Code: "CO", CountryID: "US", Type: "state", Latitude: float64Ptr(39.5501), Longitude: float64Ptr(-105.7821), Active: true},
		{ID: "US-CT", Name: "Connecticut", Code: "CT", CountryID: "US", Type: "state", Latitude: float64Ptr(41.6032), Longitude: float64Ptr(-73.0877), Active: true},
		{ID: "US-DE", Name: "Delaware", Code: "DE", CountryID: "US", Type: "state", Latitude: float64Ptr(38.9108), Longitude: float64Ptr(-75.5277), Active: true},
		{ID: "US-FL", Name: "Florida", Code: "FL", CountryID: "US", Type: "state", Latitude: float64Ptr(27.6648), Longitude: float64Ptr(-81.5158), Active: true},
		{ID: "US-GA", Name: "Georgia", Code: "GA", CountryID: "US", Type: "state", Latitude: float64Ptr(32.1656), Longitude: float64Ptr(-82.9001), Active: true},
		{ID: "US-HI", Name: "Hawaii", Code: "HI", CountryID: "US", Type: "state", Latitude: float64Ptr(19.8968), Longitude: float64Ptr(-155.5828), Active: true},
		{ID: "US-ID", Name: "Idaho", Code: "ID", CountryID: "US", Type: "state", Latitude: float64Ptr(44.0682), Longitude: float64Ptr(-114.7420), Active: true},
		{ID: "US-IL", Name: "Illinois", Code: "IL", CountryID: "US", Type: "state", Latitude: float64Ptr(40.6331), Longitude: float64Ptr(-89.3985), Active: true},
		{ID: "US-IN", Name: "Indiana", Code: "IN", CountryID: "US", Type: "state", Latitude: float64Ptr(40.2672), Longitude: float64Ptr(-86.1349), Active: true},
		{ID: "US-IA", Name: "Iowa", Code: "IA", CountryID: "US", Type: "state", Latitude: float64Ptr(41.8780), Longitude: float64Ptr(-93.0977), Active: true},
		{ID: "US-KS", Name: "Kansas", Code: "KS", CountryID: "US", Type: "state", Latitude: float64Ptr(39.0119), Longitude: float64Ptr(-98.4842), Active: true},
		{ID: "US-KY", Name: "Kentucky", Code: "KY", CountryID: "US", Type: "state", Latitude: float64Ptr(37.8393), Longitude: float64Ptr(-84.2700), Active: true},
		{ID: "US-LA", Name: "Louisiana", Code: "LA", CountryID: "US", Type: "state", Latitude: float64Ptr(30.9843), Longitude: float64Ptr(-91.9623), Active: true},
		{ID: "US-ME", Name: "Maine", Code: "ME", CountryID: "US", Type: "state", Latitude: float64Ptr(45.2538), Longitude: float64Ptr(-69.4455), Active: true},
		{ID: "US-MD", Name: "Maryland", Code: "MD", CountryID: "US", Type: "state", Latitude: float64Ptr(39.0458), Longitude: float64Ptr(-76.6413), Active: true},
		{ID: "US-MA", Name: "Massachusetts", Code: "MA", CountryID: "US", Type: "state", Latitude: float64Ptr(42.4072), Longitude: float64Ptr(-71.3824), Active: true},
		{ID: "US-MI", Name: "Michigan", Code: "MI", CountryID: "US", Type: "state", Latitude: float64Ptr(44.3148), Longitude: float64Ptr(-85.6024), Active: true},
		{ID: "US-MN", Name: "Minnesota", Code: "MN", CountryID: "US", Type: "state", Latitude: float64Ptr(46.7296), Longitude: float64Ptr(-94.6859), Active: true},
		{ID: "US-MS", Name: "Mississippi", Code: "MS", CountryID: "US", Type: "state", Latitude: float64Ptr(32.3547), Longitude: float64Ptr(-89.3985), Active: true},
		{ID: "US-MO", Name: "Missouri", Code: "MO", CountryID: "US", Type: "state", Latitude: float64Ptr(37.9643), Longitude: float64Ptr(-91.8318), Active: true},
		{ID: "US-MT", Name: "Montana", Code: "MT", CountryID: "US", Type: "state", Latitude: float64Ptr(46.8797), Longitude: float64Ptr(-110.3626), Active: true},
		{ID: "US-NE", Name: "Nebraska", Code: "NE", CountryID: "US", Type: "state", Latitude: float64Ptr(41.4925), Longitude: float64Ptr(-99.9018), Active: true},
		{ID: "US-NV", Name: "Nevada", Code: "NV", CountryID: "US", Type: "state", Latitude: float64Ptr(38.8026), Longitude: float64Ptr(-116.4194), Active: true},
		{ID: "US-NH", Name: "New Hampshire", Code: "NH", CountryID: "US", Type: "state", Latitude: float64Ptr(43.1939), Longitude: float64Ptr(-71.5724), Active: true},
		{ID: "US-NJ", Name: "New Jersey", Code: "NJ", CountryID: "US", Type: "state", Latitude: float64Ptr(40.0583), Longitude: float64Ptr(-74.4057), Active: true},
		{ID: "US-NM", Name: "New Mexico", Code: "NM", CountryID: "US", Type: "state", Latitude: float64Ptr(34.5199), Longitude: float64Ptr(-105.8701), Active: true},
		{ID: "US-NY", Name: "New York", Code: "NY", CountryID: "US", Type: "state", Latitude: float64Ptr(43.2994), Longitude: float64Ptr(-74.2179), Active: true},
		{ID: "US-NC", Name: "North Carolina", Code: "NC", CountryID: "US", Type: "state", Latitude: float64Ptr(35.7596), Longitude: float64Ptr(-79.0193), Active: true},
		{ID: "US-ND", Name: "North Dakota", Code: "ND", CountryID: "US", Type: "state", Latitude: float64Ptr(47.5515), Longitude: float64Ptr(-101.0020), Active: true},
		{ID: "US-OH", Name: "Ohio", Code: "OH", CountryID: "US", Type: "state", Latitude: float64Ptr(40.4173), Longitude: float64Ptr(-82.9071), Active: true},
		{ID: "US-OK", Name: "Oklahoma", Code: "OK", CountryID: "US", Type: "state", Latitude: float64Ptr(35.4676), Longitude: float64Ptr(-97.5164), Active: true},
		{ID: "US-OR", Name: "Oregon", Code: "OR", CountryID: "US", Type: "state", Latitude: float64Ptr(43.8041), Longitude: float64Ptr(-120.5542), Active: true},
		{ID: "US-PA", Name: "Pennsylvania", Code: "PA", CountryID: "US", Type: "state", Latitude: float64Ptr(41.2033), Longitude: float64Ptr(-77.1945), Active: true},
		{ID: "US-RI", Name: "Rhode Island", Code: "RI", CountryID: "US", Type: "state", Latitude: float64Ptr(41.5801), Longitude: float64Ptr(-71.4774), Active: true},
		{ID: "US-SC", Name: "South Carolina", Code: "SC", CountryID: "US", Type: "state", Latitude: float64Ptr(33.8361), Longitude: float64Ptr(-81.1637), Active: true},
		{ID: "US-SD", Name: "South Dakota", Code: "SD", CountryID: "US", Type: "state", Latitude: float64Ptr(43.9695), Longitude: float64Ptr(-99.9018), Active: true},
		{ID: "US-TN", Name: "Tennessee", Code: "TN", CountryID: "US", Type: "state", Latitude: float64Ptr(35.5175), Longitude: float64Ptr(-86.5804), Active: true},
		{ID: "US-TX", Name: "Texas", Code: "TX", CountryID: "US", Type: "state", Latitude: float64Ptr(31.9686), Longitude: float64Ptr(-99.9018), Active: true},
		{ID: "US-UT", Name: "Utah", Code: "UT", CountryID: "US", Type: "state", Latitude: float64Ptr(39.3210), Longitude: float64Ptr(-111.0937), Active: true},
		{ID: "US-VT", Name: "Vermont", Code: "VT", CountryID: "US", Type: "state", Latitude: float64Ptr(44.5588), Longitude: float64Ptr(-72.5778), Active: true},
		{ID: "US-VA", Name: "Virginia", Code: "VA", CountryID: "US", Type: "state", Latitude: float64Ptr(37.4316), Longitude: float64Ptr(-78.6569), Active: true},
		{ID: "US-WA", Name: "Washington", Code: "WA", CountryID: "US", Type: "state", Latitude: float64Ptr(47.7511), Longitude: float64Ptr(-120.7401), Active: true},
		{ID: "US-WV", Name: "West Virginia", Code: "WV", CountryID: "US", Type: "state", Latitude: float64Ptr(38.5976), Longitude: float64Ptr(-80.4549), Active: true},
		{ID: "US-WI", Name: "Wisconsin", Code: "WI", CountryID: "US", Type: "state", Latitude: float64Ptr(43.7844), Longitude: float64Ptr(-88.7879), Active: true},
		{ID: "US-WY", Name: "Wyoming", Code: "WY", CountryID: "US", Type: "state", Latitude: float64Ptr(43.0750), Longitude: float64Ptr(-107.2903), Active: true},
	}

	// India States
	indiaStates := []models.State{
		{ID: "IN-AN", Name: "Andaman and Nicobar Islands", Code: "AN", CountryID: "IN", Type: "union territory", Latitude: float64Ptr(11.7401), Longitude: float64Ptr(92.6586), Active: true},
		{ID: "IN-AP", Name: "Andhra Pradesh", Code: "AP", CountryID: "IN", Type: "state", Latitude: float64Ptr(15.9129), Longitude: float64Ptr(79.7400), Active: true},
		{ID: "IN-AR", Name: "Arunachal Pradesh", Code: "AR", CountryID: "IN", Type: "state", Latitude: float64Ptr(28.2180), Longitude: float64Ptr(94.7278), Active: true},
		{ID: "IN-AS", Name: "Assam", Code: "AS", CountryID: "IN", Type: "state", Latitude: float64Ptr(26.2006), Longitude: float64Ptr(92.9376), Active: true},
		{ID: "IN-BR", Name: "Bihar", Code: "BR", CountryID: "IN", Type: "state", Latitude: float64Ptr(25.0961), Longitude: float64Ptr(85.3131), Active: true},
		{ID: "IN-CH", Name: "Chandigarh", Code: "CH", CountryID: "IN", Type: "union territory", Latitude: float64Ptr(30.7333), Longitude: float64Ptr(76.7794), Active: true},
		{ID: "IN-CT", Name: "Chhattisgarh", Code: "CT", CountryID: "IN", Type: "state", Latitude: float64Ptr(21.2787), Longitude: float64Ptr(81.8661), Active: true},
		{ID: "IN-DN", Name: "Dadra and Nagar Haveli and Daman and Diu", Code: "DN", CountryID: "IN", Type: "union territory", Latitude: float64Ptr(20.3974), Longitude: float64Ptr(72.8328), Active: true},
		{ID: "IN-DL", Name: "Delhi", Code: "DL", CountryID: "IN", Type: "union territory", Latitude: float64Ptr(28.7041), Longitude: float64Ptr(77.1025), Active: true},
		{ID: "IN-GA", Name: "Goa", Code: "GA", CountryID: "IN", Type: "state", Latitude: float64Ptr(15.2993), Longitude: float64Ptr(74.1240), Active: true},
		{ID: "IN-GJ", Name: "Gujarat", Code: "GJ", CountryID: "IN", Type: "state", Latitude: float64Ptr(22.2587), Longitude: float64Ptr(71.1924), Active: true},
		{ID: "IN-HR", Name: "Haryana", Code: "HR", CountryID: "IN", Type: "state", Latitude: float64Ptr(29.0588), Longitude: float64Ptr(76.0856), Active: true},
		{ID: "IN-HP", Name: "Himachal Pradesh", Code: "HP", CountryID: "IN", Type: "state", Latitude: float64Ptr(31.1048), Longitude: float64Ptr(77.1734), Active: true},
		{ID: "IN-JK", Name: "Jammu and Kashmir", Code: "JK", CountryID: "IN", Type: "union territory", Latitude: float64Ptr(33.7782), Longitude: float64Ptr(76.5762), Active: true},
		{ID: "IN-JH", Name: "Jharkhand", Code: "JH", CountryID: "IN", Type: "state", Latitude: float64Ptr(23.6102), Longitude: float64Ptr(85.2799), Active: true},
		{ID: "IN-KA", Name: "Karnataka", Code: "KA", CountryID: "IN", Type: "state", Latitude: float64Ptr(15.3173), Longitude: float64Ptr(75.7139), Active: true},
		{ID: "IN-KL", Name: "Kerala", Code: "KL", CountryID: "IN", Type: "state", Latitude: float64Ptr(10.8505), Longitude: float64Ptr(76.2711), Active: true},
		{ID: "IN-LA", Name: "Ladakh", Code: "LA", CountryID: "IN", Type: "union territory", Latitude: float64Ptr(34.1526), Longitude: float64Ptr(77.5771), Active: true},
		{ID: "IN-LD", Name: "Lakshadweep", Code: "LD", CountryID: "IN", Type: "union territory", Latitude: float64Ptr(10.5667), Longitude: float64Ptr(72.6417), Active: true},
		{ID: "IN-MP", Name: "Madhya Pradesh", Code: "MP", CountryID: "IN", Type: "state", Latitude: float64Ptr(22.9734), Longitude: float64Ptr(78.6569), Active: true},
		{ID: "IN-MH", Name: "Maharashtra", Code: "MH", CountryID: "IN", Type: "state", Latitude: float64Ptr(19.7515), Longitude: float64Ptr(75.7139), Active: true},
		{ID: "IN-MN", Name: "Manipur", Code: "MN", CountryID: "IN", Type: "state", Latitude: float64Ptr(24.6637), Longitude: float64Ptr(93.9063), Active: true},
		{ID: "IN-ML", Name: "Meghalaya", Code: "ML", CountryID: "IN", Type: "state", Latitude: float64Ptr(25.4670), Longitude: float64Ptr(91.3662), Active: true},
		{ID: "IN-MZ", Name: "Mizoram", Code: "MZ", CountryID: "IN", Type: "state", Latitude: float64Ptr(23.1645), Longitude: float64Ptr(92.9376), Active: true},
		{ID: "IN-NL", Name: "Nagaland", Code: "NL", CountryID: "IN", Type: "state", Latitude: float64Ptr(26.1584), Longitude: float64Ptr(94.5624), Active: true},
		{ID: "IN-OR", Name: "Odisha", Code: "OR", CountryID: "IN", Type: "state", Latitude: float64Ptr(20.9517), Longitude: float64Ptr(85.0985), Active: true},
		{ID: "IN-PY", Name: "Puducherry", Code: "PY", CountryID: "IN", Type: "union territory", Latitude: float64Ptr(11.9416), Longitude: float64Ptr(79.8083), Active: true},
		{ID: "IN-PB", Name: "Punjab", Code: "PB", CountryID: "IN", Type: "state", Latitude: float64Ptr(31.1471), Longitude: float64Ptr(75.3412), Active: true},
		{ID: "IN-RJ", Name: "Rajasthan", Code: "RJ", CountryID: "IN", Type: "state", Latitude: float64Ptr(27.0238), Longitude: float64Ptr(74.2179), Active: true},
		{ID: "IN-SK", Name: "Sikkim", Code: "SK", CountryID: "IN", Type: "state", Latitude: float64Ptr(27.5330), Longitude: float64Ptr(88.5122), Active: true},
		{ID: "IN-TN", Name: "Tamil Nadu", Code: "TN", CountryID: "IN", Type: "state", Latitude: float64Ptr(11.1271), Longitude: float64Ptr(78.6569), Active: true},
		{ID: "IN-TG", Name: "Telangana", Code: "TG", CountryID: "IN", Type: "state", Latitude: float64Ptr(18.1124), Longitude: float64Ptr(79.0193), Active: true},
		{ID: "IN-TR", Name: "Tripura", Code: "TR", CountryID: "IN", Type: "state", Latitude: float64Ptr(23.9408), Longitude: float64Ptr(91.9882), Active: true},
		{ID: "IN-UP", Name: "Uttar Pradesh", Code: "UP", CountryID: "IN", Type: "state", Latitude: float64Ptr(26.8467), Longitude: float64Ptr(80.9462), Active: true},
		{ID: "IN-UT", Name: "Uttarakhand", Code: "UT", CountryID: "IN", Type: "state", Latitude: float64Ptr(30.0668), Longitude: float64Ptr(79.0193), Active: true},
		{ID: "IN-WB", Name: "West Bengal", Code: "WB", CountryID: "IN", Type: "state", Latitude: float64Ptr(22.9868), Longitude: float64Ptr(87.8550), Active: true},
	}

	// Canada Provinces
	canadaProvinces := []models.State{
		{ID: "CA-AB", Name: "Alberta", Code: "AB", CountryID: "CA", Type: "province", Latitude: float64Ptr(53.9333), Longitude: float64Ptr(-116.5765), Active: true},
		{ID: "CA-BC", Name: "British Columbia", Code: "BC", CountryID: "CA", Type: "province", Latitude: float64Ptr(53.7267), Longitude: float64Ptr(-127.6476), Active: true},
		{ID: "CA-MB", Name: "Manitoba", Code: "MB", CountryID: "CA", Type: "province", Latitude: float64Ptr(53.7609), Longitude: float64Ptr(-98.8139), Active: true},
		{ID: "CA-NB", Name: "New Brunswick", Code: "NB", CountryID: "CA", Type: "province", Latitude: float64Ptr(46.5653), Longitude: float64Ptr(-66.4619), Active: true},
		{ID: "CA-NL", Name: "Newfoundland and Labrador", Code: "NL", CountryID: "CA", Type: "province", Latitude: float64Ptr(53.1355), Longitude: float64Ptr(-57.6604), Active: true},
		{ID: "CA-NS", Name: "Nova Scotia", Code: "NS", CountryID: "CA", Type: "province", Latitude: float64Ptr(44.6820), Longitude: float64Ptr(-63.7443), Active: true},
		{ID: "CA-ON", Name: "Ontario", Code: "ON", CountryID: "CA", Type: "province", Latitude: float64Ptr(51.2538), Longitude: float64Ptr(-85.3232), Active: true},
		{ID: "CA-PE", Name: "Prince Edward Island", Code: "PE", CountryID: "CA", Type: "province", Latitude: float64Ptr(46.5107), Longitude: float64Ptr(-63.4168), Active: true},
		{ID: "CA-QC", Name: "Quebec", Code: "QC", CountryID: "CA", Type: "province", Latitude: float64Ptr(52.9399), Longitude: float64Ptr(-73.5491), Active: true},
		{ID: "CA-SK", Name: "Saskatchewan", Code: "SK", CountryID: "CA", Type: "province", Latitude: float64Ptr(52.9399), Longitude: float64Ptr(-106.4509), Active: true},
		{ID: "CA-NT", Name: "Northwest Territories", Code: "NT", CountryID: "CA", Type: "territory", Latitude: float64Ptr(64.8255), Longitude: float64Ptr(-124.8457), Active: true},
		{ID: "CA-NU", Name: "Nunavut", Code: "NU", CountryID: "CA", Type: "territory", Latitude: float64Ptr(70.2998), Longitude: float64Ptr(-83.1076), Active: true},
		{ID: "CA-YT", Name: "Yukon", Code: "YT", CountryID: "CA", Type: "territory", Latitude: float64Ptr(64.2823), Longitude: float64Ptr(-135.0000), Active: true},
	}

	// Australia States
	australiaStates := []models.State{
		{ID: "AU-NSW", Name: "New South Wales", Code: "NSW", CountryID: "AU", Type: "state", Latitude: float64Ptr(-33.8688), Longitude: float64Ptr(151.2093), Active: true},
		{ID: "AU-QLD", Name: "Queensland", Code: "QLD", CountryID: "AU", Type: "state", Latitude: float64Ptr(-27.4698), Longitude: float64Ptr(153.0251), Active: true},
		{ID: "AU-SA", Name: "South Australia", Code: "SA", CountryID: "AU", Type: "state", Latitude: float64Ptr(-34.9285), Longitude: float64Ptr(138.6007), Active: true},
		{ID: "AU-TAS", Name: "Tasmania", Code: "TAS", CountryID: "AU", Type: "state", Latitude: float64Ptr(-42.8821), Longitude: float64Ptr(147.3272), Active: true},
		{ID: "AU-VIC", Name: "Victoria", Code: "VIC", CountryID: "AU", Type: "state", Latitude: float64Ptr(-37.8136), Longitude: float64Ptr(144.9631), Active: true},
		{ID: "AU-WA", Name: "Western Australia", Code: "WA", CountryID: "AU", Type: "state", Latitude: float64Ptr(-31.9505), Longitude: float64Ptr(115.8605), Active: true},
		{ID: "AU-ACT", Name: "Australian Capital Territory", Code: "ACT", CountryID: "AU", Type: "territory", Latitude: float64Ptr(-35.2809), Longitude: float64Ptr(149.1300), Active: true},
		{ID: "AU-NT", Name: "Northern Territory", Code: "NT", CountryID: "AU", Type: "territory", Latitude: float64Ptr(-12.4634), Longitude: float64Ptr(130.8456), Active: true},
	}

	// Nepal Provinces
	nepalProvinces := []models.State{
		{ID: "NP-P1", Name: "Koshi Province", Code: "P1", CountryID: "NP", Type: "province", Latitude: float64Ptr(27.0), Longitude: float64Ptr(87.3), Active: true},
		{ID: "NP-P2", Name: "Madhesh Province", Code: "P2", CountryID: "NP", Type: "province", Latitude: float64Ptr(26.7), Longitude: float64Ptr(85.9), Active: true},
		{ID: "NP-P3", Name: "Bagmati Province", Code: "P3", CountryID: "NP", Type: "province", Latitude: float64Ptr(27.7), Longitude: float64Ptr(85.3), Active: true},
		{ID: "NP-P4", Name: "Gandaki Province", Code: "P4", CountryID: "NP", Type: "province", Latitude: float64Ptr(28.4), Longitude: float64Ptr(84.1), Active: true},
		{ID: "NP-P5", Name: "Lumbini Province", Code: "P5", CountryID: "NP", Type: "province", Latitude: float64Ptr(27.8), Longitude: float64Ptr(82.9), Active: true},
		{ID: "NP-P6", Name: "Karnali Province", Code: "P6", CountryID: "NP", Type: "province", Latitude: float64Ptr(29.4), Longitude: float64Ptr(82.4), Active: true},
		{ID: "NP-P7", Name: "Sudurpashchim Province", Code: "P7", CountryID: "NP", Type: "province", Latitude: float64Ptr(29.3), Longitude: float64Ptr(80.6), Active: true},
	}

	// Bangladesh Divisions
	bangladeshDivisions := []models.State{
		{ID: "BD-A", Name: "Barisal", Code: "A", CountryID: "BD", Type: "division", Latitude: float64Ptr(22.7), Longitude: float64Ptr(90.4), Active: true},
		{ID: "BD-B", Name: "Chittagong", Code: "B", CountryID: "BD", Type: "division", Latitude: float64Ptr(22.3), Longitude: float64Ptr(91.8), Active: true},
		{ID: "BD-C", Name: "Dhaka", Code: "C", CountryID: "BD", Type: "division", Latitude: float64Ptr(23.8), Longitude: float64Ptr(90.4), Active: true},
		{ID: "BD-D", Name: "Khulna", Code: "D", CountryID: "BD", Type: "division", Latitude: float64Ptr(22.8), Longitude: float64Ptr(89.5), Active: true},
		{ID: "BD-E", Name: "Rajshahi", Code: "E", CountryID: "BD", Type: "division", Latitude: float64Ptr(24.4), Longitude: float64Ptr(88.6), Active: true},
		{ID: "BD-F", Name: "Rangpur", Code: "F", CountryID: "BD", Type: "division", Latitude: float64Ptr(25.7), Longitude: float64Ptr(89.3), Active: true},
		{ID: "BD-G", Name: "Sylhet", Code: "G", CountryID: "BD", Type: "division", Latitude: float64Ptr(24.9), Longitude: float64Ptr(91.9), Active: true},
		{ID: "BD-H", Name: "Mymensingh", Code: "H", CountryID: "BD", Type: "division", Latitude: float64Ptr(24.7), Longitude: float64Ptr(90.4), Active: true},
	}

	// Sri Lanka Provinces
	sriLankaProvinces := []models.State{
		{ID: "LK-1", Name: "Western Province", Code: "1", CountryID: "LK", Type: "province", Latitude: float64Ptr(6.9), Longitude: float64Ptr(79.9), Active: true},
		{ID: "LK-2", Name: "Central Province", Code: "2", CountryID: "LK", Type: "province", Latitude: float64Ptr(7.3), Longitude: float64Ptr(80.6), Active: true},
		{ID: "LK-3", Name: "Southern Province", Code: "3", CountryID: "LK", Type: "province", Latitude: float64Ptr(6.1), Longitude: float64Ptr(80.5), Active: true},
		{ID: "LK-4", Name: "Northern Province", Code: "4", CountryID: "LK", Type: "province", Latitude: float64Ptr(9.7), Longitude: float64Ptr(80.0), Active: true},
		{ID: "LK-5", Name: "Eastern Province", Code: "5", CountryID: "LK", Type: "province", Latitude: float64Ptr(7.7), Longitude: float64Ptr(81.5), Active: true},
		{ID: "LK-6", Name: "North Western Province", Code: "6", CountryID: "LK", Type: "province", Latitude: float64Ptr(7.8), Longitude: float64Ptr(80.0), Active: true},
		{ID: "LK-7", Name: "North Central Province", Code: "7", CountryID: "LK", Type: "province", Latitude: float64Ptr(8.3), Longitude: float64Ptr(80.4), Active: true},
		{ID: "LK-8", Name: "Uva Province", Code: "8", CountryID: "LK", Type: "province", Latitude: float64Ptr(6.8), Longitude: float64Ptr(81.1), Active: true},
		{ID: "LK-9", Name: "Sabaragamuwa Province", Code: "9", CountryID: "LK", Type: "province", Latitude: float64Ptr(6.8), Longitude: float64Ptr(80.5), Active: true},
	}

	// Pakistan Provinces
	pakistanProvinces := []models.State{
		{ID: "PK-PB", Name: "Punjab", Code: "PB", CountryID: "PK", Type: "province", Latitude: float64Ptr(31.1), Longitude: float64Ptr(72.7), Active: true},
		{ID: "PK-SD", Name: "Sindh", Code: "SD", CountryID: "PK", Type: "province", Latitude: float64Ptr(25.9), Longitude: float64Ptr(68.5), Active: true},
		{ID: "PK-KP", Name: "Khyber Pakhtunkhwa", Code: "KP", CountryID: "PK", Type: "province", Latitude: float64Ptr(34.2), Longitude: float64Ptr(71.8), Active: true},
		{ID: "PK-BA", Name: "Balochistan", Code: "BA", CountryID: "PK", Type: "province", Latitude: float64Ptr(28.5), Longitude: float64Ptr(65.1), Active: true},
		{ID: "PK-IS", Name: "Islamabad Capital Territory", Code: "IS", CountryID: "PK", Type: "territory", Latitude: float64Ptr(33.7), Longitude: float64Ptr(73.1), Active: true},
		{ID: "PK-GB", Name: "Gilgit-Baltistan", Code: "GB", CountryID: "PK", Type: "territory", Latitude: float64Ptr(35.8), Longitude: float64Ptr(74.9), Active: true},
		{ID: "PK-JK", Name: "Azad Jammu and Kashmir", Code: "JK", CountryID: "PK", Type: "territory", Latitude: float64Ptr(33.9), Longitude: float64Ptr(73.8), Active: true},
	}

	// Indonesia Provinces (Major ones)
	indonesiaProvinces := []models.State{
		{ID: "ID-JK", Name: "Jakarta", Code: "JK", CountryID: "ID", Type: "province", Latitude: float64Ptr(-6.2), Longitude: float64Ptr(106.8), Active: true},
		{ID: "ID-JB", Name: "West Java", Code: "JB", CountryID: "ID", Type: "province", Latitude: float64Ptr(-6.9), Longitude: float64Ptr(107.6), Active: true},
		{ID: "ID-JT", Name: "Central Java", Code: "JT", CountryID: "ID", Type: "province", Latitude: float64Ptr(-7.2), Longitude: float64Ptr(110.4), Active: true},
		{ID: "ID-JI", Name: "East Java", Code: "JI", CountryID: "ID", Type: "province", Latitude: float64Ptr(-7.5), Longitude: float64Ptr(112.8), Active: true},
		{ID: "ID-YO", Name: "Yogyakarta", Code: "YO", CountryID: "ID", Type: "province", Latitude: float64Ptr(-7.8), Longitude: float64Ptr(110.4), Active: true},
		{ID: "ID-BA", Name: "Bali", Code: "BA", CountryID: "ID", Type: "province", Latitude: float64Ptr(-8.4), Longitude: float64Ptr(115.2), Active: true},
		{ID: "ID-SU", Name: "North Sumatra", Code: "SU", CountryID: "ID", Type: "province", Latitude: float64Ptr(2.1), Longitude: float64Ptr(99.5), Active: true},
		{ID: "ID-SS", Name: "South Sumatra", Code: "SS", CountryID: "ID", Type: "province", Latitude: float64Ptr(-3.3), Longitude: float64Ptr(104.5), Active: true},
		{ID: "ID-KS", Name: "South Kalimantan", Code: "KS", CountryID: "ID", Type: "province", Latitude: float64Ptr(-3.1), Longitude: float64Ptr(115.4), Active: true},
		{ID: "ID-KT", Name: "East Kalimantan", Code: "KT", CountryID: "ID", Type: "province", Latitude: float64Ptr(1.0), Longitude: float64Ptr(116.4), Active: true},
		{ID: "ID-SA", Name: "North Sulawesi", Code: "SA", CountryID: "ID", Type: "province", Latitude: float64Ptr(1.5), Longitude: float64Ptr(124.8), Active: true},
		{ID: "ID-SN", Name: "South Sulawesi", Code: "SN", CountryID: "ID", Type: "province", Latitude: float64Ptr(-5.1), Longitude: float64Ptr(119.9), Active: true},
	}

	// Philippines Regions
	philippinesRegions := []models.State{
		{ID: "PH-NCR", Name: "Metro Manila", Code: "NCR", CountryID: "PH", Type: "region", Latitude: float64Ptr(14.6), Longitude: float64Ptr(121.0), Active: true},
		{ID: "PH-01", Name: "Ilocos Region", Code: "01", CountryID: "PH", Type: "region", Latitude: float64Ptr(17.0), Longitude: float64Ptr(120.5), Active: true},
		{ID: "PH-02", Name: "Cagayan Valley", Code: "02", CountryID: "PH", Type: "region", Latitude: float64Ptr(17.6), Longitude: float64Ptr(121.7), Active: true},
		{ID: "PH-03", Name: "Central Luzon", Code: "03", CountryID: "PH", Type: "region", Latitude: float64Ptr(15.5), Longitude: float64Ptr(120.7), Active: true},
		{ID: "PH-4A", Name: "Calabarzon", Code: "4A", CountryID: "PH", Type: "region", Latitude: float64Ptr(14.1), Longitude: float64Ptr(121.3), Active: true},
		{ID: "PH-05", Name: "Bicol Region", Code: "05", CountryID: "PH", Type: "region", Latitude: float64Ptr(13.4), Longitude: float64Ptr(123.4), Active: true},
		{ID: "PH-06", Name: "Western Visayas", Code: "06", CountryID: "PH", Type: "region", Latitude: float64Ptr(11.0), Longitude: float64Ptr(122.5), Active: true},
		{ID: "PH-07", Name: "Central Visayas", Code: "07", CountryID: "PH", Type: "region", Latitude: float64Ptr(9.8), Longitude: float64Ptr(123.7), Active: true},
		{ID: "PH-08", Name: "Eastern Visayas", Code: "08", CountryID: "PH", Type: "region", Latitude: float64Ptr(11.2), Longitude: float64Ptr(125.0), Active: true},
		{ID: "PH-09", Name: "Zamboanga Peninsula", Code: "09", CountryID: "PH", Type: "region", Latitude: float64Ptr(7.8), Longitude: float64Ptr(123.4), Active: true},
		{ID: "PH-10", Name: "Northern Mindanao", Code: "10", CountryID: "PH", Type: "region", Latitude: float64Ptr(8.0), Longitude: float64Ptr(124.7), Active: true},
		{ID: "PH-11", Name: "Davao Region", Code: "11", CountryID: "PH", Type: "region", Latitude: float64Ptr(7.0), Longitude: float64Ptr(125.5), Active: true},
	}

	// Thailand Regions
	thailandRegions := []models.State{
		{ID: "TH-10", Name: "Bangkok", Code: "10", CountryID: "TH", Type: "province", Latitude: float64Ptr(13.8), Longitude: float64Ptr(100.5), Active: true},
		{ID: "TH-50", Name: "Chiang Mai", Code: "50", CountryID: "TH", Type: "province", Latitude: float64Ptr(18.8), Longitude: float64Ptr(99.0), Active: true},
		{ID: "TH-20", Name: "Chonburi", Code: "20", CountryID: "TH", Type: "province", Latitude: float64Ptr(13.4), Longitude: float64Ptr(101.0), Active: true},
		{ID: "TH-83", Name: "Phuket", Code: "83", CountryID: "TH", Type: "province", Latitude: float64Ptr(7.9), Longitude: float64Ptr(98.4), Active: true},
		{ID: "TH-40", Name: "Khon Kaen", Code: "40", CountryID: "TH", Type: "province", Latitude: float64Ptr(16.4), Longitude: float64Ptr(102.8), Active: true},
		{ID: "TH-30", Name: "Nakhon Ratchasima", Code: "30", CountryID: "TH", Type: "province", Latitude: float64Ptr(14.9), Longitude: float64Ptr(102.1), Active: true},
		{ID: "TH-90", Name: "Songkhla", Code: "90", CountryID: "TH", Type: "province", Latitude: float64Ptr(7.2), Longitude: float64Ptr(100.6), Active: true},
		{ID: "TH-71", Name: "Kanchanaburi", Code: "71", CountryID: "TH", Type: "province", Latitude: float64Ptr(14.0), Longitude: float64Ptr(99.5), Active: true},
	}

	// Vietnam Regions
	vietnamRegions := []models.State{
		{ID: "VN-HN", Name: "Hanoi", Code: "HN", CountryID: "VN", Type: "municipality", Latitude: float64Ptr(21.0), Longitude: float64Ptr(105.9), Active: true},
		{ID: "VN-SG", Name: "Ho Chi Minh City", Code: "SG", CountryID: "VN", Type: "municipality", Latitude: float64Ptr(10.8), Longitude: float64Ptr(106.6), Active: true},
		{ID: "VN-DN", Name: "Da Nang", Code: "DN", CountryID: "VN", Type: "municipality", Latitude: float64Ptr(16.1), Longitude: float64Ptr(108.2), Active: true},
		{ID: "VN-HP", Name: "Hai Phong", Code: "HP", CountryID: "VN", Type: "municipality", Latitude: float64Ptr(20.9), Longitude: float64Ptr(106.7), Active: true},
		{ID: "VN-CT", Name: "Can Tho", Code: "CT", CountryID: "VN", Type: "municipality", Latitude: float64Ptr(10.0), Longitude: float64Ptr(105.8), Active: true},
		{ID: "VN-01", Name: "Lai Chau", Code: "01", CountryID: "VN", Type: "province", Latitude: float64Ptr(22.4), Longitude: float64Ptr(103.5), Active: true},
		{ID: "VN-44", Name: "Quang Binh", Code: "44", CountryID: "VN", Type: "province", Latitude: float64Ptr(17.5), Longitude: float64Ptr(106.3), Active: true},
	}

	// Malaysia States
	malaysiaStates := []models.State{
		{ID: "MY-14", Name: "Kuala Lumpur", Code: "14", CountryID: "MY", Type: "federal territory", Latitude: float64Ptr(3.1), Longitude: float64Ptr(101.7), Active: true},
		{ID: "MY-01", Name: "Johor", Code: "01", CountryID: "MY", Type: "state", Latitude: float64Ptr(1.5), Longitude: float64Ptr(103.8), Active: true},
		{ID: "MY-02", Name: "Kedah", Code: "02", CountryID: "MY", Type: "state", Latitude: float64Ptr(6.1), Longitude: float64Ptr(100.4), Active: true},
		{ID: "MY-03", Name: "Kelantan", Code: "03", CountryID: "MY", Type: "state", Latitude: float64Ptr(5.3), Longitude: float64Ptr(102.0), Active: true},
		{ID: "MY-04", Name: "Malacca", Code: "04", CountryID: "MY", Type: "state", Latitude: float64Ptr(2.2), Longitude: float64Ptr(102.2), Active: true},
		{ID: "MY-05", Name: "Negeri Sembilan", Code: "05", CountryID: "MY", Type: "state", Latitude: float64Ptr(2.7), Longitude: float64Ptr(102.0), Active: true},
		{ID: "MY-06", Name: "Pahang", Code: "06", CountryID: "MY", Type: "state", Latitude: float64Ptr(3.8), Longitude: float64Ptr(103.3), Active: true},
		{ID: "MY-07", Name: "Penang", Code: "07", CountryID: "MY", Type: "state", Latitude: float64Ptr(5.4), Longitude: float64Ptr(100.3), Active: true},
		{ID: "MY-08", Name: "Perak", Code: "08", CountryID: "MY", Type: "state", Latitude: float64Ptr(4.6), Longitude: float64Ptr(101.1), Active: true},
		{ID: "MY-09", Name: "Perlis", Code: "09", CountryID: "MY", Type: "state", Latitude: float64Ptr(6.5), Longitude: float64Ptr(100.2), Active: true},
		{ID: "MY-10", Name: "Selangor", Code: "10", CountryID: "MY", Type: "state", Latitude: float64Ptr(3.1), Longitude: float64Ptr(101.5), Active: true},
		{ID: "MY-11", Name: "Terengganu", Code: "11", CountryID: "MY", Type: "state", Latitude: float64Ptr(5.3), Longitude: float64Ptr(103.1), Active: true},
		{ID: "MY-12", Name: "Sabah", Code: "12", CountryID: "MY", Type: "state", Latitude: float64Ptr(5.4), Longitude: float64Ptr(116.8), Active: true},
		{ID: "MY-13", Name: "Sarawak", Code: "13", CountryID: "MY", Type: "state", Latitude: float64Ptr(2.5), Longitude: float64Ptr(113.0), Active: true},
	}

	// Japan Prefectures (Major ones)
	japanPrefectures := []models.State{
		{ID: "JP-13", Name: "Tokyo", Code: "13", CountryID: "JP", Type: "prefecture", Latitude: float64Ptr(35.7), Longitude: float64Ptr(139.7), Active: true},
		{ID: "JP-27", Name: "Osaka", Code: "27", CountryID: "JP", Type: "prefecture", Latitude: float64Ptr(34.7), Longitude: float64Ptr(135.5), Active: true},
		{ID: "JP-23", Name: "Aichi", Code: "23", CountryID: "JP", Type: "prefecture", Latitude: float64Ptr(35.2), Longitude: float64Ptr(137.0), Active: true},
		{ID: "JP-14", Name: "Kanagawa", Code: "14", CountryID: "JP", Type: "prefecture", Latitude: float64Ptr(35.5), Longitude: float64Ptr(139.6), Active: true},
		{ID: "JP-28", Name: "Hyogo", Code: "28", CountryID: "JP", Type: "prefecture", Latitude: float64Ptr(34.7), Longitude: float64Ptr(135.2), Active: true},
		{ID: "JP-40", Name: "Fukuoka", Code: "40", CountryID: "JP", Type: "prefecture", Latitude: float64Ptr(33.6), Longitude: float64Ptr(130.4), Active: true},
		{ID: "JP-01", Name: "Hokkaido", Code: "01", CountryID: "JP", Type: "prefecture", Latitude: float64Ptr(43.1), Longitude: float64Ptr(141.3), Active: true},
		{ID: "JP-26", Name: "Kyoto", Code: "26", CountryID: "JP", Type: "prefecture", Latitude: float64Ptr(35.0), Longitude: float64Ptr(135.8), Active: true},
	}

	// South Korea Provinces
	southKoreaProvinces := []models.State{
		{ID: "KR-11", Name: "Seoul", Code: "11", CountryID: "KR", Type: "special city", Latitude: float64Ptr(37.6), Longitude: float64Ptr(127.0), Active: true},
		{ID: "KR-26", Name: "Busan", Code: "26", CountryID: "KR", Type: "metropolitan city", Latitude: float64Ptr(35.2), Longitude: float64Ptr(129.1), Active: true},
		{ID: "KR-27", Name: "Daegu", Code: "27", CountryID: "KR", Type: "metropolitan city", Latitude: float64Ptr(35.9), Longitude: float64Ptr(128.6), Active: true},
		{ID: "KR-28", Name: "Incheon", Code: "28", CountryID: "KR", Type: "metropolitan city", Latitude: float64Ptr(37.5), Longitude: float64Ptr(126.7), Active: true},
		{ID: "KR-41", Name: "Gyeonggi", Code: "41", CountryID: "KR", Type: "province", Latitude: float64Ptr(37.4), Longitude: float64Ptr(127.5), Active: true},
		{ID: "KR-42", Name: "Gangwon", Code: "42", CountryID: "KR", Type: "province", Latitude: float64Ptr(37.9), Longitude: float64Ptr(128.2), Active: true},
		{ID: "KR-47", Name: "North Gyeongsang", Code: "47", CountryID: "KR", Type: "province", Latitude: float64Ptr(36.4), Longitude: float64Ptr(128.9), Active: true},
		{ID: "KR-50", Name: "Jeju", Code: "50", CountryID: "KR", Type: "province", Latitude: float64Ptr(33.4), Longitude: float64Ptr(126.5), Active: true},
	}

	// Afghanistan Provinces (34)
	afghanistanProvinces := []models.State{
		{ID: "AF-BAL", Name: "Balkh", Code: "BAL", CountryID: "AF", Type: "province", Latitude: float64Ptr(36.8), Longitude: float64Ptr(66.9), Active: true},
		{ID: "AF-BAM", Name: "Bamyan", Code: "BAM", CountryID: "AF", Type: "province", Latitude: float64Ptr(34.8), Longitude: float64Ptr(67.8), Active: true},
		{ID: "AF-BDG", Name: "Badghis", Code: "BDG", CountryID: "AF", Type: "province", Latitude: float64Ptr(35.2), Longitude: float64Ptr(63.8), Active: true},
		{ID: "AF-BDS", Name: "Badakhshan", Code: "BDS", CountryID: "AF", Type: "province", Latitude: float64Ptr(36.7), Longitude: float64Ptr(70.8), Active: true},
		{ID: "AF-BGL", Name: "Baghlan", Code: "BGL", CountryID: "AF", Type: "province", Latitude: float64Ptr(36.1), Longitude: float64Ptr(68.7), Active: true},
		{ID: "AF-DAY", Name: "Daykundi", Code: "DAY", CountryID: "AF", Type: "province", Latitude: float64Ptr(33.7), Longitude: float64Ptr(66.0), Active: true},
		{ID: "AF-FRA", Name: "Farah", Code: "FRA", CountryID: "AF", Type: "province", Latitude: float64Ptr(32.5), Longitude: float64Ptr(62.1), Active: true},
		{ID: "AF-FYB", Name: "Faryab", Code: "FYB", CountryID: "AF", Type: "province", Latitude: float64Ptr(36.1), Longitude: float64Ptr(64.9), Active: true},
		{ID: "AF-GHA", Name: "Ghazni", Code: "GHA", CountryID: "AF", Type: "province", Latitude: float64Ptr(33.5), Longitude: float64Ptr(68.4), Active: true},
		{ID: "AF-GHO", Name: "Ghor", Code: "GHO", CountryID: "AF", Type: "province", Latitude: float64Ptr(34.5), Longitude: float64Ptr(64.9), Active: true},
		{ID: "AF-HEL", Name: "Helmand", Code: "HEL", CountryID: "AF", Type: "province", Latitude: float64Ptr(31.6), Longitude: float64Ptr(64.4), Active: true},
		{ID: "AF-HER", Name: "Herat", Code: "HER", CountryID: "AF", Type: "province", Latitude: float64Ptr(34.3), Longitude: float64Ptr(62.2), Active: true},
		{ID: "AF-JOW", Name: "Jowzjan", Code: "JOW", CountryID: "AF", Type: "province", Latitude: float64Ptr(36.9), Longitude: float64Ptr(65.7), Active: true},
		{ID: "AF-KAB", Name: "Kabul", Code: "KAB", CountryID: "AF", Type: "province", Latitude: float64Ptr(34.5), Longitude: float64Ptr(69.2), Active: true},
		{ID: "AF-KAN", Name: "Kandahar", Code: "KAN", CountryID: "AF", Type: "province", Latitude: float64Ptr(31.6), Longitude: float64Ptr(65.7), Active: true},
		{ID: "AF-KAP", Name: "Kapisa", Code: "KAP", CountryID: "AF", Type: "province", Latitude: float64Ptr(34.9), Longitude: float64Ptr(69.6), Active: true},
		{ID: "AF-KDZ", Name: "Kunduz", Code: "KDZ", CountryID: "AF", Type: "province", Latitude: float64Ptr(36.7), Longitude: float64Ptr(68.9), Active: true},
		{ID: "AF-KHO", Name: "Khost", Code: "KHO", CountryID: "AF", Type: "province", Latitude: float64Ptr(33.3), Longitude: float64Ptr(69.9), Active: true},
		{ID: "AF-KNR", Name: "Kunar", Code: "KNR", CountryID: "AF", Type: "province", Latitude: float64Ptr(35.0), Longitude: float64Ptr(71.1), Active: true},
		{ID: "AF-LAG", Name: "Laghman", Code: "LAG", CountryID: "AF", Type: "province", Latitude: float64Ptr(34.7), Longitude: float64Ptr(70.1), Active: true},
		{ID: "AF-LOG", Name: "Logar", Code: "LOG", CountryID: "AF", Type: "province", Latitude: float64Ptr(34.0), Longitude: float64Ptr(69.0), Active: true},
		{ID: "AF-NAN", Name: "Nangarhar", Code: "NAN", CountryID: "AF", Type: "province", Latitude: float64Ptr(34.2), Longitude: float64Ptr(70.6), Active: true},
		{ID: "AF-NIM", Name: "Nimroz", Code: "NIM", CountryID: "AF", Type: "province", Latitude: float64Ptr(31.0), Longitude: float64Ptr(62.0), Active: true},
		{ID: "AF-NUR", Name: "Nuristan", Code: "NUR", CountryID: "AF", Type: "province", Latitude: float64Ptr(35.3), Longitude: float64Ptr(70.9), Active: true},
		{ID: "AF-PAN", Name: "Panjshir", Code: "PAN", CountryID: "AF", Type: "province", Latitude: float64Ptr(35.3), Longitude: float64Ptr(70.1), Active: true},
		{ID: "AF-PAR", Name: "Parwan", Code: "PAR", CountryID: "AF", Type: "province", Latitude: float64Ptr(35.0), Longitude: float64Ptr(68.8), Active: true},
		{ID: "AF-PIA", Name: "Paktia", Code: "PIA", CountryID: "AF", Type: "province", Latitude: float64Ptr(33.7), Longitude: float64Ptr(69.4), Active: true},
		{ID: "AF-PKA", Name: "Paktika", Code: "PKA", CountryID: "AF", Type: "province", Latitude: float64Ptr(32.3), Longitude: float64Ptr(68.5), Active: true},
		{ID: "AF-SAM", Name: "Samangan", Code: "SAM", CountryID: "AF", Type: "province", Latitude: float64Ptr(36.3), Longitude: float64Ptr(68.0), Active: true},
		{ID: "AF-SAR", Name: "Sar-e Pol", Code: "SAR", CountryID: "AF", Type: "province", Latitude: float64Ptr(36.2), Longitude: float64Ptr(65.9), Active: true},
		{ID: "AF-TAK", Name: "Takhar", Code: "TAK", CountryID: "AF", Type: "province", Latitude: float64Ptr(36.7), Longitude: float64Ptr(69.5), Active: true},
		{ID: "AF-URU", Name: "Uruzgan", Code: "URU", CountryID: "AF", Type: "province", Latitude: float64Ptr(32.9), Longitude: float64Ptr(66.6), Active: true},
		{ID: "AF-WAR", Name: "Wardak", Code: "WAR", CountryID: "AF", Type: "province", Latitude: float64Ptr(34.0), Longitude: float64Ptr(68.2), Active: true},
		{ID: "AF-ZAB", Name: "Zabul", Code: "ZAB", CountryID: "AF", Type: "province", Latitude: float64Ptr(32.1), Longitude: float64Ptr(67.2), Active: true},
	}

	// Brunei Districts (4)
	bruneiDistricts := []models.State{
		{ID: "BN-BE", Name: "Belait", Code: "BE", CountryID: "BN", Type: "district", Latitude: float64Ptr(4.6), Longitude: float64Ptr(114.4), Active: true},
		{ID: "BN-BM", Name: "Brunei-Muara", Code: "BM", CountryID: "BN", Type: "district", Latitude: float64Ptr(4.9), Longitude: float64Ptr(114.9), Active: true},
		{ID: "BN-TE", Name: "Temburong", Code: "TE", CountryID: "BN", Type: "district", Latitude: float64Ptr(4.6), Longitude: float64Ptr(115.1), Active: true},
		{ID: "BN-TU", Name: "Tutong", Code: "TU", CountryID: "BN", Type: "district", Latitude: float64Ptr(4.8), Longitude: float64Ptr(114.6), Active: true},
	}

	// Albania Counties (12)
	albaniaCounties := []models.State{
		{ID: "AL-01", Name: "Berat", Code: "01", CountryID: "AL", Type: "county", Latitude: float64Ptr(40.7), Longitude: float64Ptr(19.9), Active: true},
		{ID: "AL-09", Name: "Dibër", Code: "09", CountryID: "AL", Type: "county", Latitude: float64Ptr(41.6), Longitude: float64Ptr(20.2), Active: true},
		{ID: "AL-02", Name: "Durrës", Code: "02", CountryID: "AL", Type: "county", Latitude: float64Ptr(41.3), Longitude: float64Ptr(19.5), Active: true},
		{ID: "AL-03", Name: "Elbasan", Code: "03", CountryID: "AL", Type: "county", Latitude: float64Ptr(41.1), Longitude: float64Ptr(20.1), Active: true},
		{ID: "AL-04", Name: "Fier", Code: "04", CountryID: "AL", Type: "county", Latitude: float64Ptr(40.7), Longitude: float64Ptr(19.6), Active: true},
		{ID: "AL-05", Name: "Gjirokastër", Code: "05", CountryID: "AL", Type: "county", Latitude: float64Ptr(40.1), Longitude: float64Ptr(20.1), Active: true},
		{ID: "AL-06", Name: "Korçë", Code: "06", CountryID: "AL", Type: "county", Latitude: float64Ptr(40.6), Longitude: float64Ptr(20.8), Active: true},
		{ID: "AL-07", Name: "Kukës", Code: "07", CountryID: "AL", Type: "county", Latitude: float64Ptr(42.1), Longitude: float64Ptr(20.4), Active: true},
		{ID: "AL-08", Name: "Lezhë", Code: "08", CountryID: "AL", Type: "county", Latitude: float64Ptr(41.8), Longitude: float64Ptr(19.6), Active: true},
		{ID: "AL-10", Name: "Shkodër", Code: "10", CountryID: "AL", Type: "county", Latitude: float64Ptr(42.1), Longitude: float64Ptr(19.5), Active: true},
		{ID: "AL-11", Name: "Tirana", Code: "11", CountryID: "AL", Type: "county", Latitude: float64Ptr(41.3), Longitude: float64Ptr(19.8), Active: true},
		{ID: "AL-12", Name: "Vlorë", Code: "12", CountryID: "AL", Type: "county", Latitude: float64Ptr(40.5), Longitude: float64Ptr(19.5), Active: true},
	}

	// Argentina Provinces (24)
	argentinaProvinces := []models.State{
		{ID: "AR-C", Name: "Buenos Aires (City)", Code: "C", CountryID: "AR", Type: "autonomous city", Latitude: float64Ptr(-34.6), Longitude: float64Ptr(-58.4), Active: true},
		{ID: "AR-B", Name: "Buenos Aires (Province)", Code: "B", CountryID: "AR", Type: "province", Latitude: float64Ptr(-36.7), Longitude: float64Ptr(-60.0), Active: true},
		{ID: "AR-K", Name: "Catamarca", Code: "K", CountryID: "AR", Type: "province", Latitude: float64Ptr(-28.5), Longitude: float64Ptr(-65.8), Active: true},
		{ID: "AR-H", Name: "Chaco", Code: "H", CountryID: "AR", Type: "province", Latitude: float64Ptr(-26.4), Longitude: float64Ptr(-60.4), Active: true},
		{ID: "AR-U", Name: "Chubut", Code: "U", CountryID: "AR", Type: "province", Latitude: float64Ptr(-43.8), Longitude: float64Ptr(-68.5), Active: true},
		{ID: "AR-X", Name: "Córdoba", Code: "X", CountryID: "AR", Type: "province", Latitude: float64Ptr(-31.4), Longitude: float64Ptr(-64.2), Active: true},
		{ID: "AR-W", Name: "Corrientes", Code: "W", CountryID: "AR", Type: "province", Latitude: float64Ptr(-27.5), Longitude: float64Ptr(-58.8), Active: true},
		{ID: "AR-E", Name: "Entre Ríos", Code: "E", CountryID: "AR", Type: "province", Latitude: float64Ptr(-31.8), Longitude: float64Ptr(-59.0), Active: true},
		{ID: "AR-P", Name: "Formosa", Code: "P", CountryID: "AR", Type: "province", Latitude: float64Ptr(-24.9), Longitude: float64Ptr(-59.9), Active: true},
		{ID: "AR-Y", Name: "Jujuy", Code: "Y", CountryID: "AR", Type: "province", Latitude: float64Ptr(-23.3), Longitude: float64Ptr(-65.8), Active: true},
		{ID: "AR-L", Name: "La Pampa", Code: "L", CountryID: "AR", Type: "province", Latitude: float64Ptr(-36.6), Longitude: float64Ptr(-64.3), Active: true},
		{ID: "AR-F", Name: "La Rioja", Code: "F", CountryID: "AR", Type: "province", Latitude: float64Ptr(-29.4), Longitude: float64Ptr(-66.9), Active: true},
		{ID: "AR-M", Name: "Mendoza", Code: "M", CountryID: "AR", Type: "province", Latitude: float64Ptr(-34.6), Longitude: float64Ptr(-68.6), Active: true},
		{ID: "AR-N", Name: "Misiones", Code: "N", CountryID: "AR", Type: "province", Latitude: float64Ptr(-26.9), Longitude: float64Ptr(-54.4), Active: true},
		{ID: "AR-Q", Name: "Neuquén", Code: "Q", CountryID: "AR", Type: "province", Latitude: float64Ptr(-38.9), Longitude: float64Ptr(-70.1), Active: true},
		{ID: "AR-R", Name: "Río Negro", Code: "R", CountryID: "AR", Type: "province", Latitude: float64Ptr(-40.8), Longitude: float64Ptr(-67.0), Active: true},
		{ID: "AR-A", Name: "Salta", Code: "A", CountryID: "AR", Type: "province", Latitude: float64Ptr(-24.8), Longitude: float64Ptr(-65.4), Active: true},
		{ID: "AR-J", Name: "San Juan", Code: "J", CountryID: "AR", Type: "province", Latitude: float64Ptr(-31.5), Longitude: float64Ptr(-68.5), Active: true},
		{ID: "AR-D", Name: "San Luis", Code: "D", CountryID: "AR", Type: "province", Latitude: float64Ptr(-33.3), Longitude: float64Ptr(-66.4), Active: true},
		{ID: "AR-Z", Name: "Santa Cruz", Code: "Z", CountryID: "AR", Type: "province", Latitude: float64Ptr(-48.8), Longitude: float64Ptr(-69.2), Active: true},
		{ID: "AR-S", Name: "Santa Fe", Code: "S", CountryID: "AR", Type: "province", Latitude: float64Ptr(-31.6), Longitude: float64Ptr(-60.7), Active: true},
		{ID: "AR-G", Name: "Santiago del Estero", Code: "G", CountryID: "AR", Type: "province", Latitude: float64Ptr(-27.8), Longitude: float64Ptr(-64.3), Active: true},
		{ID: "AR-V", Name: "Tierra del Fuego", Code: "V", CountryID: "AR", Type: "province", Latitude: float64Ptr(-54.3), Longitude: float64Ptr(-67.7), Active: true},
		{ID: "AR-T", Name: "Tucumán", Code: "T", CountryID: "AR", Type: "province", Latitude: float64Ptr(-26.8), Longitude: float64Ptr(-65.2), Active: true},
	}

	// Bhutan Districts (20)
	bhutanDistricts := []models.State{
		{ID: "BT-33", Name: "Bumthang", Code: "33", CountryID: "BT", Type: "district", Latitude: float64Ptr(27.5), Longitude: float64Ptr(90.7), Active: true},
		{ID: "BT-12", Name: "Chhukha", Code: "12", CountryID: "BT", Type: "district", Latitude: float64Ptr(27.0), Longitude: float64Ptr(89.5), Active: true},
		{ID: "BT-22", Name: "Dagana", Code: "22", CountryID: "BT", Type: "district", Latitude: float64Ptr(27.1), Longitude: float64Ptr(89.9), Active: true},
		{ID: "BT-GA", Name: "Gasa", Code: "GA", CountryID: "BT", Type: "district", Latitude: float64Ptr(28.0), Longitude: float64Ptr(89.7), Active: true},
		{ID: "BT-13", Name: "Haa", Code: "13", CountryID: "BT", Type: "district", Latitude: float64Ptr(27.4), Longitude: float64Ptr(89.3), Active: true},
		{ID: "BT-44", Name: "Lhuentse", Code: "44", CountryID: "BT", Type: "district", Latitude: float64Ptr(27.7), Longitude: float64Ptr(91.2), Active: true},
		{ID: "BT-42", Name: "Mongar", Code: "42", CountryID: "BT", Type: "district", Latitude: float64Ptr(27.3), Longitude: float64Ptr(91.2), Active: true},
		{ID: "BT-11", Name: "Paro", Code: "11", CountryID: "BT", Type: "district", Latitude: float64Ptr(27.4), Longitude: float64Ptr(89.4), Active: true},
		{ID: "BT-43", Name: "Pemagatshel", Code: "43", CountryID: "BT", Type: "district", Latitude: float64Ptr(27.0), Longitude: float64Ptr(91.4), Active: true},
		{ID: "BT-23", Name: "Punakha", Code: "23", CountryID: "BT", Type: "district", Latitude: float64Ptr(27.6), Longitude: float64Ptr(89.9), Active: true},
		{ID: "BT-45", Name: "Samdrup Jongkhar", Code: "45", CountryID: "BT", Type: "district", Latitude: float64Ptr(26.8), Longitude: float64Ptr(91.5), Active: true},
		{ID: "BT-14", Name: "Samtse", Code: "14", CountryID: "BT", Type: "district", Latitude: float64Ptr(27.0), Longitude: float64Ptr(89.1), Active: true},
		{ID: "BT-31", Name: "Sarpang", Code: "31", CountryID: "BT", Type: "district", Latitude: float64Ptr(27.0), Longitude: float64Ptr(90.3), Active: true},
		{ID: "BT-15", Name: "Thimphu", Code: "15", CountryID: "BT", Type: "district", Latitude: float64Ptr(27.5), Longitude: float64Ptr(89.6), Active: true},
		{ID: "BT-41", Name: "Trashigang", Code: "41", CountryID: "BT", Type: "district", Latitude: float64Ptr(27.3), Longitude: float64Ptr(91.6), Active: true},
		{ID: "BT-TY", Name: "Trashiyangtse", Code: "TY", CountryID: "BT", Type: "district", Latitude: float64Ptr(27.6), Longitude: float64Ptr(91.5), Active: true},
		{ID: "BT-32", Name: "Trongsa", Code: "32", CountryID: "BT", Type: "district", Latitude: float64Ptr(27.5), Longitude: float64Ptr(90.5), Active: true},
		{ID: "BT-21", Name: "Tsirang", Code: "21", CountryID: "BT", Type: "district", Latitude: float64Ptr(27.0), Longitude: float64Ptr(90.1), Active: true},
		{ID: "BT-24", Name: "Wangdue Phodrang", Code: "24", CountryID: "BT", Type: "district", Latitude: float64Ptr(27.5), Longitude: float64Ptr(90.1), Active: true},
		{ID: "BT-34", Name: "Zhemgang", Code: "34", CountryID: "BT", Type: "district", Latitude: float64Ptr(27.2), Longitude: float64Ptr(90.7), Active: true},
	}

	// Germany States (16)
	germanyStates := []models.State{
		{ID: "DE-BW", Name: "Baden-Württemberg", Code: "BW", CountryID: "DE", Type: "state", Latitude: float64Ptr(48.7), Longitude: float64Ptr(9.1), Active: true},
		{ID: "DE-BY", Name: "Bavaria", Code: "BY", CountryID: "DE", Type: "state", Latitude: float64Ptr(48.8), Longitude: float64Ptr(11.4), Active: true},
		{ID: "DE-BE", Name: "Berlin", Code: "BE", CountryID: "DE", Type: "state", Latitude: float64Ptr(52.5), Longitude: float64Ptr(13.4), Active: true},
		{ID: "DE-BB", Name: "Brandenburg", Code: "BB", CountryID: "DE", Type: "state", Latitude: float64Ptr(52.4), Longitude: float64Ptr(13.1), Active: true},
		{ID: "DE-HB", Name: "Bremen", Code: "HB", CountryID: "DE", Type: "state", Latitude: float64Ptr(53.1), Longitude: float64Ptr(8.8), Active: true},
		{ID: "DE-HH", Name: "Hamburg", Code: "HH", CountryID: "DE", Type: "state", Latitude: float64Ptr(53.6), Longitude: float64Ptr(10.0), Active: true},
		{ID: "DE-HE", Name: "Hesse", Code: "HE", CountryID: "DE", Type: "state", Latitude: float64Ptr(50.7), Longitude: float64Ptr(9.0), Active: true},
		{ID: "DE-MV", Name: "Mecklenburg-Vorpommern", Code: "MV", CountryID: "DE", Type: "state", Latitude: float64Ptr(53.6), Longitude: float64Ptr(12.4), Active: true},
		{ID: "DE-NI", Name: "Lower Saxony", Code: "NI", CountryID: "DE", Type: "state", Latitude: float64Ptr(52.6), Longitude: float64Ptr(9.8), Active: true},
		{ID: "DE-NW", Name: "North Rhine-Westphalia", Code: "NW", CountryID: "DE", Type: "state", Latitude: float64Ptr(51.4), Longitude: float64Ptr(7.7), Active: true},
		{ID: "DE-RP", Name: "Rhineland-Palatinate", Code: "RP", CountryID: "DE", Type: "state", Latitude: float64Ptr(50.1), Longitude: float64Ptr(7.3), Active: true},
		{ID: "DE-SL", Name: "Saarland", Code: "SL", CountryID: "DE", Type: "state", Latitude: float64Ptr(49.4), Longitude: float64Ptr(7.0), Active: true},
		{ID: "DE-SN", Name: "Saxony", Code: "SN", CountryID: "DE", Type: "state", Latitude: float64Ptr(51.1), Longitude: float64Ptr(13.2), Active: true},
		{ID: "DE-ST", Name: "Saxony-Anhalt", Code: "ST", CountryID: "DE", Type: "state", Latitude: float64Ptr(51.9), Longitude: float64Ptr(11.5), Active: true},
		{ID: "DE-SH", Name: "Schleswig-Holstein", Code: "SH", CountryID: "DE", Type: "state", Latitude: float64Ptr(54.2), Longitude: float64Ptr(9.7), Active: true},
		{ID: "DE-TH", Name: "Thuringia", Code: "TH", CountryID: "DE", Type: "state", Latitude: float64Ptr(50.9), Longitude: float64Ptr(11.0), Active: true},
	}

	// United Kingdom Countries (4)
	ukCountries := []models.State{
		{ID: "GB-ENG", Name: "England", Code: "ENG", CountryID: "GB", Type: "country", Latitude: float64Ptr(52.4), Longitude: float64Ptr(-1.2), Active: true},
		{ID: "GB-SCT", Name: "Scotland", Code: "SCT", CountryID: "GB", Type: "country", Latitude: float64Ptr(56.5), Longitude: float64Ptr(-4.2), Active: true},
		{ID: "GB-WLS", Name: "Wales", Code: "WLS", CountryID: "GB", Type: "country", Latitude: float64Ptr(52.1), Longitude: float64Ptr(-3.8), Active: true},
		{ID: "GB-NIR", Name: "Northern Ireland", Code: "NIR", CountryID: "GB", Type: "country", Latitude: float64Ptr(54.6), Longitude: float64Ptr(-6.7), Active: true},
	}

	// France Regions (13)
	franceRegions := []models.State{
		{ID: "FR-IDF", Name: "Île-de-France", Code: "IDF", CountryID: "FR", Type: "region", Latitude: float64Ptr(48.8), Longitude: float64Ptr(2.3), Active: true},
		{ID: "FR-ARA", Name: "Auvergne-Rhône-Alpes", Code: "ARA", CountryID: "FR", Type: "region", Latitude: float64Ptr(45.4), Longitude: float64Ptr(4.4), Active: true},
		{ID: "FR-HDF", Name: "Hauts-de-France", Code: "HDF", CountryID: "FR", Type: "region", Latitude: float64Ptr(49.9), Longitude: float64Ptr(2.8), Active: true},
		{ID: "FR-NAQ", Name: "Nouvelle-Aquitaine", Code: "NAQ", CountryID: "FR", Type: "region", Latitude: float64Ptr(45.7), Longitude: float64Ptr(0.2), Active: true},
		{ID: "FR-OCC", Name: "Occitanie", Code: "OCC", CountryID: "FR", Type: "region", Latitude: float64Ptr(43.9), Longitude: float64Ptr(2.4), Active: true},
		{ID: "FR-GES", Name: "Grand Est", Code: "GES", CountryID: "FR", Type: "region", Latitude: float64Ptr(48.7), Longitude: float64Ptr(6.2), Active: true},
		{ID: "FR-PAC", Name: "Provence-Alpes-Côte d'Azur", Code: "PAC", CountryID: "FR", Type: "region", Latitude: float64Ptr(43.9), Longitude: float64Ptr(6.0), Active: true},
		{ID: "FR-PDL", Name: "Pays de la Loire", Code: "PDL", CountryID: "FR", Type: "region", Latitude: float64Ptr(47.5), Longitude: float64Ptr(-1.0), Active: true},
		{ID: "FR-BRE", Name: "Brittany", Code: "BRE", CountryID: "FR", Type: "region", Latitude: float64Ptr(48.2), Longitude: float64Ptr(-2.9), Active: true},
		{ID: "FR-NOR", Name: "Normandy", Code: "NOR", CountryID: "FR", Type: "region", Latitude: float64Ptr(49.2), Longitude: float64Ptr(-0.4), Active: true},
		{ID: "FR-BFC", Name: "Bourgogne-Franche-Comté", Code: "BFC", CountryID: "FR", Type: "region", Latitude: float64Ptr(47.3), Longitude: float64Ptr(5.0), Active: true},
		{ID: "FR-CVL", Name: "Centre-Val de Loire", Code: "CVL", CountryID: "FR", Type: "region", Latitude: float64Ptr(47.8), Longitude: float64Ptr(1.7), Active: true},
		{ID: "FR-COR", Name: "Corsica", Code: "COR", CountryID: "FR", Type: "region", Latitude: float64Ptr(42.0), Longitude: float64Ptr(9.0), Active: true},
	}

	// Spain Autonomous Communities (17)
	spainCommunities := []models.State{
		{ID: "ES-MD", Name: "Madrid", Code: "MD", CountryID: "ES", Type: "autonomous community", Latitude: float64Ptr(40.4), Longitude: float64Ptr(-3.7), Active: true},
		{ID: "ES-CT", Name: "Catalonia", Code: "CT", CountryID: "ES", Type: "autonomous community", Latitude: float64Ptr(41.6), Longitude: float64Ptr(1.5), Active: true},
		{ID: "ES-AN", Name: "Andalusia", Code: "AN", CountryID: "ES", Type: "autonomous community", Latitude: float64Ptr(37.4), Longitude: float64Ptr(-4.8), Active: true},
		{ID: "ES-VC", Name: "Valencia", Code: "VC", CountryID: "ES", Type: "autonomous community", Latitude: float64Ptr(39.5), Longitude: float64Ptr(-0.5), Active: true},
		{ID: "ES-GA", Name: "Galicia", Code: "GA", CountryID: "ES", Type: "autonomous community", Latitude: float64Ptr(42.6), Longitude: float64Ptr(-7.9), Active: true},
		{ID: "ES-CL", Name: "Castile and León", Code: "CL", CountryID: "ES", Type: "autonomous community", Latitude: float64Ptr(41.8), Longitude: float64Ptr(-4.0), Active: true},
		{ID: "ES-PV", Name: "Basque Country", Code: "PV", CountryID: "ES", Type: "autonomous community", Latitude: float64Ptr(42.9), Longitude: float64Ptr(-2.6), Active: true},
		{ID: "ES-CM", Name: "Castilla-La Mancha", Code: "CM", CountryID: "ES", Type: "autonomous community", Latitude: float64Ptr(39.3), Longitude: float64Ptr(-3.0), Active: true},
		{ID: "ES-CN", Name: "Canary Islands", Code: "CN", CountryID: "ES", Type: "autonomous community", Latitude: float64Ptr(28.3), Longitude: float64Ptr(-15.8), Active: true},
		{ID: "ES-MU", Name: "Murcia", Code: "MU", CountryID: "ES", Type: "autonomous community", Latitude: float64Ptr(37.9), Longitude: float64Ptr(-1.1), Active: true},
		{ID: "ES-AR", Name: "Aragon", Code: "AR", CountryID: "ES", Type: "autonomous community", Latitude: float64Ptr(41.6), Longitude: float64Ptr(-0.9), Active: true},
		{ID: "ES-IB", Name: "Balearic Islands", Code: "IB", CountryID: "ES", Type: "autonomous community", Latitude: float64Ptr(39.6), Longitude: float64Ptr(2.9), Active: true},
		{ID: "ES-EX", Name: "Extremadura", Code: "EX", CountryID: "ES", Type: "autonomous community", Latitude: float64Ptr(39.2), Longitude: float64Ptr(-6.2), Active: true},
		{ID: "ES-AS", Name: "Asturias", Code: "AS", CountryID: "ES", Type: "autonomous community", Latitude: float64Ptr(43.4), Longitude: float64Ptr(-5.8), Active: true},
		{ID: "ES-NC", Name: "Navarra", Code: "NC", CountryID: "ES", Type: "autonomous community", Latitude: float64Ptr(42.7), Longitude: float64Ptr(-1.7), Active: true},
		{ID: "ES-CB", Name: "Cantabria", Code: "CB", CountryID: "ES", Type: "autonomous community", Latitude: float64Ptr(43.2), Longitude: float64Ptr(-4.0), Active: true},
		{ID: "ES-RI", Name: "La Rioja", Code: "RI", CountryID: "ES", Type: "autonomous community", Latitude: float64Ptr(42.3), Longitude: float64Ptr(-2.4), Active: true},
	}

	// Italy Regions (20)
	italyRegions := []models.State{
		{ID: "IT-25", Name: "Lombardy", Code: "25", CountryID: "IT", Type: "region", Latitude: float64Ptr(45.5), Longitude: float64Ptr(9.9), Active: true},
		{ID: "IT-62", Name: "Lazio", Code: "62", CountryID: "IT", Type: "region", Latitude: float64Ptr(41.9), Longitude: float64Ptr(12.8), Active: true},
		{ID: "IT-72", Name: "Campania", Code: "72", CountryID: "IT", Type: "region", Latitude: float64Ptr(40.8), Longitude: float64Ptr(14.6), Active: true},
		{ID: "IT-82", Name: "Sicily", Code: "82", CountryID: "IT", Type: "region", Latitude: float64Ptr(37.6), Longitude: float64Ptr(14.0), Active: true},
		{ID: "IT-34", Name: "Veneto", Code: "34", CountryID: "IT", Type: "region", Latitude: float64Ptr(45.7), Longitude: float64Ptr(11.6), Active: true},
		{ID: "IT-21", Name: "Piedmont", Code: "21", CountryID: "IT", Type: "region", Latitude: float64Ptr(45.1), Longitude: float64Ptr(7.6), Active: true},
		{ID: "IT-45", Name: "Emilia-Romagna", Code: "45", CountryID: "IT", Type: "region", Latitude: float64Ptr(44.5), Longitude: float64Ptr(11.3), Active: true},
		{ID: "IT-75", Name: "Apulia", Code: "75", CountryID: "IT", Type: "region", Latitude: float64Ptr(41.0), Longitude: float64Ptr(16.5), Active: true},
		{ID: "IT-52", Name: "Tuscany", Code: "52", CountryID: "IT", Type: "region", Latitude: float64Ptr(43.4), Longitude: float64Ptr(11.1), Active: true},
		{ID: "IT-78", Name: "Calabria", Code: "78", CountryID: "IT", Type: "region", Latitude: float64Ptr(39.1), Longitude: float64Ptr(16.6), Active: true},
		{ID: "IT-88", Name: "Sardinia", Code: "88", CountryID: "IT", Type: "region", Latitude: float64Ptr(40.1), Longitude: float64Ptr(9.0), Active: true},
		{ID: "IT-42", Name: "Liguria", Code: "42", CountryID: "IT", Type: "region", Latitude: float64Ptr(44.4), Longitude: float64Ptr(8.9), Active: true},
		{ID: "IT-57", Name: "Marche", Code: "57", CountryID: "IT", Type: "region", Latitude: float64Ptr(43.6), Longitude: float64Ptr(13.5), Active: true},
		{ID: "IT-65", Name: "Abruzzo", Code: "65", CountryID: "IT", Type: "region", Latitude: float64Ptr(42.2), Longitude: float64Ptr(13.6), Active: true},
		{ID: "IT-36", Name: "Friuli Venezia Giulia", Code: "36", CountryID: "IT", Type: "region", Latitude: float64Ptr(46.0), Longitude: float64Ptr(13.2), Active: true},
		{ID: "IT-32", Name: "Trentino-Alto Adige", Code: "32", CountryID: "IT", Type: "region", Latitude: float64Ptr(46.4), Longitude: float64Ptr(11.3), Active: true},
		{ID: "IT-55", Name: "Umbria", Code: "55", CountryID: "IT", Type: "region", Latitude: float64Ptr(42.9), Longitude: float64Ptr(12.6), Active: true},
		{ID: "IT-77", Name: "Basilicata", Code: "77", CountryID: "IT", Type: "region", Latitude: float64Ptr(40.5), Longitude: float64Ptr(16.3), Active: true},
		{ID: "IT-67", Name: "Molise", Code: "67", CountryID: "IT", Type: "region", Latitude: float64Ptr(41.7), Longitude: float64Ptr(14.4), Active: true},
		{ID: "IT-23", Name: "Aosta Valley", Code: "23", CountryID: "IT", Type: "region", Latitude: float64Ptr(45.7), Longitude: float64Ptr(7.3), Active: true},
	}

	// Brazil States (27)
	brazilStates := []models.State{
		{ID: "BR-SP", Name: "São Paulo", Code: "SP", CountryID: "BR", Type: "state", Latitude: float64Ptr(-23.6), Longitude: float64Ptr(-46.6), Active: true},
		{ID: "BR-RJ", Name: "Rio de Janeiro", Code: "RJ", CountryID: "BR", Type: "state", Latitude: float64Ptr(-22.9), Longitude: float64Ptr(-43.2), Active: true},
		{ID: "BR-MG", Name: "Minas Gerais", Code: "MG", CountryID: "BR", Type: "state", Latitude: float64Ptr(-18.5), Longitude: float64Ptr(-44.3), Active: true},
		{ID: "BR-BA", Name: "Bahia", Code: "BA", CountryID: "BR", Type: "state", Latitude: float64Ptr(-12.6), Longitude: float64Ptr(-41.7), Active: true},
		{ID: "BR-RS", Name: "Rio Grande do Sul", Code: "RS", CountryID: "BR", Type: "state", Latitude: float64Ptr(-30.0), Longitude: float64Ptr(-51.2), Active: true},
		{ID: "BR-PR", Name: "Paraná", Code: "PR", CountryID: "BR", Type: "state", Latitude: float64Ptr(-25.3), Longitude: float64Ptr(-49.3), Active: true},
		{ID: "BR-PE", Name: "Pernambuco", Code: "PE", CountryID: "BR", Type: "state", Latitude: float64Ptr(-8.3), Longitude: float64Ptr(-36.5), Active: true},
		{ID: "BR-CE", Name: "Ceará", Code: "CE", CountryID: "BR", Type: "state", Latitude: float64Ptr(-5.5), Longitude: float64Ptr(-39.3), Active: true},
		{ID: "BR-PA", Name: "Pará", Code: "PA", CountryID: "BR", Type: "state", Latitude: float64Ptr(-3.8), Longitude: float64Ptr(-52.5), Active: true},
		{ID: "BR-SC", Name: "Santa Catarina", Code: "SC", CountryID: "BR", Type: "state", Latitude: float64Ptr(-27.3), Longitude: float64Ptr(-49.5), Active: true},
		{ID: "BR-GO", Name: "Goiás", Code: "GO", CountryID: "BR", Type: "state", Latitude: float64Ptr(-16.0), Longitude: float64Ptr(-49.3), Active: true},
		{ID: "BR-MA", Name: "Maranhão", Code: "MA", CountryID: "BR", Type: "state", Latitude: float64Ptr(-5.4), Longitude: float64Ptr(-45.4), Active: true},
		{ID: "BR-AM", Name: "Amazonas", Code: "AM", CountryID: "BR", Type: "state", Latitude: float64Ptr(-3.4), Longitude: float64Ptr(-65.9), Active: true},
		{ID: "BR-ES", Name: "Espírito Santo", Code: "ES", CountryID: "BR", Type: "state", Latitude: float64Ptr(-19.2), Longitude: float64Ptr(-40.3), Active: true},
		{ID: "BR-PB", Name: "Paraíba", Code: "PB", CountryID: "BR", Type: "state", Latitude: float64Ptr(-7.2), Longitude: float64Ptr(-36.8), Active: true},
		{ID: "BR-MT", Name: "Mato Grosso", Code: "MT", CountryID: "BR", Type: "state", Latitude: float64Ptr(-12.6), Longitude: float64Ptr(-56.1), Active: true},
		{ID: "BR-RN", Name: "Rio Grande do Norte", Code: "RN", CountryID: "BR", Type: "state", Latitude: float64Ptr(-5.7), Longitude: float64Ptr(-36.5), Active: true},
		{ID: "BR-PI", Name: "Piauí", Code: "PI", CountryID: "BR", Type: "state", Latitude: float64Ptr(-7.7), Longitude: float64Ptr(-42.7), Active: true},
		{ID: "BR-AL", Name: "Alagoas", Code: "AL", CountryID: "BR", Type: "state", Latitude: float64Ptr(-9.6), Longitude: float64Ptr(-36.8), Active: true},
		{ID: "BR-MS", Name: "Mato Grosso do Sul", Code: "MS", CountryID: "BR", Type: "state", Latitude: float64Ptr(-20.5), Longitude: float64Ptr(-54.6), Active: true},
		{ID: "BR-DF", Name: "Federal District", Code: "DF", CountryID: "BR", Type: "federal district", Latitude: float64Ptr(-15.8), Longitude: float64Ptr(-47.9), Active: true},
		{ID: "BR-SE", Name: "Sergipe", Code: "SE", CountryID: "BR", Type: "state", Latitude: float64Ptr(-10.6), Longitude: float64Ptr(-37.4), Active: true},
		{ID: "BR-RO", Name: "Rondônia", Code: "RO", CountryID: "BR", Type: "state", Latitude: float64Ptr(-10.8), Longitude: float64Ptr(-63.6), Active: true},
		{ID: "BR-TO", Name: "Tocantins", Code: "TO", CountryID: "BR", Type: "state", Latitude: float64Ptr(-10.2), Longitude: float64Ptr(-48.3), Active: true},
		{ID: "BR-AC", Name: "Acre", Code: "AC", CountryID: "BR", Type: "state", Latitude: float64Ptr(-9.0), Longitude: float64Ptr(-70.8), Active: true},
		{ID: "BR-AP", Name: "Amapá", Code: "AP", CountryID: "BR", Type: "state", Latitude: float64Ptr(1.4), Longitude: float64Ptr(-51.8), Active: true},
		{ID: "BR-RR", Name: "Roraima", Code: "RR", CountryID: "BR", Type: "state", Latitude: float64Ptr(2.8), Longitude: float64Ptr(-61.4), Active: true},
	}

	// Mexico States (32)
	mexicoStates := []models.State{
		{ID: "MX-CMX", Name: "Mexico City", Code: "CMX", CountryID: "MX", Type: "federal district", Latitude: float64Ptr(19.4), Longitude: float64Ptr(-99.1), Active: true},
		{ID: "MX-JAL", Name: "Jalisco", Code: "JAL", CountryID: "MX", Type: "state", Latitude: float64Ptr(20.7), Longitude: float64Ptr(-103.3), Active: true},
		{ID: "MX-MEX", Name: "State of Mexico", Code: "MEX", CountryID: "MX", Type: "state", Latitude: float64Ptr(19.4), Longitude: float64Ptr(-99.7), Active: true},
		{ID: "MX-NLE", Name: "Nuevo León", Code: "NLE", CountryID: "MX", Type: "state", Latitude: float64Ptr(25.6), Longitude: float64Ptr(-99.9), Active: true},
		{ID: "MX-VER", Name: "Veracruz", Code: "VER", CountryID: "MX", Type: "state", Latitude: float64Ptr(19.2), Longitude: float64Ptr(-96.1), Active: true},
		{ID: "MX-PUE", Name: "Puebla", Code: "PUE", CountryID: "MX", Type: "state", Latitude: float64Ptr(19.0), Longitude: float64Ptr(-98.2), Active: true},
		{ID: "MX-GUA", Name: "Guanajuato", Code: "GUA", CountryID: "MX", Type: "state", Latitude: float64Ptr(21.0), Longitude: float64Ptr(-101.3), Active: true},
		{ID: "MX-CHH", Name: "Chihuahua", Code: "CHH", CountryID: "MX", Type: "state", Latitude: float64Ptr(28.6), Longitude: float64Ptr(-106.1), Active: true},
		{ID: "MX-MIC", Name: "Michoacán", Code: "MIC", CountryID: "MX", Type: "state", Latitude: float64Ptr(19.6), Longitude: float64Ptr(-101.7), Active: true},
		{ID: "MX-OAX", Name: "Oaxaca", Code: "OAX", CountryID: "MX", Type: "state", Latitude: float64Ptr(17.1), Longitude: float64Ptr(-96.7), Active: true},
		{ID: "MX-SIN", Name: "Sinaloa", Code: "SIN", CountryID: "MX", Type: "state", Latitude: float64Ptr(24.8), Longitude: float64Ptr(-107.4), Active: true},
		{ID: "MX-SON", Name: "Sonora", Code: "SON", CountryID: "MX", Type: "state", Latitude: float64Ptr(29.1), Longitude: float64Ptr(-110.9), Active: true},
		{ID: "MX-GRO", Name: "Guerrero", Code: "GRO", CountryID: "MX", Type: "state", Latitude: float64Ptr(17.4), Longitude: float64Ptr(-99.5), Active: true},
		{ID: "MX-TAM", Name: "Tamaulipas", Code: "TAM", CountryID: "MX", Type: "state", Latitude: float64Ptr(24.3), Longitude: float64Ptr(-98.8), Active: true},
		{ID: "MX-COA", Name: "Coahuila", Code: "COA", CountryID: "MX", Type: "state", Latitude: float64Ptr(27.3), Longitude: float64Ptr(-102.0), Active: true},
		{ID: "MX-BCN", Name: "Baja California", Code: "BCN", CountryID: "MX", Type: "state", Latitude: float64Ptr(30.8), Longitude: float64Ptr(-115.3), Active: true},
	}

	// UAE Emirates (7)
	uaeEmirates := []models.State{
		{ID: "AE-AZ", Name: "Abu Dhabi", Code: "AZ", CountryID: "AE", Type: "emirate", Latitude: float64Ptr(24.5), Longitude: float64Ptr(54.4), Active: true},
		{ID: "AE-DU", Name: "Dubai", Code: "DU", CountryID: "AE", Type: "emirate", Latitude: float64Ptr(25.3), Longitude: float64Ptr(55.3), Active: true},
		{ID: "AE-SH", Name: "Sharjah", Code: "SH", CountryID: "AE", Type: "emirate", Latitude: float64Ptr(25.4), Longitude: float64Ptr(55.5), Active: true},
		{ID: "AE-AJ", Name: "Ajman", Code: "AJ", CountryID: "AE", Type: "emirate", Latitude: float64Ptr(25.4), Longitude: float64Ptr(55.5), Active: true},
		{ID: "AE-UQ", Name: "Umm Al Quwain", Code: "UQ", CountryID: "AE", Type: "emirate", Latitude: float64Ptr(25.6), Longitude: float64Ptr(55.6), Active: true},
		{ID: "AE-RK", Name: "Ras Al Khaimah", Code: "RK", CountryID: "AE", Type: "emirate", Latitude: float64Ptr(25.8), Longitude: float64Ptr(55.9), Active: true},
		{ID: "AE-FU", Name: "Fujairah", Code: "FU", CountryID: "AE", Type: "emirate", Latitude: float64Ptr(25.1), Longitude: float64Ptr(56.3), Active: true},
	}

	// South Africa Provinces (9)
	southAfricaProvinces := []models.State{
		{ID: "ZA-GP", Name: "Gauteng", Code: "GP", CountryID: "ZA", Type: "province", Latitude: float64Ptr(-26.3), Longitude: float64Ptr(28.1), Active: true},
		{ID: "ZA-KZN", Name: "KwaZulu-Natal", Code: "KZN", CountryID: "ZA", Type: "province", Latitude: float64Ptr(-28.5), Longitude: float64Ptr(30.5), Active: true},
		{ID: "ZA-WC", Name: "Western Cape", Code: "WC", CountryID: "ZA", Type: "province", Latitude: float64Ptr(-33.2), Longitude: float64Ptr(18.8), Active: true},
		{ID: "ZA-EC", Name: "Eastern Cape", Code: "EC", CountryID: "ZA", Type: "province", Latitude: float64Ptr(-32.0), Longitude: float64Ptr(26.5), Active: true},
		{ID: "ZA-LP", Name: "Limpopo", Code: "LP", CountryID: "ZA", Type: "province", Latitude: float64Ptr(-23.9), Longitude: float64Ptr(29.4), Active: true},
		{ID: "ZA-MP", Name: "Mpumalanga", Code: "MP", CountryID: "ZA", Type: "province", Latitude: float64Ptr(-25.6), Longitude: float64Ptr(30.5), Active: true},
		{ID: "ZA-NW", Name: "North West", Code: "NW", CountryID: "ZA", Type: "province", Latitude: float64Ptr(-25.9), Longitude: float64Ptr(25.6), Active: true},
		{ID: "ZA-FS", Name: "Free State", Code: "FS", CountryID: "ZA", Type: "province", Latitude: float64Ptr(-29.1), Longitude: float64Ptr(26.2), Active: true},
		{ID: "ZA-NC", Name: "Northern Cape", Code: "NC", CountryID: "ZA", Type: "province", Latitude: float64Ptr(-29.0), Longitude: float64Ptr(21.9), Active: true},
	}

	// European Countries with N/A (small countries without subdivisions)
	europeanNA := []models.State{
		{ID: "AD-NA", Name: "N/A", Code: "NA", CountryID: "AD", Type: "N/A", Latitude: float64Ptr(42.5), Longitude: float64Ptr(1.5), Active: true},
		{ID: "LI-NA", Name: "N/A", Code: "NA", CountryID: "LI", Type: "N/A", Latitude: float64Ptr(47.2), Longitude: float64Ptr(9.5), Active: true},
		{ID: "LU-NA", Name: "N/A", Code: "NA", CountryID: "LU", Type: "N/A", Latitude: float64Ptr(49.8), Longitude: float64Ptr(6.1), Active: true},
		{ID: "MC-NA", Name: "N/A", Code: "NA", CountryID: "MC", Type: "N/A", Latitude: float64Ptr(43.7), Longitude: float64Ptr(7.4), Active: true},
		{ID: "MT-NA", Name: "N/A", Code: "NA", CountryID: "MT", Type: "N/A", Latitude: float64Ptr(35.9), Longitude: float64Ptr(14.4), Active: true},
		{ID: "SM-NA", Name: "N/A", Code: "NA", CountryID: "SM", Type: "N/A", Latitude: float64Ptr(43.9), Longitude: float64Ptr(12.5), Active: true},
		{ID: "VA-NA", Name: "N/A", Code: "NA", CountryID: "VA", Type: "N/A", Latitude: float64Ptr(41.9), Longitude: float64Ptr(12.5), Active: true},
		{ID: "SG-NA", Name: "N/A", Code: "NA", CountryID: "SG", Type: "N/A", Latitude: float64Ptr(1.4), Longitude: float64Ptr(103.8), Active: true},
		{ID: "HK-NA", Name: "N/A", Code: "NA", CountryID: "HK", Type: "N/A", Latitude: float64Ptr(22.3), Longitude: float64Ptr(114.2), Active: true},
		{ID: "MO-NA", Name: "N/A", Code: "NA", CountryID: "MO", Type: "N/A", Latitude: float64Ptr(22.2), Longitude: float64Ptr(113.5), Active: true},
	}

	// Netherlands Provinces (12)
	netherlandsProvinces := []models.State{
		{ID: "NL-NH", Name: "North Holland", Code: "NH", CountryID: "NL", Type: "province", Latitude: float64Ptr(52.5), Longitude: float64Ptr(4.8), Active: true},
		{ID: "NL-ZH", Name: "South Holland", Code: "ZH", CountryID: "NL", Type: "province", Latitude: float64Ptr(52.0), Longitude: float64Ptr(4.4), Active: true},
		{ID: "NL-NB", Name: "North Brabant", Code: "NB", CountryID: "NL", Type: "province", Latitude: float64Ptr(51.5), Longitude: float64Ptr(5.1), Active: true},
		{ID: "NL-GE", Name: "Gelderland", Code: "GE", CountryID: "NL", Type: "province", Latitude: float64Ptr(52.0), Longitude: float64Ptr(5.9), Active: true},
		{ID: "NL-LI", Name: "Limburg", Code: "LI", CountryID: "NL", Type: "province", Latitude: float64Ptr(51.2), Longitude: float64Ptr(5.9), Active: true},
		{ID: "NL-OV", Name: "Overijssel", Code: "OV", CountryID: "NL", Type: "province", Latitude: float64Ptr(52.4), Longitude: float64Ptr(6.5), Active: true},
		{ID: "NL-FL", Name: "Flevoland", Code: "FL", CountryID: "NL", Type: "province", Latitude: float64Ptr(52.5), Longitude: float64Ptr(5.5), Active: true},
		{ID: "NL-UT", Name: "Utrecht", Code: "UT", CountryID: "NL", Type: "province", Latitude: float64Ptr(52.1), Longitude: float64Ptr(5.1), Active: true},
		{ID: "NL-FR", Name: "Friesland", Code: "FR", CountryID: "NL", Type: "province", Latitude: float64Ptr(53.2), Longitude: float64Ptr(5.8), Active: true},
		{ID: "NL-GR", Name: "Groningen", Code: "GR", CountryID: "NL", Type: "province", Latitude: float64Ptr(53.2), Longitude: float64Ptr(6.6), Active: true},
		{ID: "NL-DR", Name: "Drenthe", Code: "DR", CountryID: "NL", Type: "province", Latitude: float64Ptr(52.9), Longitude: float64Ptr(6.6), Active: true},
		{ID: "NL-ZE", Name: "Zeeland", Code: "ZE", CountryID: "NL", Type: "province", Latitude: float64Ptr(51.5), Longitude: float64Ptr(3.8), Active: true},
	}

	// Combine all states
	states = append(states, usStates...)
	states = append(states, indiaStates...)
	states = append(states, canadaProvinces...)
	states = append(states, australiaStates...)
	states = append(states, nepalProvinces...)
	states = append(states, bangladeshDivisions...)
	states = append(states, sriLankaProvinces...)
	states = append(states, pakistanProvinces...)
	states = append(states, indonesiaProvinces...)
	states = append(states, philippinesRegions...)
	states = append(states, thailandRegions...)
	states = append(states, vietnamRegions...)
	states = append(states, malaysiaStates...)
	states = append(states, japanPrefectures...)
	states = append(states, southKoreaProvinces...)
	states = append(states, afghanistanProvinces...)
	states = append(states, bruneiDistricts...)
	states = append(states, albaniaCounties...)
	states = append(states, argentinaProvinces...)
	states = append(states, bhutanDistricts...)
	states = append(states, germanyStates...)
	states = append(states, ukCountries...)
	states = append(states, franceRegions...)
	states = append(states, spainCommunities...)
	states = append(states, italyRegions...)
	states = append(states, brazilStates...)
	states = append(states, mexicoStates...)
	states = append(states, uaeEmirates...)
	states = append(states, southAfricaProvinces...)
	states = append(states, europeanNA...)
	states = append(states, netherlandsProvinces...)

	return states
}
