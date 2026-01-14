package seeder

import (
	"github.com/tesseract-hub/domains/common/services/location-service/internal/models"
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

	// Combine all states
	states = append(states, usStates...)
	states = append(states, indiaStates...)
	states = append(states, canadaProvinces...)
	states = append(states, australiaStates...)

	return states
}
