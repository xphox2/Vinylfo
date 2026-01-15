# Database Structure for Vinylfo

This document describes the database schema for the Vinylfo application.

## Tables

### Albums
- `id` (integer, primary key, auto-increment)
- `title` (varchar, not null, unique)
- `artist` (varchar, not null)
- `release_year` (integer)
- `genre` (varchar)
- `cover_image_url` (varchar)
- `created_at` (datetime)
- `updated_at` (datetime)

### Tracks
- `id` (integer, primary key, auto-increment)
- `album_id` (integer, foreign key referencing Albums.id, not null, indexed)
- `title` (varchar, not null)
- `duration` (integer, in seconds)
- `track_number` (integer)
- `audio_file_url` (varchar)
- `created_at` (datetime)
- `updated_at` (datetime)

### Playback Sessions
- `id` (integer, primary key, auto-increment)
- `track_id` (integer, foreign key referencing Tracks.id, not null, indexed)
- `start_time` (datetime)
- `end_time` (datetime)
- `duration` (integer, in seconds)
- `progress` (integer, in seconds)
- `created_at` (datetime)
- `updated_at` (datetime)

## Relationships

- Albums to Tracks: One-to-Many (via album_id foreign key)
- Tracks to Playback Sessions: One-to-Many (via track_id foreign key)

## Constraints

- Indexes on frequently queried fields (album_id, track_id)
- Proper foreign key constraints for referential integrity