package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"io"
	"net/http"
	"bytes"
	"time"

	"github.com/umahmood/haversine"
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
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var result map[string]Sonde
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return fmt.Errorf("decoding Sondehub JSON: %w", err)
	}

	if len(result) == 0 {
		fmt.Println("✅ No nearby landed sondes.")
		return nil
	}

	for id, sonde := range result {
		timestamp, err := time.Parse(time.RFC3339, sonde.Datetime)
		if err != nil {
			fmt.Printf("⚠️ Could not parse time for sonde %s: %v\n", id, err)
			continue
		}
		timeAgo := time.Since(timestamp).Round(time.Minute)
		userCoord := haversine.Coord{Lat: userLat, Lon: userLon}
		sondeCoord := haversine.Coord{Lat: sonde.Lat, Lon: sonde.Lon}

		_, km := haversine.Distance(userCoord, sondeCoord) // distance in kilometers

		msg := fmt.Sprintf(
			"📡 Sonde %s at %.0f m about %s ago, %.1f km away",
			id, sonde.Alt, timeAgo, km,
		)
		fmt.Println(msg)

		url := fmt.Sprintf("https://sondehub.org/%s", id)
		err = notifyHA(msg, url)
		if err != nil {
			fmt.Printf("⚠️ Failed to notify for %s: %v\n", id, err)
		}
	}

	return nil
}

func notifyHA(message, url string) error {
	notificationURL := haURL + "/api/services/notify/notify"
	payload := map[string]interface{}{
		"message": message,
		"data": map[string]string{
			"url": url, // tap action opens sonde page
		},
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", notificationURL, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+haToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HA notify error: %s", body)
	}

	return nil
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
			notifyHA("Error checking sondes: " + err.Error(), "https://example.com")
		} 
		fmt.Println("Loop complete, sleeping...")
		time.Sleep(checkInterval)
	}
}
