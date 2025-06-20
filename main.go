package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"io"
	"net/http"
	"strings"
	"time"

)

// ========== CONFIG ==========
const (
	haToken      = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiIxZjQ1NDkxZjgyNjQ0MGY3YWVkZWI1OWQ1YTkxMGNhMyIsImlhdCI6MTc1MDM0MTY1OCwiZXhwIjoyMDY1NzAxNjU4fQ.zX-xUTLWq9iXGVUfbrL-xwhPC8I4-MbCgPvjsdWDNoo"
	haURL        = "https://hass.vanstaveren.us"
	entityID     = "person.patrick_van_staveren"
	distanceKM   = 100
	checkInterval = 10 * time.Minute
)

// ========== TYPES ==========

type HAState struct {
	Attributes struct {
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
	} `json:"attributes"`
}

type SondeResponse struct {
	Sondes []struct {
		Lat   float64 `json:"lat"`
		Lon   float64 `json:"lon"`
		Frame struct {
			Type string `json:"type"`
		} `json:"frame"`
	} `json:"sondes"`
}

type Sonde struct {
	Lat      float64 `json:"lat"`
	Lon      float64 `json:"lon"`
	Alt      float64 `json:"alt"`
	Serial   string  `json:"serial"`
	Datetime string  `json:"datetime"`
}

// ========== MAIN LOOP ==========

func checkNearbySondes() error {
	userLat, userLon, err := getUserLocation()
	if err != nil {
		return fmt.Errorf("getting user location: %w", err)
	}

	url := fmt.Sprintf(
		"https://api.v2.sondehub.org/sondes?frame_types=landing&lat=%f&lon=%f&distance=%d",
		userLat, userLon, int(distanceKM*1000),
	)

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("requesting sondes: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	// The returned JSON is a map of string keys to sonde data
	var result map[string]struct {
		Lat float64 `json:"lat"`
		Lon float64 `json:"lon"`
		Alt float64 `json:"alt"`
	}

	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return fmt.Errorf("decoding Sondehub JSON: %w", err)
	}

	if len(result) == 0 {
		fmt.Println("✅ No nearby landed sondes.")
		return nil
	}

	// Optional: print info about each found sonde
	for serial, sonde := range result {
		fmt.Printf("📡 Sonde %s landed at %.5f, %.5f (alt %.1f m)\n", serial, sonde.Lat, sonde.Lon, sonde.Alt)
	}

	// Trigger HA notification
	msg := fmt.Sprintf("⚠️ %d radiosonde(s) landed nearby!", len(result))
	notifyHA(msg)

	return nil
}

func notifyHA(message string) {
	url := haURL + "/api/services/notify/notify"
	payload := fmt.Sprintf(`{"message": "%s"}`, message)
	req, _ := http.NewRequest("POST", url, strings.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+haToken)
	req.Header.Set("Content-Type", "application/json")

	_, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println("Failed to send notification:", err)
	}
}
func getUserLocation() (float64, float64, error) {
	url := fmt.Sprintf("%s/api/states/%s", haURL, entityID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0, 0, err
	}

	req.Header.Set("Authorization", "Bearer "+haToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return 0, 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := ioutil.ReadAll(resp.Body)
		return 0, 0, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Attributes struct {
			Latitude  float64 `json:"latitude"`
			Longitude float64 `json:"longitude"`
		} `json:"attributes"`
	}

	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return 0, 0, fmt.Errorf("decoding JSON: %w", err)
	}
    fmt.Printf("User %s is at %f, %f\n", entityID, result.Attributes.Latitude, result.Attributes.Longitude)
	return result.Attributes.Latitude, result.Attributes.Longitude, nil
}



func main() {
	for {
		err := checkNearbySondes()
		if err != nil {
			fmt.Println("Error:", err)
			notifyHA("Error checking sondes: " + err.Error())
		} else {
			fmt.Println("Nothing to see here")
		}
		time.Sleep(checkInterval)
	}
}
