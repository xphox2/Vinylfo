# Vinylfo - Vinyl Collection Organizer

Vinylfo is a web application for organizing your vinyl collection. It connects to the Discogs API to fetch album information and allows you to manage your collection in a centralized database.

## Features

- Fetch album information from Discogs API
- Organize vinyl collection in a local database
- RESTful API for collection management
- Playback session tracking
- Web interface for browsing collection

## Getting Started

### Prerequisites

- Go 1.21 or higher
- MySQL database
- Discogs API key

### Installation

1. Clone the repository
2. Install dependencies:

```bash
go mod tidy
```

3. Build the application:

```bash
go build -o vinylfo main.go
```

4. Run the application:

```bash
./vinylfo
```

### Configuration

1. Create a MySQL database named `vinylfo`
2. Update the database connection string in `main.go` with your credentials
3. Obtain a Discogs API key from https://www.discogs.com/settings/developer

### API Endpoints

- `GET /health` - Health check endpoint
- `GET /api/albums` - Get all albums in collection
- `GET /api/tracks` - Get all tracks
- `GET /api/playback` - Get current playback session
- `POST /api/sync-collection` - Synchronize collection with Discogs API

## Database Models

- `Album`: Represents a vinyl album
- `Track`: Represents a track on an album
- `PlaybackSession`: Represents the current playback state

## License

This project is licensed under the MIT License.