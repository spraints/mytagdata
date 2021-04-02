package data

import "time"

// WirelessTagUpdate is a JSON struct received from a wireless tag manager.
//
// Configure it by doing this:
// 1. Visit https://mytaglist.com/eth/.
// 2. Click on ">" (More) for a tag.
// 3. Click on "URL Calling".
// 4. Check "When tag sends a temperature/humidity/brightness update - {0}: Tag name, {1}: Tag ID, {2}: temperature in °C, {3}: humidity/moisture (%), {4}: brightness (lux), {5}: timestamp, {6}: battery voltage".
// 5. Fill in the URL with the appropriate value (e.g. "http://192.168.164.128:8900/wirelesstags/updates").
// 6. Check "This URL uses private IP address (Call from Tag Manager)" if you'll be running this on the same LAN as the tag manager.
// 7. Click on ">" (More HTTP Settings).
// 8. Fill in the template with this:
//      {"tag_name":"{0}","tag_id":"{1}","degrees_c":{2},"humidity":{3},"now":"{5}","battery":{6}}
// 9. Scroll down and click "Save".
type WirelessTagUpdate struct {
	Name      string    `json:"tag_name"`
	ID        string    `json:"tag_id"`
	DegreesC  float64   `json:"degrees_c"`
	Humidity  float64   `json:"humidity"`
	Battery   float64   `json:"battery"`
	Timestamp time.Time `json:"now"`
}
