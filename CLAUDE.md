# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

sonde-alert is a Go daemon that monitors your location via Home Assistant and sends push notifications about nearby landed weather balloons (radiosondes) detected by Sondehub.

## Build and Run Commands

```bash
# Build the binary
go build -v ./...

# Run tests
go test -v ./...

# Run directly (requires .env file configured)
./main
```

The application is designed to run as a systemd service, polling every 10 minutes.

## Architecture

Single-file application (main.go) with these key components:

- **Configuration**: Loads from `.env` file using godotenv
- **Location tracking**: Fetches user location from Home Assistant Person entity
- **Sonde discovery**: Queries Sondehub API for landed sondes within configured radius
- **Notifications**: Sends push notifications via Home Assistant mobile app service
- **Deduplication**: Tracks notified sondes in `notified.json` to prevent duplicates

**Control flow**: `main()` runs infinite loop → `getUserLocation()` → `checkNearbySondes()` → `notifyHA()` for each new sonde

## External APIs

- **Sondehub**: `https://api.v2.sondehub.org/sondes?frame_types=landing&lat=X&lon=Y&distance=Z`
- **Home Assistant**: GET `/api/states/{entity_id}` for location, POST `/api/services/notify/{service_name}` for notifications

## Configuration

Environment variables (see `.env.example`):
- `HA_URL` - Home Assistant instance URL
- `HA_TOKEN` - Long-lived access token
- `HA_PERSON_ENTITY_ID` - Person entity to track location
- `DISTANCE_KM` - Search radius in kilometers

## Dependencies

Minimal external dependencies:
- `github.com/joho/godotenv` - Environment file loading
- `github.com/umahmood/haversine` - Geographic distance calculations
