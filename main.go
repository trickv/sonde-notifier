package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"io"
	"net/http"
	"bytes"
	"time"
	"os"
	"strconv"

	"github.com/umahmood/haversine"
	"github.com/joho/godotenv"
)

// ========== CONFIG ==========

func loadConfig() error {
	return godotenv.Load(".env")
}

var (
	haURL     string
	haToken   string
	entityID  string
	distanceKM float64
)

const (
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

func loadNotified() (map[string]bool, error) {
	data, err := os.ReadFile("notified.json")
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]bool), nil // file doesn't exist yet
		}
		return nil, err
	}
	var notified map[string]bool
	err = json.Unmarshal(data, &notified)
	return notified, err
}

func saveNotified(notified map[string]bool) error {
	data, err := json.MarshalIndent(notified, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile("notified.json", data, 0644)
}


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

	notified, _ := loadNotified()

	for id, sonde := range result {
		if notified[id] {
            fmt.Println("Already notified about sonde ", id)
            continue // already notified
        }
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
		} else {
			notified[id] = true // mark as notified
		}
		saveNotified(notified)
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
	loadConfig()
	haURL = os.Getenv("HA_URL")
	haToken = os.Getenv("HA_TOKEN")
	entityID = os.Getenv("HA_PERSON_ENTITY_ID")
	distanceKM, _ = strconv.ParseFloat(os.Getenv("DISTANCE_KM"), 64)
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
